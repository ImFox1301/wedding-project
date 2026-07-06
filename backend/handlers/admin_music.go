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

func ListMusic(c *gin.Context) {
	role := c.Query("role")
	query := `SELECT id, role, filename, original_name, file_order FROM music_files`
	args := []interface{}{}
	if role != "" {
		query += ` WHERE role=$1`
		args = append(args, role)
	}
	query += ` ORDER BY file_order ASC, id ASC`

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type MusicRow struct {
		ID           int    `json:"id"`
		Role         string `json:"role"`
		Filename     string `json:"filename"`
		OriginalName string `json:"original_name"`
		Order        int    `json:"order"`
	}
	var music []MusicRow
	for rows.Next() {
		var m MusicRow
		rows.Scan(&m.ID, &m.Role, &m.Filename, &m.OriginalName, &m.Order)
		music = append(music, m)
	}
	if music == nil {
		music = []MusicRow{}
	}
	c.JSON(http.StatusOK, music)
}

func UploadMusic(c *gin.Context) {
	role := c.PostForm("role")
	if role != "friends" && role != "family" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be friends or family"})
		return
	}

	file, err := c.FormFile("music")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "music file required"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".mp3": true, ".ogg": true, ".wav": true, ".flac": true, ".m4a": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file type"})
		return
	}

	filename := fmt.Sprintf("music_%s_%d%s", role, time.Now().UnixNano(), ext)
	uploadPath := "/app/uploads/" + filename
	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		os.MkdirAll("uploads", 0755)
		uploadPath = "uploads/" + filename
		if err := c.SaveUploadedFile(file, uploadPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	var id int
	db.DB.QueryRow(
		`INSERT INTO music_files (role, filename, original_name) VALUES ($1,$2,$3) RETURNING id`,
		role, filename, file.Filename,
	).Scan(&id)
	c.JSON(http.StatusCreated, gin.H{"id": id, "filename": filename})
}

func DeleteMusic(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var filename string
	db.DB.QueryRow(`SELECT filename FROM music_files WHERE id=$1`, id).Scan(&filename)
	db.DB.Exec(`DELETE FROM music_files WHERE id=$1`, id)
	os.Remove("/app/uploads/" + filename)
	os.Remove("uploads/" + filename)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func UpdateMusicOrder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Order int `json:"order"`
	}
	c.ShouldBindJSON(&req)
	db.DB.Exec(`UPDATE music_files SET file_order=$1 WHERE id=$2`, req.Order, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
