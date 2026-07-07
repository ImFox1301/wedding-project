package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"wedding/db"

	"github.com/gin-gonic/gin"
)


// GetInvitePage resolves a token, marks visit, returns page data for the guest
func GetInvitePage(c *gin.Context) {
	token := c.Param("token")

	// Find link
	var linkID int
	var guestID *int
	var groupID *int
	err := db.DB.QueryRow(
		`SELECT id, guest_id, group_id FROM invitation_links WHERE token=$1`, token,
	).Scan(&linkID, &guestID, &groupID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "ссылка не найдена"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Collect guests for this link (1 guest or group)
	type GuestInfo struct {
		ID               int    `json:"id"`
		LastName         string `json:"last_name"`
		FirstName        string `json:"first_name"`
		MiddleName       string `json:"middle_name"`
		Role             string `json:"role"`
		Gender           string `json:"gender"`
		CustomSalutation string `json:"custom_salutation"`
		AmIGosha         bool   `json:"am_i_gosha"`
	}

	var guests []GuestInfo
	if guestID != nil {
		var g GuestInfo
		db.DB.QueryRow(
			`SELECT id, last_name, first_name, middle_name, role, gender, custom_salutation, am_i_gosha FROM guests WHERE id=$1`, *guestID,
		).Scan(&g.ID, &g.LastName, &g.FirstName, &g.MiddleName, &g.Role, &g.Gender, &g.CustomSalutation, &g.AmIGosha)
		guests = append(guests, g)

		// Mark visited
		db.DB.Exec(`UPDATE guests SET visited=TRUE, visited_at=NOW() WHERE id=$1`, *guestID)
		db.DB.Exec(`INSERT INTO visit_logs (guest_id, link_token) VALUES ($1,$2)`, *guestID, token)
	} else if groupID != nil {
		rows, _ := db.DB.Query(
			`SELECT id, last_name, first_name, middle_name, role, gender, custom_salutation, am_i_gosha FROM guests WHERE group_id=$1`, *groupID,
		)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var g GuestInfo
				rows.Scan(&g.ID, &g.LastName, &g.FirstName, &g.MiddleName, &g.Role, &g.Gender, &g.CustomSalutation, &g.AmIGosha)
				guests = append(guests, g)
				db.DB.Exec(`UPDATE guests SET visited=TRUE, visited_at=NOW() WHERE id=$1`, g.ID)
			}
		}
		db.DB.Exec(`INSERT INTO visit_logs (link_token) VALUES ($1)`, token)
	}

	if len(guests) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "гости не найдены"})
		return
	}

	// Determine role (all guests in the link should have same role)
	role := guests[0].Role

	// Load page sections for this role
	sectionRows, _ := db.DB.Query(
		`SELECT id, title, content, section_order FROM page_sections WHERE role=$1 ORDER BY section_order`, role,
	)
	type Section struct {
		ID      int    `json:"id"`
		Title   string `json:"title"`
		Content string `json:"content"`
		Order   int    `json:"order"`
		Photos  []struct {
			ID       int    `json:"id"`
			Filename string `json:"filename"`
			Order    int    `json:"order"`
		} `json:"photos"`
	}
	sectionMap := map[int]*Section{}
	var sOrder []int
	if sectionRows != nil {
		defer sectionRows.Close()
		for sectionRows.Next() {
			var s Section
			sectionRows.Scan(&s.ID, &s.Title, &s.Content, &s.Order)
			s.Photos = []struct {
				ID       int    `json:"id"`
				Filename string `json:"filename"`
				Order    int    `json:"order"`
			}{}
			sectionMap[s.ID] = &s
			sOrder = append(sOrder, s.ID)
		}
	}
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
	var sections []Section
	for _, id := range sOrder {
		sections = append(sections, *sectionMap[id])
	}
	if sections == nil {
		sections = []Section{}
	}

	// Load existing responses — use primary guest ID regardless of link type
	// (group links have guestID=nil, but primaryGuest is always set)
	var friendResp *map[string]interface{}
	var familyResp *map[string]interface{}

	if len(guests) > 0 {
		primaryID := guests[0].ID

		if role == "friends" {
			var going bool
			var df, dt *string
			var tourn bool
			var oppID *int
			var comment string
			var adminReply string
			err := db.DB.QueryRow(
				`SELECT going_cottage, cottage_date_from::text, cottage_date_to::text, tournament, preferred_opponent_id, comment, admin_reply
				 FROM friend_responses WHERE guest_id=$1`, primaryID,
			).Scan(&going, &df, &dt, &tourn, &oppID, &comment, &adminReply)
			if err == nil {
				m := map[string]interface{}{
					"going_cottage":         going,
					"cottage_date_from":     df,
					"cottage_date_to":       dt,
					"tournament":            tourn,
					"preferred_opponent_id": oppID,
					"comment":               comment,
					"admin_reply":           adminReply,
				}
				friendResp = &m
			}
		}

		if role == "family" {
			var going, transport bool
			err := db.DB.QueryRow(
				`SELECT going_loft, needs_transport FROM family_responses WHERE guest_id=$1`, primaryID,
			).Scan(&going, &transport)
			if err == nil {
				m := map[string]interface{}{
					"going_loft":      going,
					"needs_transport": transport,
				}
				familyResp = &m
			}
		}
	}

	// Per-guest family responses — needed for group invitations so that each
	// member's "loft / transport" answers are stored and shown individually.
	famResponses := gin.H{}
	if role == "family" {
		for _, g := range guests {
			var gl, nt bool
			if err := db.DB.QueryRow(
				`SELECT going_loft, needs_transport FROM family_responses WHERE guest_id=$1`, g.ID,
			).Scan(&gl, &nt); err == nil {
				famResponses[strconv.Itoa(g.ID)] = gin.H{"going_loft": gl, "needs_transport": nt}
			}
		}
	}

	// Cottage date range from settings
	var cottageFrom, cottageTo string
	db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='cottage_date_from'`).Scan(&cottageFrom)
	db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='cottage_date_to'`).Scan(&cottageTo)

	// Page subtitle
	subtitleKey := "page_subtitle_friends"
	if role == "family" {
		subtitleKey = "page_subtitle_family"
	}
	var pageSubtitle string
	db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key=$1`, subtitleKey).Scan(&pageSubtitle)
	if pageSubtitle == "" {
		pageSubtitle = "Вы приглашены на нашу свадьбу 💍"
	}

	// Header / footer text
	var headerText, footerText string
	db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='header_text'`).Scan(&headerText)
	db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='footer_text'`).Scan(&footerText)
	if headerText == "" {
		headerText = "💍 Свадьба"
	}
	if footerText == "" {
		footerText = "С любовью ❤️"
	}

	// Attendance ("Приду / Не приду"). For groups it is shared, so reading the
	// primary guest's value reflects the whole group.
	var attending *bool
	if role == "friends" {
		db.DB.QueryRow(`SELECT attending FROM friend_responses WHERE guest_id=$1`, guests[0].ID).Scan(&attending)
	} else {
		db.DB.QueryRow(`SELECT attending FROM family_responses WHERE guest_id=$1`, guests[0].ID).Scan(&attending)
	}

	// Configurable "не приду" text and editable checkbox labels
	var declineText string
	db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key='decline_text'`).Scan(&declineText)

	labels := gin.H{}
	for _, k := range []string{
		"label_cottage_title", "label_cottage_desc",
		"label_tournament_title", "label_tournament_desc",
		"label_loft_title", "label_loft_desc",
		"label_transport_title", "label_transport_desc",
	} {
		var v string
		db.DB.QueryRow(`SELECT value FROM admin_settings WHERE key=$1`, k).Scan(&v)
		labels[k] = v
	}

	// Issue a guest JWT so invite page can make authenticated API calls
	// (gifts, pick/unpick, forms). Use first guest's ID and role.
	primaryGuest := guests[0]
	guestToken := makeToken(primaryGuest.ID, primaryGuest.FirstName, role)

	c.JSON(http.StatusOK, gin.H{
		"guests":            guests,
		"role":              role,
		"sections":          sections,
		"friend_response":   friendResp,
		"family_response":   familyResp,
		"family_responses":  famResponses,
		"attending":         attending,
		"decline_text":      declineText,
		"labels":            labels,
		"cottage_date_from": cottageFrom,
		"cottage_date_to":   cottageTo,
		"page_subtitle":     pageSubtitle,
		"header_text":       headerText,
		"footer_text":       footerText,
		"link_token":        token,
		"group_id":          groupID,
		"guest_token":       guestToken,
		"guest_id":          primaryGuest.ID,
	})
}

