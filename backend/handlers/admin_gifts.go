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

func ListGifts(c *gin.Context) {
	role := c.Query("role")
	query := `
		SELECT g.id, g.name, g.description, g.role, g.is_pickable, g.selected_by_guest_id,
		       COALESCE(gu.last_name || ' ' || gu.first_name, '') as selected_by_name,
		       g.photo_filename, g.link_url
		FROM gifts g
		LEFT JOIN guests gu ON gu.id = g.selected_by_guest_id
	`
	args := []interface{}{}
	if role != "" {
		query += ` WHERE g.role=$1`
		args = append(args, role)
	}
	query += ` ORDER BY g.id`

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type GiftRow struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Description       string `json:"description"`
		Role              string `json:"role"`
		IsPickable        bool   `json:"is_pickable"`
		SelectedByGuestID *int   `json:"selected_by_guest_id"`
		SelectedByName    string `json:"selected_by_name"`
		PhotoFilename     string `json:"photo_filename"`
		LinkURL           string `json:"link_url"`
	}

	var gifts []GiftRow
	for rows.Next() {
		var g GiftRow
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Role, &g.IsPickable,
			&g.SelectedByGuestID, &g.SelectedByName, &g.PhotoFilename, &g.LinkURL); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		gifts = append(gifts, g)
	}
	if gifts == nil {
		gifts = []GiftRow{}
	}
	c.JSON(http.StatusOK, gifts)
}

func CreateGift(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Role        string `json:"role" binding:"required"`
		IsPickable  bool   `json:"is_pickable"`
		LinkURL     string `json:"link_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var id int
	err := db.DB.QueryRow(
		`INSERT INTO gifts (name, description, role, is_pickable, link_url) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		req.Name, req.Description, req.Role, req.IsPickable, req.LinkURL,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func UpdateGift(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Role        string `json:"role"`
		IsPickable  bool   `json:"is_pickable"`
		LinkURL     string `json:"link_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := db.DB.Exec(
		`UPDATE gifts SET name=$1, description=$2, role=$3, is_pickable=$4, link_url=$5 WHERE id=$6`,
		req.Name, req.Description, req.Role, req.IsPickable, req.LinkURL, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DeleteGift(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	// Remove photo file if exists
	var photoFilename string
	db.DB.QueryRow(`SELECT photo_filename FROM gifts WHERE id=$1`, id).Scan(&photoFilename)
	if photoFilename != "" {
		os.Remove("/app/uploads/" + photoFilename)
		os.Remove("uploads/" + photoFilename)
	}
	db.DB.Exec(`DELETE FROM gifts WHERE id=$1`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func UploadGiftPhoto(c *gin.Context) {
	giftID, _ := strconv.Atoi(c.Param("id"))
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

	// Remove old photo if exists
	var oldFilename string
	db.DB.QueryRow(`SELECT photo_filename FROM gifts WHERE id=$1`, giftID).Scan(&oldFilename)
	if oldFilename != "" {
		os.Remove("/app/uploads/" + oldFilename)
		os.Remove("uploads/" + oldFilename)
	}

	filename := fmt.Sprintf("gift_%d_%d%s", giftID, time.Now().UnixNano(), ext)
	uploadPath := "/app/uploads/" + filename
	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		os.MkdirAll("uploads", 0755)
		uploadPath = "uploads/" + filename
		if err := c.SaveUploadedFile(file, uploadPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	db.DB.Exec(`UPDATE gifts SET photo_filename=$1 WHERE id=$2`, filename, giftID)
	c.JSON(http.StatusOK, gin.H{"filename": filename})
}

func DeleteGiftPhoto(c *gin.Context) {
	giftID, _ := strconv.Atoi(c.Param("id"))
	var filename string
	db.DB.QueryRow(`SELECT photo_filename FROM gifts WHERE id=$1`, giftID).Scan(&filename)
	if filename != "" {
		os.Remove("/app/uploads/" + filename)
		os.Remove("uploads/" + filename)
		db.DB.Exec(`UPDATE gifts SET photo_filename='' WHERE id=$1`, giftID)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
