package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"wedding/db"

	"github.com/gin-gonic/gin"
)

func ListSections(c *gin.Context) {
	role := c.Query("role")
	// Common sections only (personal ones live under "Персональные разделы").
	query := `SELECT id, role, title, content, section_order, created_at FROM page_sections
	          WHERE guest_id IS NULL AND group_id IS NULL`
	args := []interface{}{}
	if role != "" {
		query += ` AND role=$1`
		args = append(args, role)
	}
	query += ` ORDER BY section_order ASC`

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type SectionRow struct {
		ID        int    `json:"id"`
		Role      string `json:"role"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		Order     int    `json:"order"`
		CreatedAt string `json:"created_at"`
		Photos    []struct {
			ID       int    `json:"id"`
			Filename string `json:"filename"`
			Order    int    `json:"order"`
		} `json:"photos"`
	}

	sectionMap := map[int]*SectionRow{}
	var order []int
	for rows.Next() {
		var s SectionRow
		if err := rows.Scan(&s.ID, &s.Role, &s.Title, &s.Content, &s.Order, &s.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		s.Photos = []struct {
			ID       int    `json:"id"`
			Filename string `json:"filename"`
			Order    int    `json:"order"`
		}{}
		sectionMap[s.ID] = &s
		order = append(order, s.ID)
	}

	// Load photos
	pRows, _ := db.DB.Query(`SELECT id, section_id, filename, photo_order FROM section_photos ORDER BY photo_order`)
	if pRows != nil {
		defer pRows.Close()
		for pRows.Next() {
			var id, sid, ord int
			var fn string
			pRows.Scan(&id, &sid, &fn, &ord)
			if s, ok := sectionMap[sid]; ok {
				s.Photos = append(s.Photos, struct {
					ID       int    `json:"id"`
					Filename string `json:"filename"`
					Order    int    `json:"order"`
				}{id, fn, ord})
			}
		}
	}

	var sections []SectionRow
	for _, id := range order {
		sections = append(sections, *sectionMap[id])
	}
	if sections == nil {
		sections = []SectionRow{}
	}
	c.JSON(http.StatusOK, sections)
}

// ListPersonalSections returns all guest/group personal sections with a lock flag.
func ListPersonalSections(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT ps.id, ps.role, ps.title, ps.content, ps.guest_id, ps.group_id,
		       COALESCE(CASE WHEN ps.guest_id IS NOT NULL
		                     THEN g.last_name || ' ' || g.first_name
		                     ELSE gg.name END, '') AS target_name,
		       CASE WHEN ps.guest_id IS NOT NULL THEN 'guest' ELSE 'group' END AS target_type,
		       (ps.guest_id IS NOT NULL AND g.group_id IS NOT NULL) AS locked
		FROM page_sections ps
		LEFT JOIN guests g ON g.id = ps.guest_id
		LEFT JOIN guest_groups gg ON gg.id = ps.group_id
		WHERE ps.guest_id IS NOT NULL OR ps.group_id IS NOT NULL
		ORDER BY target_type, target_name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Row struct {
		ID         int    `json:"id"`
		Role       string `json:"role"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		GuestID    *int   `json:"guest_id"`
		GroupID    *int   `json:"group_id"`
		TargetName string `json:"target_name"`
		TargetType string `json:"target_type"`
		Locked     bool   `json:"locked"`
		Photos     []struct {
			ID       int    `json:"id"`
			Filename string `json:"filename"`
			Order    int    `json:"order"`
		} `json:"photos"`
	}
	secMap := map[int]*Row{}
	var order []int
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.Role, &r.Title, &r.Content, &r.GuestID, &r.GroupID,
			&r.TargetName, &r.TargetType, &r.Locked); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		r.Photos = []struct {
			ID       int    `json:"id"`
			Filename string `json:"filename"`
			Order    int    `json:"order"`
		}{}
		secMap[r.ID] = &r
		order = append(order, r.ID)
	}

	pRows, _ := db.DB.Query(`SELECT id, section_id, filename, photo_order FROM section_photos ORDER BY photo_order`)
	if pRows != nil {
		defer pRows.Close()
		for pRows.Next() {
			var id, sid, ord int
			var fn string
			pRows.Scan(&id, &sid, &fn, &ord)
			if s, ok := secMap[sid]; ok {
				s.Photos = append(s.Photos, struct {
					ID       int    `json:"id"`
					Filename string `json:"filename"`
					Order    int    `json:"order"`
				}{id, fn, ord})
			}
		}
	}

	var out []Row
	for _, id := range order {
		out = append(out, *secMap[id])
	}
	if out == nil {
		out = []Row{}
	}
	c.JSON(http.StatusOK, out)
}

