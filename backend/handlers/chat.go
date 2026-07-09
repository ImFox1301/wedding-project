package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"wedding/db"
	"wedding/middleware"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ── Hub ───────────────────────────────────────────────────────────
// Two shared rooms keyed by role: "friends" and "family".

type chatClient struct {
	room string
	send chan []byte
}

type chatHub struct {
	mu      sync.RWMutex
	clients map[string]map[*chatClient]bool
}

var hub = &chatHub{clients: map[string]map[*chatClient]bool{}}

// chatMaxMessages returns the admin-configured max messages kept per room
// (setting key "chat_max_messages"). Falls back to 200 if unset/invalid.
// A value <= 0 disables auto-trim.
func chatMaxMessages() int {
	var v string
	db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='chat_max_messages'`).Scan(&v)
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 200
	}
	return n
}

func (h *chatHub) add(cl *chatClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[cl.room] == nil {
		h.clients[cl.room] = map[*chatClient]bool{}
	}
	h.clients[cl.room][cl] = true
}

func (h *chatHub) remove(cl *chatClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.clients[cl.room]; set != nil {
		delete(set, cl)
	}
}

func (h *chatHub) broadcast(room string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for cl := range h.clients[room] {
		select {
		case cl.send <- msg:
		default: // slow client — drop
		}
	}
}

var chatUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type chatMsgOut struct {
	ID         int    `json:"id"`
	Role       string `json:"role"`
	GuestID    *int   `json:"guest_id"`
	SenderName string `json:"sender_name"`
	IsAdmin    bool   `json:"is_admin"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
}

// firstRune returns the first character of a string (for initials).
func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return ""
}

// Display name for an individual guest: "Иванов И. П."
func guestDisplayName(last, first, middle string) string {
	name := last
	if fi := firstRune(first); fi != "" {
		name += " " + fi + "."
	}
	if mi := firstRune(middle); mi != "" {
		name += " " + mi + "."
	}
	return strings.TrimSpace(name)
}

// resolveSender returns room, guestID, groupID, senderName for the connection.
func resolveSender(claims *middleware.Claims, queryRoom string) (room string, guestID *int, groupID *int, senderName string, isAdmin bool, ok bool) {
	if claims.Role == "admin" {
		room = queryRoom
		if room != "friends" && room != "family" {
			room = "friends"
		}
		return room, nil, nil, "Организаторы", true, true
	}
	// Guest: room is their own role
	room = claims.Role
	if room != "friends" && room != "family" {
		return "", nil, nil, "", false, false
	}
	gid := claims.UserID
	var last, first, middle string
	var grp *int
	db.DB.QueryRow(`SELECT last_name, first_name, middle_name, group_id FROM guests WHERE id=$1`, gid).
		Scan(&last, &first, &middle, &grp)
	if grp != nil {
		var gname string
		db.DB.QueryRow(`SELECT name FROM guest_groups WHERE id=$1`, *grp).Scan(&gname)
		if gname == "" {
			gname = guestDisplayName(last, first, middle)
		}
		return room, &gid, grp, gname, false, true
	}
	return room, &gid, nil, guestDisplayName(last, first, middle), false, true
}

