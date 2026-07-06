package handlers

import (
	"net/http"
	"wedding/db"

	"github.com/gin-gonic/gin"
)

func GetSettings(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT key, value FROM admin_settings ORDER BY key`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	settings := map[string]string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		settings[k] = v
	}
	c.JSON(http.StatusOK, settings)
}

func UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for k, v := range req {
		db.DB.Exec(`INSERT INTO admin_settings (key, value) VALUES ($1,$2) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, k, v)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
