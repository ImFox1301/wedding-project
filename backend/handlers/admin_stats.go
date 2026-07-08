package handlers

import (
	"net/http"
	"wedding/db"

	"github.com/gin-gonic/gin"
)

func StatsVisits(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT g.id, g.last_name, g.first_name, g.role, g.visited,
		       to_char(g.visited_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as visited_at
		FROM guests g
		ORDER BY g.role, g.last_name, g.first_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type VisitRow struct {
		ID        int     `json:"id"`
		LastName  string  `json:"last_name"`
		FirstName string  `json:"first_name"`
		Role      string  `json:"role"`
		Visited   bool    `json:"visited"`
		VisitedAt *string `json:"visited_at"`
	}
	var result []VisitRow
	for rows.Next() {
		var r VisitRow
		rows.Scan(&r.ID, &r.LastName, &r.FirstName, &r.Role, &r.Visited, &r.VisitedAt)
		result = append(result, r)
	}
	if result == nil {
		result = []VisitRow{}
	}
	c.JSON(http.StatusOK, result)
}

func StatsCottage(c *gin.Context) {
	// Get cottage date range from settings
	var dateFrom, dateTo string
	db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='cottage_date_from'`).Scan(&dateFrom)
	db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='cottage_date_to'`).Scan(&dateTo)

	rows, err := db.DB.Query(`
		SELECT fr.cottage_date_from::text, fr.cottage_date_to::text, COUNT(*) as count
		FROM friend_responses fr
		WHERE fr.going_cottage = TRUE
		  AND fr.cottage_date_from IS NOT NULL
		GROUP BY fr.cottage_date_from, fr.cottage_date_to
		ORDER BY fr.cottage_date_from
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type DateRange struct {
		DateFrom string `json:"date_from"`
		DateTo   string `json:"date_to"`
		Count    int    `json:"count"`
	}
	var ranges []DateRange
	for rows.Next() {
		var r DateRange
		rows.Scan(&r.DateFrom, &r.DateTo, &r.Count)
		ranges = append(ranges, r)
	}
	if ranges == nil {
		ranges = []DateRange{}
	}

	// Also return per-date count for chart
	dateRows, _ := db.DB.Query(`
		SELECT g.last_name, g.first_name, fr.cottage_date_from::text, fr.cottage_date_to::text
		FROM friend_responses fr
		JOIN guests g ON g.id = fr.guest_id
		WHERE fr.going_cottage = TRUE AND fr.cottage_date_from IS NOT NULL
		ORDER BY fr.cottage_date_from
	`)
	type GuestDateRow struct {
		LastName  string `json:"last_name"`
		FirstName string `json:"first_name"`
		DateFrom  string `json:"date_from"`
		DateTo    string `json:"date_to"`
	}
	var guestDates []GuestDateRow
	if dateRows != nil {
		defer dateRows.Close()
		for dateRows.Next() {
			var r GuestDateRow
			dateRows.Scan(&r.LastName, &r.FirstName, &r.DateFrom, &r.DateTo)
			guestDates = append(guestDates, r)
		}
	}
	if guestDates == nil {
		guestDates = []GuestDateRow{}
	}

	c.JSON(http.StatusOK, gin.H{
		"date_from":   dateFrom,
		"date_to":     dateTo,
		"ranges":      ranges,
		"guest_dates": guestDates,
	})
}

func StatsTournament(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT g.last_name, g.first_name,
		       COALESCE(opp.last_name || ' ' || opp.first_name, '') as opponent_name
		FROM friend_responses fr
		JOIN guests g ON g.id = fr.guest_id
		LEFT JOIN guests opp ON opp.id = fr.preferred_opponent_id
		WHERE fr.tournament = TRUE
		ORDER BY g.last_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type TournRow struct {
		LastName     string `json:"last_name"`
		FirstName    string `json:"first_name"`
		OpponentName string `json:"opponent_name"`
	}
	var result []TournRow
	for rows.Next() {
		var r TournRow
		rows.Scan(&r.LastName, &r.FirstName, &r.OpponentName)
		result = append(result, r)
	}
	if result == nil {
		result = []TournRow{}
	}

	// Total count
	var total int
	db.DB.QueryRow(`SELECT COUNT(*) FROM friend_responses WHERE tournament=TRUE`).Scan(&total)

	c.JSON(http.StatusOK, gin.H{"participants": result, "total": total})
}

// StatsAttendance returns "Приду / Не приду" counts per role.
func StatsAttendance(c *gin.Context) {
	roleStats := func(role, table string) gin.H {
		var total, coming, declined int
		db.DB.QueryRow(`SELECT COUNT(*) FROM guests WHERE role=$1`, role).Scan(&total)
		db.DB.QueryRow(`
			SELECT COUNT(*) FROM `+table+` r JOIN guests g ON g.id=r.guest_id
			WHERE g.role=$1 AND r.attending=TRUE`, role).Scan(&coming)
		db.DB.QueryRow(`
			SELECT COUNT(*) FROM `+table+` r JOIN guests g ON g.id=r.guest_id
			WHERE g.role=$1 AND r.attending=FALSE`, role).Scan(&declined)
		undecided := total - coming - declined
		if undecided < 0 {
			undecided = 0
		}
		return gin.H{"total": total, "coming": coming, "declined": declined, "undecided": undecided}
	}

	c.JSON(http.StatusOK, gin.H{
		"friends": roleStats("friends", "friend_responses"),
		"family":  roleStats("family", "family_responses"),
	})
}

func StatsLoft(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT g.last_name, g.first_name, fr.going_loft, fr.needs_transport
		FROM family_responses fr
		JOIN guests g ON g.id = fr.guest_id
		ORDER BY g.last_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type LoftRow struct {
		LastName       string `json:"last_name"`
		FirstName      string `json:"first_name"`
		GoingLoft      bool   `json:"going_loft"`
		NeedsTransport bool   `json:"needs_transport"`
	}
	var result []LoftRow
	for rows.Next() {
		var r LoftRow
		rows.Scan(&r.LastName, &r.FirstName, &r.GoingLoft, &r.NeedsTransport)
		result = append(result, r)
	}
	if result == nil {
		result = []LoftRow{}
	}

	var goingCount, transportCount int
	db.DB.QueryRow(`SELECT COUNT(*) FROM family_responses WHERE going_loft=TRUE`).Scan(&goingCount)
	db.DB.QueryRow(`SELECT COUNT(*) FROM family_responses WHERE needs_transport=TRUE`).Scan(&transportCount)

	c.JSON(http.StatusOK, gin.H{
		"guests":          result,
		"going_count":     goingCount,
		"transport_count": transportCount,
	})
}