// ChatWS upgrades to a WebSocket. Token comes via ?token=… (browsers can't set
// Authorization on WS). Guests join their role's room; admin picks ?room=.
func ChatWS(c *gin.Context) {
	claims, err := middleware.ParseToken(c.Query("token"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	room, guestID, groupID, senderName, isAdmin, ok := resolveSender(claims, c.Query("room"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad room"})
		return
	}

	conn, err := chatUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	cl := &chatClient{room: room, send: make(chan []byte, 32)}
	hub.add(cl)

	// Writer goroutine — the only place that writes to the socket.
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for msg := range cl.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Reader loop
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var in struct {
			Body string `json:"body"`
		}
		if json.Unmarshal(data, &in) != nil {
			continue
		}
		body := strings.TrimSpace(in.Body)
		if body == "" {
			continue
		}
		if len([]rune(body)) > 2000 {
			body = string([]rune(body)[:2000])
		}

		var id int
		var createdAt string
		err = db.DB.QueryRow(`
			INSERT INTO chat_messages (role, guest_id, group_id, sender_name, is_admin, body)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			room, guestID, groupID, senderName, isAdmin, body,
		).Scan(&id, &createdAt)
		if err != nil {
			continue
		}
		out := chatMsgOut{ID: id, Role: room, GuestID: guestID, SenderName: senderName, IsAdmin: isAdmin, Body: body, CreatedAt: createdAt}
		b, _ := json.Marshal(out)
		hub.broadcast(room, b)

		// Auto-clear: keep only the newest N messages in this room (N is
		// admin-configurable via the "chat_max_messages" setting). N<=0 disables.
		if keep := chatMaxMessages(); keep > 0 {
			db.DB.Exec(`DELETE FROM chat_messages WHERE role=$1 AND id NOT IN (
				SELECT id FROM chat_messages WHERE role=$1 ORDER BY id DESC LIMIT $2)`, room, keep)
		}

		// Sender has implicitly seen up to their own message
		if guestID != nil {
			db.DB.Exec(`UPDATE guests SET chat_seen_at=NOW() WHERE id=$1`, *guestID)
		}
	}

	hub.remove(cl)
	close(cl.send)
	conn.Close()
	<-writerDone
}

// queryMessages loads the last 200 messages for a room in chronological order.
func queryMessages(room string) []chatMsgOut {
	rows, err := db.DB.Query(`
		SELECT id, role, guest_id, sender_name, is_admin, body,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM chat_messages WHERE role=$1 ORDER BY id DESC LIMIT 200`, room)
	if err != nil {
		return []chatMsgOut{}
	}
	defer rows.Close()
	var list []chatMsgOut
	for rows.Next() {
		var m chatMsgOut
		rows.Scan(&m.ID, &m.Role, &m.GuestID, &m.SenderName, &m.IsAdmin, &m.Body, &m.CreatedAt)
		list = append(list, m)
	}
	// reverse to chronological
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	if list == nil {
		list = []chatMsgOut{}
	}
	return list
}

// GetChatMessages — guest's own room history.
func GetChatMessages(c *gin.Context) {
	role := c.GetString("role")
	c.JSON(http.StatusOK, gin.H{"room": role, "messages": queryMessages(role)})
}

// MarkChatSeen — guest opened the chat.
func MarkChatSeen(c *gin.Context) {
	guestID := c.GetInt("user_id")
	db.DB.Exec(`UPDATE guests SET chat_seen_at=NOW() WHERE id=$1`, guestID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetChatUnread — number of messages in the guest's room newer than the last
// time they viewed the chat, excluding their own messages.
func GetChatUnread(c *gin.Context) {
	guestID := c.GetInt("user_id")
	role := c.GetString("role")
	var count int
	db.DB.QueryRow(`
		SELECT COUNT(*) FROM chat_messages m
		WHERE m.role=$1
		  AND (m.guest_id IS NULL OR m.guest_id <> $2)
		  AND m.created_at > COALESCE((SELECT chat_seen_at FROM guests WHERE id=$2), to_timestamp(0))`,
		role, guestID,
	).Scan(&count)
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// broadcastGiftUpdate notifies everyone in a room that the gift list changed,
// so connected clients re-fetch and re-render (real-time gift sync).
func broadcastGiftUpdate(room string) {
	b, _ := json.Marshal(map[string]interface{}{"type": "gift_update", "role": room})
	hub.broadcast(room, b)
}

// GetAdminChatMessages — admin views a chosen room.
func GetAdminChatMessages(c *gin.Context) {
	room := c.Query("room")
	if room != "friends" && room != "family" {
		room = "friends"
	}
	c.JSON(http.StatusOK, gin.H{"room": room, "messages": queryMessages(room)})
}

// broadcastChatReload tells clients in a room to reload their chat history
// (used after the admin clears history or deletes a message).
func broadcastChatReload(room string) {
	b, _ := json.Marshal(map[string]interface{}{"type": "chat_reload", "role": room})
	hub.broadcast(room, b)
}

// ClearChat deletes all messages of a room (admin only).
func ClearChat(c *gin.Context) {
	room := c.Query("room")
	if room != "friends" && room != "family" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad room"})
		return
	}
	if _, err := db.DB.Exec(`DELETE FROM chat_messages WHERE role=$1`, room); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	broadcastChatReload(room)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteChatMessage deletes a single message (admin only).
func DeleteChatMessage(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var room string
	db.DB.QueryRow(`SELECT role FROM chat_messages WHERE id=$1`, id).Scan(&room)
	if _, err := db.DB.Exec(`DELETE FROM chat_messages WHERE id=$1`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if room != "" {
		broadcastChatReload(room)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
