package database

import (
	"database/sql"
	"fmt"
	"log/slog"

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

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func migrate(db *sql.DB) error {
	// 1. Ensure schema_version table exists
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER DEFAULT 0)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// 2. Get current version
	var version int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	// 3. Handle legacy databases (v1.0.0 or v1.1.0 without version table)
	if version == 0 {
		var name string
		// Check if 'lists' table exists
		err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='lists'").Scan(&name)
		if err == nil {
			// Table exists. Check schema state.
			hasType := false
			if rows, err := db.Query("SELECT type FROM lists LIMIT 1"); err == nil {
				rows.Close()
				hasType = true
			}

			hasName := false
			if rows, err := db.Query("SELECT name FROM lists LIMIT 1"); err == nil {
				rows.Close()
				hasName = true
			}

			if hasName {
				// Prefer newer schema when both 'type' and 'name' exist (unlikely but possible)
				version = 2
			} else if hasType {
				version = 1
			}
			// Update the version table to match reality
			_, _ = db.Exec("INSERT INTO schema_version (version) VALUES (?)", version)
		}
	}

	slog.Info("Current database schema version", "version", version)

	// 4. Define migrations
	migrations := []func(*sql.Tx) error{
		// Migration 1: Initial Schema (v1.0.0)
		func(tx *sql.Tx) error {
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
			for _, q := range queries {
				if _, err := tx.Exec(q); err != nil {
					return err
				}
			}
			return nil
		},
		// Migration 2: Update to v1.1.0 (Rename type->name, add content_type)
		func(tx *sql.Tx) error {
			// We use a separate check here because SQLite ALTER TABLE is limited
			// Backfill content_type for existing 'tv' lists
			// Note: The DEFAULT 'movie' handles everything else.
			if _, err := tx.Exec("UPDATE lists SET content_type = 'tv' WHERE name = 'tv'"); err != nil {
				return fmt.Errorf("failed to backfill content_type: %w", err)
			}
			// But since we are in a transaction and version controlled, we can just run it.
			if _, err := tx.Exec("ALTER TABLE lists RENAME COLUMN type TO name"); err != nil {
				return fmt.Errorf("failed to rename column: %w", err)
			}
			if _, err := tx.Exec("ALTER TABLE lists ADD COLUMN content_type TEXT DEFAULT 'movie'"); err != nil {
				return fmt.Errorf("failed to add column: %w", err)
			}
			return nil
		},
	}

	// 5. Apply migrations
	for i := version; i < len(migrations); i++ {
		slog.Info("Applying migration", "version", i+1)
		tx, err := db.Begin()
		if err != nil {
			return err
		}

		if err := migrations[i](tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}

		// Update version
		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", i+1); err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}
