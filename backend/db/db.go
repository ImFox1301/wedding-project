package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}

	Migrate()
	log.Println("Database connected and migrated")
}

func Migrate() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS guest_groups (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS guests (
			id SERIAL PRIMARY KEY,
			last_name VARCHAR(100) NOT NULL,
			first_name VARCHAR(100) NOT NULL,
			middle_name VARCHAR(100) DEFAULT '',
			login VARCHAR(100) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(50) NOT NULL DEFAULT 'friends',
			am_i_gosha BOOLEAN DEFAULT FALSE,
			group_id INTEGER REFERENCES guest_groups(id) ON DELETE SET NULL,
			visited BOOLEAN DEFAULT FALSE,
			visited_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS invitation_links (
			id SERIAL PRIMARY KEY,
			token VARCHAR(255) UNIQUE NOT NULL,
			guest_id INTEGER REFERENCES guests(id) ON DELETE CASCADE,
			group_id INTEGER REFERENCES guest_groups(id) ON DELETE CASCADE,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS visit_logs (
			id SERIAL PRIMARY KEY,
			guest_id INTEGER REFERENCES guests(id) ON DELETE SET NULL,
			link_token VARCHAR(255),
			visited_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS page_sections (
			id SERIAL PRIMARY KEY,
			role VARCHAR(50) NOT NULL,
			title VARCHAR(255) DEFAULT '',
			content TEXT DEFAULT '',
			section_order INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS section_photos (
			id SERIAL PRIMARY KEY,
			section_id INTEGER NOT NULL REFERENCES page_sections(id) ON DELETE CASCADE,
			filename VARCHAR(255) NOT NULL,
			photo_order INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS gifts (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT DEFAULT '',
			role VARCHAR(50) NOT NULL,
			is_pickable BOOLEAN DEFAULT TRUE,
			selected_by_guest_id INTEGER REFERENCES guests(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS friend_responses (
			id SERIAL PRIMARY KEY,
			guest_id INTEGER UNIQUE NOT NULL REFERENCES guests(id) ON DELETE CASCADE,
			going_cottage BOOLEAN DEFAULT FALSE,
			cottage_date_from DATE,
			cottage_date_to DATE,
			tournament BOOLEAN DEFAULT FALSE,
			preferred_opponent_id INTEGER REFERENCES guests(id) ON DELETE SET NULL,
			comment TEXT DEFAULT '',
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS family_responses (
			id SERIAL PRIMARY KEY,
			guest_id INTEGER UNIQUE NOT NULL REFERENCES guests(id) ON DELETE CASCADE,
			going_loft BOOLEAN DEFAULT FALSE,
			needs_transport BOOLEAN DEFAULT FALSE,
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS admin_settings (
			key VARCHAR(100) PRIMARY KEY,
			value TEXT DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS music_files (
			id SERIAL PRIMARY KEY,
			role VARCHAR(50) NOT NULL,
			filename VARCHAR(255) NOT NULL,
			original_name VARCHAR(255) DEFAULT '',
			file_order INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		// New columns (idempotent)
		`ALTER TABLE guests ADD COLUMN IF NOT EXISTS gender VARCHAR(10) NOT NULL DEFAULT 'male'`,
		`ALTER TABLE guests ADD COLUMN IF NOT EXISTS custom_salutation VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE gifts ADD COLUMN IF NOT EXISTS photo_filename VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE gifts ADD COLUMN IF NOT EXISTS link_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE music_files ADD COLUMN IF NOT EXISTS title  VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE music_files ADD COLUMN IF NOT EXISTS artist VARCHAR(255) NOT NULL DEFAULT ''`,
		// Admin reply to a guest's comment
		`ALTER TABLE friend_responses ADD COLUMN IF NOT EXISTS admin_reply TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE friend_responses ADD COLUMN IF NOT EXISTS admin_reply_at TIMESTAMP`,
		// Default settings
		`INSERT INTO admin_settings (key, value) VALUES
			('cottage_date_from',      ''),
			('cottage_date_to',        ''),
			('page_subtitle_friends',  'Вы приглашены на нашу свадьбу 💍'),
			('page_subtitle_family',   'Вы приглашены на нашу свадьбу 💍'),
			('header_text',            '💍 Свадьба'),
			('footer_text',            'С любовью ❤️')
		ON CONFLICT (key) DO NOTHING`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			log.Fatalf("Migration failed: %v\nQuery: %s", err, q)
		}
	}
}
