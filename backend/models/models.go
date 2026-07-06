package models

import "time"

type Role string

const (
	RoleFriends Role = "friends"
	RoleFamily  Role = "family"
	RoleAdmin   Role = "admin"
)

type Guest struct {
	ID           int       `json:"id" db:"id"`
	LastName     string    `json:"last_name" db:"last_name"`
	FirstName    string    `json:"first_name" db:"first_name"`
	MiddleName   string    `json:"middle_name" db:"middle_name"`
	Login        string    `json:"login" db:"login"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         Role      `json:"role" db:"role"`
	AmIGosha     bool      `json:"am_i_gosha" db:"am_i_gosha"`
	GroupID      *int      `json:"group_id" db:"group_id"`
	Visited      bool      `json:"visited" db:"visited"`
	VisitedAt    *time.Time `json:"visited_at" db:"visited_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type GuestGroup struct {
	ID        int       `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type InvitationLink struct {
	ID        int       `json:"id" db:"id"`
	Token     string    `json:"token" db:"token"`
	GuestID   *int      `json:"guest_id" db:"guest_id"`
	GroupID   *int      `json:"group_id" db:"group_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type PageSection struct {
	ID        int       `json:"id" db:"id"`
	Role      Role      `json:"role" db:"role"`
	Title     string    `json:"title" db:"title"`
	Content   string    `json:"content" db:"content"`
	Order     int       `json:"order" db:"section_order"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	Photos    []SectionPhoto `json:"photos"`
}

type SectionPhoto struct {
	ID        int    `json:"id" db:"id"`
	SectionID int    `json:"section_id" db:"section_id"`
	Filename  string `json:"filename" db:"filename"`
	Order     int    `json:"order" db:"photo_order"`
}

type Gift struct {
	ID               int    `json:"id" db:"id"`
	Name             string `json:"name" db:"name"`
	Description      string `json:"description" db:"description"`
	Role             Role   `json:"role" db:"role"`
	IsPickable       bool   `json:"is_pickable" db:"is_pickable"`
	SelectedByGuestID *int  `json:"selected_by_guest_id" db:"selected_by_guest_id"`
	SelectedByName   string `json:"selected_by_name,omitempty"`
}

type FriendResponse struct {
	ID                  int       `json:"id" db:"id"`
	GuestID             int       `json:"guest_id" db:"guest_id"`
	GoingCottage        bool      `json:"going_cottage" db:"going_cottage"`
	CottageDateFrom     *string   `json:"cottage_date_from" db:"cottage_date_from"`
	CottageDateTo       *string   `json:"cottage_date_to" db:"cottage_date_to"`
	Tournament          bool      `json:"tournament" db:"tournament"`
	PreferredOpponentID *int      `json:"preferred_opponent_id" db:"preferred_opponent_id"`
	Comment             string    `json:"comment" db:"comment"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

type FamilyResponse struct {
	ID             int       `json:"id" db:"id"`
	GuestID        int       `json:"guest_id" db:"guest_id"`
	GoingLoft      bool      `json:"going_loft" db:"going_loft"`
	NeedsTransport bool      `json:"needs_transport" db:"needs_transport"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type AdminSetting struct {
	Key   string `json:"key" db:"key"`
	Value string `json:"value" db:"value"`
}

type MusicFile struct {
	ID           int    `json:"id" db:"id"`
	Role         Role   `json:"role" db:"role"`
	Filename     string `json:"filename" db:"filename"`
	OriginalName string `json:"original_name" db:"original_name"`
	Order        int    `json:"order" db:"file_order"`
}

type VisitLog struct {
	ID        int       `json:"id" db:"id"`
	GuestID   *int      `json:"guest_id" db:"guest_id"`
	LinkToken string    `json:"link_token" db:"link_token"`
	VisitedAt time.Time `json:"visited_at" db:"visited_at"`
}