// SaveFriendResponse saves/updates friend's form response
func SaveFriendResponse(c *gin.Context) {
	guestIDVal, _ := c.Get("user_id")
	guestID := guestIDVal.(int)

	var req struct {
		GoingCottage        bool    `json:"going_cottage"`
		CottageDateFrom     *string `json:"cottage_date_from"`
		CottageDateTo       *string `json:"cottage_date_to"`
		Tournament          bool    `json:"tournament"`
		PreferredOpponentID *int    `json:"preferred_opponent_id"`
		Comment             string  `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.DB.Exec(`
		INSERT INTO friend_responses (guest_id, going_cottage, cottage_date_from, cottage_date_to, tournament, preferred_opponent_id, comment, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		ON CONFLICT (guest_id) DO UPDATE SET
			going_cottage=EXCLUDED.going_cottage,
			cottage_date_from=EXCLUDED.cottage_date_from,
			cottage_date_to=EXCLUDED.cottage_date_to,
			tournament=EXCLUDED.tournament,
			preferred_opponent_id=EXCLUDED.preferred_opponent_id,
			comment=EXCLUDED.comment,
			updated_at=NOW()`,
		guestID, req.GoingCottage, req.CottageDateFrom, req.CottageDateTo,
		req.Tournament, req.PreferredOpponentID, req.Comment,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SaveFamilyResponse saves/updates family form responses.
// Supports both a single answer (legacy) and a per-guest array `responses`
// for group invitations. Callers may only save answers for themselves or for
// members of their own group.
func SaveFamilyResponse(c *gin.Context) {
	guestIDVal, _ := c.Get("user_id")
	guestID := guestIDVal.(int)

	var req struct {
		// legacy single-guest form
		GoingLoft      *bool `json:"going_loft"`
		NeedsTransport *bool `json:"needs_transport"`
		// per-guest form (group invitations)
		Responses []struct {
			GuestID        int  `json:"guest_id"`
			GoingLoft      bool `json:"going_loft"`
			NeedsTransport bool `json:"needs_transport"`
		} `json:"responses"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build the set of guest IDs this caller is allowed to write: themselves
	// plus everyone in the same group.
	allowed := map[int]bool{guestID: true}
	var groupID *int
	db.DB.QueryRow(`SELECT group_id FROM guests WHERE id=$1`, guestID).Scan(&groupID)
	if groupID != nil {
		if rows, err := db.DB.Query(`SELECT id FROM guests WHERE group_id=$1`, *groupID); err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int
				rows.Scan(&id)
				allowed[id] = true
			}
		}
	}

	upsert := func(gid int, gl, nt bool) error {
		_, err := db.DB.Exec(`
			INSERT INTO family_responses (guest_id, going_loft, needs_transport, updated_at)
			VALUES ($1,$2,$3,NOW())
			ON CONFLICT (guest_id) DO UPDATE SET
				going_loft=EXCLUDED.going_loft,
				needs_transport=EXCLUDED.needs_transport,
				updated_at=NOW()`,
			gid, gl, nt,
		)
		return err
	}

	if len(req.Responses) > 0 {
		for _, r := range req.Responses {
			if !allowed[r.GuestID] {
				continue // silently skip guests outside the caller's group
			}
			if err := upsert(r.GuestID, r.GoingLoft, r.NeedsTransport); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	} else if req.GoingLoft != nil {
		nt := req.NeedsTransport != nil && *req.NeedsTransport
		if err := upsert(guestID, *req.GoingLoft, nt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SaveAttendance stores the "Приду / Не приду" choice. For friends it is
// personal; for family it is shared across the whole group.
func SaveAttendance(c *gin.Context) {
	guestIDVal, _ := c.Get("user_id")
	guestID := guestIDVal.(int)

	var req struct {
		Attending bool `json:"attending"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var role string
	db.DB.QueryRow(`SELECT role FROM guests WHERE id=$1`, guestID).Scan(&role)

	if role == "friends" {
		_, err := db.DB.Exec(`
			INSERT INTO friend_responses (guest_id, attending, updated_at)
			VALUES ($1,$2,NOW())
			ON CONFLICT (guest_id) DO UPDATE SET attending=EXCLUDED.attending, updated_at=NOW()`,
			guestID, req.Attending,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Family: apply to the whole group (shared decision), or just self.
		targets := []int{guestID}
		var groupID *int
		db.DB.QueryRow(`SELECT group_id FROM guests WHERE id=$1`, guestID).Scan(&groupID)
		if groupID != nil {
			if rows, err := db.DB.Query(`SELECT id FROM guests WHERE group_id=$1`, *groupID); err == nil {
				defer rows.Close()
				targets = nil
				for rows.Next() {
					var id int
					rows.Scan(&id)
					targets = append(targets, id)
				}
			}
		}
		for _, id := range targets {
			if _, err := db.DB.Exec(`
				INSERT INTO family_responses (guest_id, attending, updated_at)
				VALUES ($1,$2,NOW())
				ON CONFLICT (guest_id) DO UPDATE SET attending=EXCLUDED.attending, updated_at=NOW()`,
				id, req.Attending,
			); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetGifts returns gifts for the current user's role
func GetGifts(c *gin.Context) {
	guestIDVal, _ := c.Get("user_id")
	guestID := guestIDVal.(int)

	var role string
	db.DB.QueryRow(`SELECT role FROM guests WHERE id=$1`, guestID).Scan(&role)

	rows, err := db.DB.Query(`
		SELECT g.id, g.name, g.description, g.is_pickable, g.selected_by_guest_id,
		       COALESCE(gu.last_name || ' ' || gu.first_name, '') as selected_by_name,
		       g.photo_filename, g.link_url
		FROM gifts g
		LEFT JOIN guests gu ON gu.id = g.selected_by_guest_id
		WHERE g.role=$1
		ORDER BY g.id`, role,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type GiftRow struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Description       string `json:"description"`
		IsPickable        bool   `json:"is_pickable"`
		SelectedByGuestID *int   `json:"selected_by_guest_id"`
		SelectedByName    string `json:"selected_by_name"`
		IsMyPick          bool   `json:"is_my_pick"`
		PhotoFilename     string `json:"photo_filename"`
		LinkURL           string `json:"link_url"`
	}
	var gifts []GiftRow
	for rows.Next() {
		var g GiftRow
		rows.Scan(&g.ID, &g.Name, &g.Description, &g.IsPickable, &g.SelectedByGuestID, &g.SelectedByName, &g.PhotoFilename, &g.LinkURL)
		if g.SelectedByGuestID != nil && *g.SelectedByGuestID == guestID {
			g.IsMyPick = true
		}
		gifts = append(gifts, g)
	}
	if gifts == nil {
		gifts = []GiftRow{}
	}
	c.JSON(http.StatusOK, gifts)
}

// PickGift lets a guest select a gift
func PickGift(c *gin.Context) {
	giftID, _ := strconv.Atoi(c.Param("id"))
	guestIDVal, _ := c.Get("user_id")
	guestID := guestIDVal.(int)

	// Check gift exists and is pickable
	var isPickable bool
	var selectedBy *int
	err := db.DB.QueryRow(`SELECT is_pickable, selected_by_guest_id FROM gifts WHERE id=$1`, giftID).Scan(&isPickable, &selectedBy)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "gift not found"})
		return
	}
	if !isPickable {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gift is not pickable"})
		return
	}
	if selectedBy != nil && *selectedBy != guestID {
		c.JSON(http.StatusConflict, gin.H{"error": "gift already taken"})
		return
	}

	// One pick per guest: release any previously picked gift first
	db.DB.Exec(`UPDATE gifts SET selected_by_guest_id=NULL WHERE selected_by_guest_id=$1`, guestID)

	db.DB.Exec(`UPDATE gifts SET selected_by_guest_id=$1 WHERE id=$2`, guestID, giftID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// UnpickGift lets a guest cancel their gift selection
func UnpickGift(c *gin.Context) {
	giftID, _ := strconv.Atoi(c.Param("id"))
	guestIDVal, _ := c.Get("user_id")
	guestID := guestIDVal.(int)

	db.DB.Exec(`UPDATE gifts SET selected_by_guest_id=NULL WHERE id=$1 AND selected_by_guest_id=$2`, giftID, guestID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetMusic returns music files for the current user's role
func GetMusic(c *gin.Context) {
	guestIDVal, _ := c.Get("user_id")
	guestID := guestIDVal.(int)

	var role string
	db.DB.QueryRow(`SELECT role FROM guests WHERE id=$1`, guestID).Scan(&role)

	rows, _ := db.DB.Query(
		`SELECT id, filename, original_name, title, artist FROM music_files WHERE role=$1 ORDER BY file_order, id`, role,
	)
	type MusicRow struct {
		ID           int    `json:"id"`
		Filename     string `json:"filename"`
		OriginalName string `json:"original_name"`
		Title        string `json:"title"`
		Artist       string `json:"artist"`
	}
	var music []MusicRow
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var m MusicRow
			rows.Scan(&m.ID, &m.Filename, &m.OriginalName, &m.Title, &m.Artist)
			music = append(music, m)
		}
	}
	if music == nil {
		music = []MusicRow{}
	}
	c.JSON(http.StatusOK, music)
}

// GetFriends returns list of all friends (for tournament opponent selection)
func GetFriends(c *gin.Context) {
	guestIDVal, _ := c.Get("user_id")
	guestID := guestIDVal.(int)

	rows, _ := db.DB.Query(
		`SELECT id, last_name, first_name, middle_name FROM guests WHERE role='friends' AND id != $1 ORDER BY last_name, first_name`, guestID,
	)
	type FriendRow struct {
		ID         int    `json:"id"`
		LastName   string `json:"last_name"`
		FirstName  string `json:"first_name"`
		MiddleName string `json:"middle_name"`
	}
	var friends []FriendRow
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var f FriendRow
			rows.Scan(&f.ID, &f.LastName, &f.FirstName, &f.MiddleName)
			friends = append(friends, f)
		}
	}
	if friends == nil {
		friends = []FriendRow{}
	}
	c.JSON(http.StatusOK, friends)
}

// GetGuestInfo returns info about the current logged-in guest
func GetGuestInfo(c *gin.Context) {
	guestIDVal, _ := c.Get("user_id")
	guestID := guestIDVal.(int)

	type GuestFull struct {
		ID               int    `json:"id"`
		LastName         string `json:"last_name"`
		FirstName        string `json:"first_name"`
		MiddleName       string `json:"middle_name"`
		Role             string `json:"role"`
		Gender           string `json:"gender"`
		CustomSalutation string `json:"custom_salutation"`
		AmIGosha         bool   `json:"am_i_gosha"`
		GroupID          *int   `json:"group_id"`
	}
	var g GuestFull
	err := db.DB.QueryRow(
		`SELECT id, last_name, first_name, middle_name, role, gender, custom_salutation, am_i_gosha, group_id FROM guests WHERE id=$1`, guestID,
	).Scan(&g.ID, &g.LastName, &g.FirstName, &g.MiddleName, &g.Role, &g.Gender, &g.CustomSalutation, &g.AmIGosha, &g.GroupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, g)
}
