package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func InitDB(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS lists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_name TEXT NOT NULL,
			type TEXT NOT NULL,
			kodi_host TEXT NOT NULL,
			username TEXT DEFAULT '',
			password TEXT DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			list_id INTEGER NOT NULL,
			kodi_id INTEGER,
			media_type TEXT, 
			title TEXT,
			year INTEGER,
			poster_path TEXT,
			runtime INTEGER, 
			episode_count INTEGER,
			season INTEGER,
			rating REAL,
			sort_order INTEGER DEFAULT 0,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(list_id) REFERENCES lists(id),
			UNIQUE(list_id, kodi_id, media_type, season)
		);`,
		`CREATE TABLE IF NOT EXISTS library_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			list_id INTEGER NOT NULL,
			kodi_id INTEGER NOT NULL,
			media_type TEXT NOT NULL, -- movie, show
			title TEXT NOT NULL,
			year INTEGER,
			poster_path TEXT,
			runtime INTEGER,
			episode_count INTEGER,
			rating REAL,
			plot TEXT,
			FOREIGN KEY(list_id) REFERENCES lists(id),
			UNIQUE(list_id, kodi_id, media_type)
		);`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			log.Printf("Error executing query: %s\nError: %v", query, err)
			return err
		}
	}
	return nil
}
