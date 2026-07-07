package handlers

import (
	"net/http"
	"strconv"
	"wedding/db"

	"github.com/gin-gonic/gin"
)

// ListComments returns all guest comments (friends who left a comment or
// already received a reply), together with any admin reply.
func ListComments(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT g.id, g.last_name, g.first_name, g.middle_name, g.role,
		       fr.comment,
		       to_char(fr.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS updated_at,
		       fr.admin_reply,
		       to_char(fr.admin_reply_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS admin_reply_at
		FROM friend_responses fr
		JOIN guests g ON g.id = fr.guest_id
		WHERE (fr.comment IS NOT NULL AND fr.comment <> '')
		   OR (fr.admin_reply IS NOT NULL AND fr.admin_reply <> '')
		ORDER BY fr.updated_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type CommentRow struct {
		GuestID      int     `json:"guest_id"`
		LastName     string  `json:"last_name"`
		FirstName    string  `json:"first_name"`
		MiddleName   string  `json:"middle_name"`
		Role         string  `json:"role"`
		Comment      string  `json:"comment"`
		UpdatedAt    *string `json:"updated_at"`
		AdminReply   string  `json:"admin_reply"`
		AdminReplyAt *string `json:"admin_reply_at"`
	}
	var result []CommentRow
	for rows.Next() {
		var r CommentRow
		rows.Scan(&r.GuestID, &r.LastName, &r.FirstName, &r.MiddleName, &r.Role,
			&r.Comment, &r.UpdatedAt, &r.AdminReply, &r.AdminReplyAt)
		result = append(result, r)
	}
	if result == nil {
		result = []CommentRow{}
	}
	c.JSON(http.StatusOK, result)
}

// ReplyComment stores (or clears) the admin's reply for a guest's comment.
func ReplyComment(c *gin.Context) {
	guestID, err := strconv.Atoi(c.Param("guestId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid guest id"})
		return
	}

	var req struct {
		Reply string `json:"reply"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only friends have friend_responses rows; ensure one exists.
	res, err := db.DB.Exec(`
		UPDATE friend_responses
		SET admin_reply = $1,
		    admin_reply_at = CASE WHEN $1 = '' THEN NULL ELSE NOW() END
		WHERE guest_id = $2`,
		req.Reply, guestID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "у гостя нет ответа для комментария"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
