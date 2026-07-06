package handlers

import (
	"net/http"
	"strconv"
	"wedding/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ListLinks(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT l.id, l.token, l.guest_id, l.group_id,
		       to_char(l.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at,
		       g.last_name, g.first_name, g.middle_name,
		       gg.name as group_name
		FROM invitation_links l
		LEFT JOIN guests g ON g.id = l.guest_id
		LEFT JOIN guest_groups gg ON gg.id = l.group_id
		ORDER BY l.created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type LinkRow struct {
		ID         int     `json:"id"`
		Token      string  `json:"token"`
		GuestID    *int    `json:"guest_id"`
		GroupID    *int    `json:"group_id"`
		CreatedAt  string  `json:"created_at"`
		LastName   *string `json:"last_name"`
		FirstName  *string `json:"first_name"`
		MiddleName *string `json:"middle_name"`
		GroupName  *string `json:"group_name"`
	}

	var links []LinkRow
	for rows.Next() {
		var l LinkRow
		if err := rows.Scan(
			&l.ID, &l.Token, &l.GuestID, &l.GroupID, &l.CreatedAt,
			&l.LastName, &l.FirstName, &l.MiddleName, &l.GroupName,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		links = append(links, l)
	}
	if links == nil {
		links = []LinkRow{}
	}
	c.JSON(http.StatusOK, links)
}

// AvailableForLink returns guests and groups that don't yet have an invitation link
func AvailableForLink(c *gin.Context) {
	// Guests who have no individual link AND are not in any group
	// (guests in groups must be invited via a group link, not individually)
	guestRows, err := db.DB.Query(`
		SELECT g.id, g.last_name, g.first_name, g.middle_name, g.role, g.group_id
		FROM guests g
		WHERE g.role != 'admin'
		  AND g.group_id IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM invitation_links l WHERE l.guest_id = g.id
		  )
		ORDER BY g.last_name, g.first_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer guestRows.Close()

	type GuestAvail struct {
		ID         int    `json:"id"`
		LastName   string `json:"last_name"`
		FirstName  string `json:"first_name"`
		MiddleName string `json:"middle_name"`
		Role       string `json:"role"`
		GroupID    *int   `json:"group_id"`
	}
	var guests []GuestAvail
	for guestRows.Next() {
		var g GuestAvail
		guestRows.Scan(&g.ID, &g.LastName, &g.FirstName, &g.MiddleName, &g.Role, &g.GroupID)
		guests = append(guests, g)
	}
	if guests == nil {
		guests = []GuestAvail{}
	}

	// Groups that have no link yet
	groupRows, err := db.DB.Query(`
		SELECT gg.id, gg.name
		FROM guest_groups gg
		WHERE NOT EXISTS (
		    SELECT 1 FROM invitation_links l WHERE l.group_id = gg.id
		)
		ORDER BY gg.name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer groupRows.Close()

	type GroupAvail struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var groups []GroupAvail
	for groupRows.Next() {
		var g GroupAvail
		groupRows.Scan(&g.ID, &g.Name)
		groups = append(groups, g)
	}
	if groups == nil {
		groups = []GroupAvail{}
	}

	c.JSON(http.StatusOK, gin.H{"guests": guests, "groups": groups})
}

func CreateLink(c *gin.Context) {
	var req struct {
		GuestID *int `json:"guest_id"`
		GroupID *int `json:"group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.GuestID == nil && req.GroupID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "guest_id or group_id required"})
		return
	}

	// Check for existing link
	if req.GuestID != nil {
		var exists bool
		db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM invitation_links WHERE guest_id=$1)`, *req.GuestID).Scan(&exists)
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "у этого гостя уже есть ссылка-приглашение"})
			return
		}
		// Guests in any group must be invited through a group link
		var groupID *int
		db.DB.QueryRow(`SELECT group_id FROM guests WHERE id=$1`, *req.GuestID).Scan(&groupID)
		if groupID != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "гость состоит в группе — создайте ссылку для группы"})
			return
		}
	}
	if req.GroupID != nil {
		var exists bool
		db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM invitation_links WHERE group_id=$1)`, *req.GroupID).Scan(&exists)
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "у этой группы уже есть ссылка-приглашение"})
			return
		}
	}

	token := uuid.New().String()
	var id int
	err := db.DB.QueryRow(
		`INSERT INTO invitation_links (token, guest_id, group_id) VALUES ($1,$2,$3) RETURNING id`,
		token, req.GuestID, req.GroupID,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "token": token})
}

func DeleteLink(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	db.DB.Exec(`DELETE FROM invitation_links WHERE id=$1`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
