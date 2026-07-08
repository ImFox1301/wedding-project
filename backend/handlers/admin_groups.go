package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"wedding/db"

	"github.com/gin-gonic/gin"
)

func ListGroups(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT id, name, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM guest_groups ORDER BY name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type GroupRow struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
		Members   []struct {
			ID        int    `json:"id"`
			FullName  string `json:"full_name"`
		} `json:"members"`
	}

	var groups []GroupRow
	groupMap := map[int]*GroupRow{}
	var order []int
	for rows.Next() {
		var g GroupRow
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		g.Members = []struct {
			ID       int    `json:"id"`
			FullName string `json:"full_name"`
		}{}
		groupMap[g.ID] = &g
		order = append(order, g.ID)
	}

	// Load members
	mrows, err := db.DB.Query(`SELECT id, group_id, last_name || ' ' || first_name FROM guests WHERE group_id IS NOT NULL`)
	if err == nil {
		defer mrows.Close()
		for mrows.Next() {
			var gid, mid int
			var name string
			_ = mrows.Scan(&mid, &gid, &name)
			if gr, ok := groupMap[gid]; ok {
				gr.Members = append(gr.Members, struct {
					ID       int    `json:"id"`
					FullName string `json:"full_name"`
				}{mid, name})
			}
		}
	}

	for _, id := range order {
		groups = append(groups, *groupMap[id])
	}
	if groups == nil {
		groups = []GroupRow{}
	}
	c.JSON(http.StatusOK, groups)
}

func CreateGroup(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var id int
	err := db.DB.QueryRow(`INSERT INTO guest_groups (name) VALUES ($1) RETURNING id`, req.Name).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func UpdateGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Name             string `json:"name"`
		Subtitle         string `json:"subtitle"`
		CustomSalutation string `json:"custom_salutation"`
		Members          []int  `json:"members"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate: all members must have the same role
	if len(req.Members) > 1 {
		rows, err := db.DB.Query(`SELECT DISTINCT role FROM guests WHERE id = ANY($1::int[])`,
			intSliceToArray(req.Members))
		if err == nil {
			defer rows.Close()
			var roles []string
			for rows.Next() {
				var r string
				rows.Scan(&r)
				roles = append(roles, r)
			}
			if len(roles) > 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "все участники группы должны иметь одинаковую роль (Друзья или Семья)"})
				return
			}
		}
	}

	if req.Name != "" {
		db.DB.Exec(`UPDATE guest_groups SET name=$1 WHERE id=$2`, req.Name, id)
	}
	db.DB.Exec(`UPDATE guest_groups SET subtitle=$1, custom_salutation=$2 WHERE id=$3`, req.Subtitle, req.CustomSalutation, id)
	// Remove all current members
	db.DB.Exec(`UPDATE guests SET group_id=NULL WHERE group_id=$1`, id)
	// Assign new members — but never steal a guest who is already in another
	// group; they must be removed from it manually first.
	for _, gid := range req.Members {
		db.DB.Exec(`UPDATE guests SET group_id=$1 WHERE id=$2 AND (group_id IS NULL OR group_id=$1)`, id, gid)
	}
	// Guests removed from any group lose their group gift pick
	db.DB.Exec(`DELETE FROM group_gift_picks WHERE guest_id IN (SELECT id FROM guests WHERE group_id IS NULL)`)

	// Guests now in this group lose their personal invitation link and any single
	// (exclusive) gift pick. The group link is never auto-deleted (removed only
	// manually or on cascade), even if the group ends up with one/zero members.
	db.DB.Exec(`DELETE FROM invitation_links WHERE guest_id IN (SELECT id FROM guests WHERE group_id=$1)`, id)
	db.DB.Exec(`UPDATE gifts SET selected_by_guest_id=NULL WHERE selected_by_guest_id IN (SELECT id FROM guests WHERE group_id=$1)`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// intSliceToArray converts []int to a PostgreSQL array literal for ANY()
func intSliceToArray(ids []int) interface{} {
	// Use pq.Array for proper PostgreSQL array binding
	// Since we can't import pq here easily, build the array manually
	if len(ids) == 0 {
		return "{}"
	}
	s := "{"
	for i, id := range ids {
		if i > 0 {
			s += ","
		}
		s += strconv.Itoa(id)
	}
	s += "}"
	return s
}

func DeleteGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var exists bool
	db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM guest_groups WHERE id=$1)`, id).Scan(&exists)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	db.DB.Exec(`UPDATE guests SET group_id=NULL WHERE group_id=$1`, id)
	db.DB.Exec(`DELETE FROM group_gift_picks WHERE guest_id IN (SELECT id FROM guests WHERE group_id IS NULL)`)
	db.DB.Exec(`DELETE FROM guest_groups WHERE id=$1`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func GetGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var name, createdAt, subtitle, customSalutation string
	err := db.DB.QueryRow(`SELECT name, subtitle, custom_salutation, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM guest_groups WHERE id=$1`, id).Scan(&name, &subtitle, &customSalutation, &createdAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	rows, _ := db.DB.Query(`SELECT id, last_name, first_name FROM guests WHERE group_id=$1`, id)
	defer rows.Close()
	type Member struct {
		ID        int    `json:"id"`
		LastName  string `json:"last_name"`
		FirstName string `json:"first_name"`
	}
	var members []Member
	for rows.Next() {
		var m Member
		rows.Scan(&m.ID, &m.LastName, &m.FirstName)
		members = append(members, m)
	}
	if members == nil {
		members = []Member{}
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "name": name, "subtitle": subtitle, "custom_salutation": customSalutation, "created_at": createdAt, "members": members})
}
