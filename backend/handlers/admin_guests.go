package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"wedding/db"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type guestRequest struct {
	LastName         string `json:"last_name" binding:"required"`
	FirstName        string `json:"first_name" binding:"required"`
	MiddleName       string `json:"middle_name"`
	Login            string `json:"login"`   // auto-generated if empty
	Password         string `json:"password"` // auto-generated on create if empty
	Role             string `json:"role" binding:"required"`
	Gender           string `json:"gender"`
	CustomSalutation string `json:"custom_salutation"`
	Subtitle         string `json:"subtitle"`
	AmIGosha         bool   `json:"am_i_gosha"`
	GroupID          *int   `json:"group_id"`
}

func ListGuests(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT g.id, g.last_name, g.first_name, g.middle_name, g.login, g.role,
		       g.gender, g.custom_salutation, g.am_i_gosha, g.group_id, g.visited,
		       to_char(g.visited_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as visited_at,
		       to_char(g.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at,
		       gg.name as group_name
		FROM guests g
		LEFT JOIN guest_groups gg ON gg.id = g.group_id
		ORDER BY g.last_name, g.first_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type GuestRow struct {
		ID               int     `json:"id"`
		LastName         string  `json:"last_name"`
		FirstName        string  `json:"first_name"`
		MiddleName       string  `json:"middle_name"`
		Login            string  `json:"login"`
		Role             string  `json:"role"`
		Gender           string  `json:"gender"`
		CustomSalutation string  `json:"custom_salutation"`
		AmIGosha         bool    `json:"am_i_gosha"`
		GroupID          *int    `json:"group_id"`
		GroupName        *string `json:"group_name"`
		Visited          bool    `json:"visited"`
		VisitedAt        *string `json:"visited_at"`
		CreatedAt        string  `json:"created_at"`
	}

	var guests []GuestRow
	for rows.Next() {
		var g GuestRow
		if err := rows.Scan(
			&g.ID, &g.LastName, &g.FirstName, &g.MiddleName, &g.Login,
			&g.Role, &g.Gender, &g.CustomSalutation,
			&g.AmIGosha, &g.GroupID, &g.Visited, &g.VisitedAt, &g.CreatedAt,
			&g.GroupName,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		guests = append(guests, g)
	}
	if guests == nil {
		guests = []GuestRow{}
	}
	c.JSON(http.StatusOK, guests)
}

func CreateGuest(c *gin.Context) {
	var req guestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Login == "" {
		req.Login = fmt.Sprintf("guest_%d", time.Now().UnixNano())
	}
	if req.Password == "" {
		req.Password = fmt.Sprintf("%x", time.Now().UnixNano()+42)
	}
	if req.Gender == "" {
		req.Gender = "male"
	}
	// A group must be single-role: don't add a guest to a group of a different role.
	if req.GroupID != nil {
		var mismatch int
		db.DB.QueryRow(`SELECT COUNT(*) FROM guests WHERE group_id=$1 AND role<>$2`, *req.GroupID, req.Role).Scan(&mismatch)
		if mismatch > 0 {
			req.GroupID = nil
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var id int
	err = db.DB.QueryRow(`
		INSERT INTO guests (last_name, first_name, middle_name, login, password_hash, role, gender, custom_salutation, subtitle, am_i_gosha, group_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		req.LastName, req.FirstName, req.MiddleName, req.Login, string(hash),
		req.Role, req.Gender, req.CustomSalutation, req.Subtitle, req.AmIGosha, req.GroupID,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func UpdateGuest(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req guestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Gender == "" {
		req.Gender = "male"
	}

	// A group must be single-role. If the (possibly changed) role no longer
	// matches the group's other members, exclude the guest from the group.
	if req.GroupID != nil {
		var mismatch int
		db.DB.QueryRow(`SELECT COUNT(*) FROM guests WHERE group_id=$1 AND id<>$2 AND role<>$3`,
			*req.GroupID, id, req.Role).Scan(&mismatch)
		if mismatch > 0 {
			req.GroupID = nil
		}
	}

	// Preserve existing login if not provided
	if req.Login == "" {
		db.DB.QueryRow(`SELECT login FROM guests WHERE id=$1`, id).Scan(&req.Login)
	}

	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		_, err = db.DB.Exec(`
			UPDATE guests SET last_name=$1, first_name=$2, middle_name=$3, login=$4,
			password_hash=$5, role=$6, gender=$7, custom_salutation=$8, subtitle=$9, am_i_gosha=$10, group_id=$11 WHERE id=$12`,
			req.LastName, req.FirstName, req.MiddleName, req.Login,
			string(hash), req.Role, req.Gender, req.CustomSalutation, req.Subtitle, req.AmIGosha, req.GroupID, id,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		_, err := db.DB.Exec(`
			UPDATE guests SET last_name=$1, first_name=$2, middle_name=$3, login=$4,
			role=$5, gender=$6, custom_salutation=$7, subtitle=$8, am_i_gosha=$9, group_id=$10 WHERE id=$11`,
			req.LastName, req.FirstName, req.MiddleName, req.Login,
			req.Role, req.Gender, req.CustomSalutation, req.Subtitle, req.AmIGosha, req.GroupID, id,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	// A guest removed from a group loses their group gift pick.
	if req.GroupID == nil {
		db.DB.Exec(`DELETE FROM group_gift_picks WHERE guest_id=$1`, id)
	}
	// Added to a group → drop the guest's personal invitation link and release
	// any single (exclusive) gift pick. Group links are never auto-deleted (not
	// even when the last member leaves) — they go only manually or on cascade.
	if req.GroupID != nil {
		db.DB.Exec(`DELETE FROM invitation_links WHERE guest_id=$1`, id)
		db.DB.Exec(`UPDATE gifts SET selected_by_guest_id=NULL WHERE selected_by_guest_id=$1`, id)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DeleteGuest(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := db.DB.Exec(`DELETE FROM guests WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func GetGuest(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	type GuestRow struct {
		ID               int    `json:"id"`
		LastName         string `json:"last_name"`
		FirstName        string `json:"first_name"`
		MiddleName       string `json:"middle_name"`
		Login            string `json:"login"`
		Role             string `json:"role"`
		Gender           string `json:"gender"`
		CustomSalutation string `json:"custom_salutation"`
		Subtitle         string `json:"subtitle"`
		AmIGosha         bool   `json:"am_i_gosha"`
		GroupID          *int   `json:"group_id"`
		Visited          bool   `json:"visited"`
		CreatedAt        string `json:"created_at"`
	}
	var g GuestRow
	err := db.DB.QueryRow(`
		SELECT id, last_name, first_name, middle_name, login, role, gender, custom_salutation, subtitle,
		       am_i_gosha, group_id, visited,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM guests WHERE id=$1`, id,
	).Scan(&g.ID, &g.LastName, &g.FirstName, &g.MiddleName, &g.Login,
		&g.Role, &g.Gender, &g.CustomSalutation, &g.Subtitle,
		&g.AmIGosha, &g.GroupID, &g.Visited, &g.CreatedAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, g)
}
