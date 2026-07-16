package handlers

import (
	"net/http"
	"strconv"
	"wedding/db"

	"github.com/gin-gonic/gin"
)

type Drink struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

// ListDrinks returns all drinks ordered for display.
func ListDrinks(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT id, name, sort_order FROM drinks ORDER BY sort_order, id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	drinks := []Drink{}
	for rows.Next() {
		var d Drink
		rows.Scan(&d.ID, &d.Name, &d.SortOrder)
		drinks = append(drinks, d)
	}
	c.JSON(http.StatusOK, drinks)
}

// CreateDrink adds a drink to the list.
func CreateDrink(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "укажите название напитка"})
		return
	}
	var id int
	err := db.DB.QueryRow(
		`INSERT INTO drinks (name, sort_order)
		 VALUES ($1, COALESCE((SELECT MAX(sort_order)+1 FROM drinks), 0))
		 RETURNING id`, req.Name,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

// UpdateDrink renames a drink.
func UpdateDrink(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "укажите название напитка"})
		return
	}
	if _, err := db.DB.Exec(`UPDATE drinks SET name=$1 WHERE id=$2`, req.Name, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteDrink removes a drink (guest picks cascade).
func DeleteDrink(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if _, err := db.DB.Exec(`DELETE FROM drinks WHERE id=$1`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListDrinkComments returns guests' drink comments (non-empty only).
func ListDrinkComments(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT g.id, g.last_name, g.first_name, g.middle_name, g.role,
		       fr.drinks_comment, fr.updated_at
		FROM friend_responses fr
		JOIN guests g ON g.id = fr.guest_id
		WHERE TRIM(fr.drinks_comment) <> ''
		ORDER BY fr.updated_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	out := []gin.H{}
	for rows.Next() {
		var id int
		var ln, fn, mn, role, comment string
		var updatedAt *string
		rows.Scan(&id, &ln, &fn, &mn, &role, &comment, &updatedAt)
		out = append(out, gin.H{
			"guest_id":    id,
			"last_name":   ln,
			"first_name":  fn,
			"middle_name": mn,
			"role":        role,
			"comment":     comment,
			"updated_at":  updatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

// StatsDrinks returns pick counts per drink (only guests going to the cottage).
func StatsDrinks(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT d.id, d.name, COUNT(fr.guest_id) AS cnt
		FROM drinks d
		LEFT JOIN guest_drinks gd ON gd.drink_id = d.id
		LEFT JOIN friend_responses fr ON fr.guest_id = gd.guest_id AND fr.going_cottage = TRUE
		GROUP BY d.id, d.name
		ORDER BY d.sort_order, d.id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type row struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	out := []row{}
	for rows.Next() {
		var r row
		rows.Scan(&r.ID, &r.Name, &r.Count)
		out = append(out, r)
	}
	c.JSON(http.StatusOK, gin.H{"drinks": out})
}