// CreatePersonalSection creates one personal section for a guest or a group.
func CreatePersonalSection(c *gin.Context) {
	var req struct {
		GuestID *int   `json:"guest_id"`
		GroupID *int   `json:"group_id"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if (req.GuestID == nil) == (req.GroupID == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "укажите гостя ИЛИ группу"})
		return
	}

	var role string
	if req.GuestID != nil {
		var grp *int
		if err := db.DB.QueryRow(`SELECT role, group_id FROM guests WHERE id=$1`, *req.GuestID).Scan(&role, &grp); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "гость не найден"})
			return
		}
		if grp != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "гость состоит в группе — создайте раздел для группы"})
			return
		}
		var exists bool
		db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM page_sections WHERE guest_id=$1)`, *req.GuestID).Scan(&exists)
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "персональный раздел для гостя уже существует"})
			return
		}
	} else {
		db.DB.QueryRow(`SELECT role FROM guests WHERE group_id=$1 LIMIT 1`, *req.GroupID).Scan(&role)
		if role == "" {
			role = "family"
		}
		var exists bool
		db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM page_sections WHERE group_id=$1)`, *req.GroupID).Scan(&exists)
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "персональный раздел для группы уже существует"})
			return
		}
	}

	var id int
	err := db.DB.QueryRow(
		`INSERT INTO page_sections (role, title, content, section_order, guest_id, group_id)
		 VALUES ($1,$2,$3,0,$4,$5) RETURNING id`,
		role, req.Title, req.Content, req.GuestID, req.GroupID,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func CreateSection(c *gin.Context) {
	var req struct {
		Role    string `json:"role" binding:"required"`
		Title   string `json:"title"`
		Content string `json:"content"`
		Order   int    `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var id int
	err := db.DB.QueryRow(
		`INSERT INTO page_sections (role, title, content, section_order) VALUES ($1,$2,$3,$4) RETURNING id`,
		req.Role, req.Title, req.Content, req.Order,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func UpdateSection(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Order   int    `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// A personal section of a guest who is now in a group is locked (read-only).
	var locked bool
	db.DB.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM page_sections ps JOIN guests g ON g.id = ps.guest_id
		WHERE ps.id=$1 AND g.group_id IS NOT NULL)`, id).Scan(&locked)
	if locked {
		c.JSON(http.StatusConflict, gin.H{"error": "раздел заблокирован: гость состоит в группе"})
		return
	}
	_, err := db.DB.Exec(
		`UPDATE page_sections SET title=$1, content=$2, section_order=$3 WHERE id=$4`,
		req.Title, req.Content, req.Order, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func UpdateSectionOrder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Order int `json:"order"`
	}
	c.ShouldBindJSON(&req)
	db.DB.Exec(`UPDATE page_sections SET section_order=$1 WHERE id=$2`, req.Order, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DeleteSection(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	db.DB.Exec(`DELETE FROM page_sections WHERE id=$1`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func UploadSectionPhoto(c *gin.Context) {
	sectionID, _ := strconv.Atoi(c.Param("id"))
	file, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "photo required"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file type"})
		return
	}

	filename := fmt.Sprintf("photo_%d_%d%s", sectionID, time.Now().UnixNano(), ext)
	uploadPath := "/app/uploads/" + filename
	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		// Fallback for local dev
		os.MkdirAll("uploads", 0755)
		uploadPath = "uploads/" + filename
		if err := c.SaveUploadedFile(file, uploadPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	var id int
	db.DB.QueryRow(`INSERT INTO section_photos (section_id, filename) VALUES ($1,$2) RETURNING id`, sectionID, filename).Scan(&id)
	c.JSON(http.StatusCreated, gin.H{"id": id, "filename": filename})
}

func DeleteSectionPhoto(c *gin.Context) {
	photoID, _ := strconv.Atoi(c.Param("photoId"))
	var filename string
	db.DB.QueryRow(`SELECT filename FROM section_photos WHERE id=$1`, photoID).Scan(&filename)
	db.DB.Exec(`DELETE FROM section_photos WHERE id=$1`, photoID)
	// Try to remove file
	os.Remove("/app/uploads/" + filename)
	os.Remove("uploads/" + filename)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
