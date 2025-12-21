package database

import (
	"database/sql"
	"fmt"
)

// Data structs related to DB
type List struct {
	ID          int64  `json:"id"`
	GroupName   string `json:"group_name"`
	Name        string `json:"list_name"`
	ContentType string `json:"content_type"`
	KodiHost    string `json:"kodi_host"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

type Config struct {
	Lists    []List `json:"lists"`
	Subtitle string `json:"subtitle"`
	Footer   string `json:"footer"`
}

type Item struct {
	ID           int64   `json:"id"`
	ListID       int64   `json:"list_id"`
	KodiID       int     `json:"kodi_id"`
	MediaType    string  `json:"media_type"` // movie, episode, show, season
	Title        string  `json:"title"`
	Year         int     `json:"year"`
	Poster       string  `json:"poster_path"`
	Runtime      int     `json:"runtime"`
	EpisodeCount int     `json:"episode_count"`
	Season       int     `json:"season"`
	Rating       float64 `json:"rating"`
	SortOrder    int     `json:"sort_order"`
	AddedAt      string  `json:"added_at"`
}

type CachedItem struct {
	ListID       int64   `json:"list_id"`
	KodiID       int     `json:"kodi_id"`
	MediaType    string  `json:"media_type"`
	Title        string  `json:"title"`
	Year         int     `json:"year"`
	Poster       string  `json:"poster_path"`
	Runtime      int     `json:"runtime"`
	EpisodeCount int     `json:"episode_count"`
	Rating       float64 `json:"rating"`
	Plot         string  `json:"plot"`
}

func (db *DB) GetAllLists() ([]List, error) {
	rows, err := db.Query("SELECT id, group_name, name, content_type, kodi_host, username, password FROM lists ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []List
	for rows.Next() {
		var l List
		var contentType sql.NullString
		if err := rows.Scan(&l.ID, &l.GroupName, &l.Name, &contentType, &l.KodiHost, &l.Username, &l.Password); err != nil {
			return nil, err
		}
		l.ContentType = contentType.String
		lists = append(lists, l)
	}
	return lists, nil
}

func (db *DB) SyncLists(lists []List) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmtFind, _ := tx.Prepare("SELECT id FROM lists WHERE group_name = ? AND name = ?")
	stmtUpdate, _ := tx.Prepare("UPDATE lists SET kodi_host=?, username=?, password=?, content_type=? WHERE id=?")
	stmtInsert, _ := tx.Prepare("INSERT INTO lists (group_name, name, content_type, kodi_host, username, password) VALUES (?, ?, ?, ?, ?, ?)")

	for _, l := range lists {
		// Default content_type if missing in config
		if l.ContentType == "" {
			if l.Name == "tv" {
				l.ContentType = "tv"
			} else {
				l.ContentType = "movie"
			}
		}

		var id int64
		err := stmtFind.QueryRow(l.GroupName, l.Name).Scan(&id)
		if err == nil {
			if _, err := stmtUpdate.Exec(l.KodiHost, l.Username, l.Password, l.ContentType, id); err != nil {
				return err
			}
		} else {
			if _, err := stmtInsert.Exec(l.GroupName, l.Name, l.ContentType, l.KodiHost, l.Username, l.Password); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (db *DB) GetItems(listID int64) ([]Item, error) {
	rows, err := db.Query(`
		SELECT id, list_id, kodi_id, media_type, title, year, poster_path, runtime, episode_count, season, rating, sort_order, added_at 
		FROM items 
		WHERE list_id = ? 
		ORDER BY sort_order ASC, added_at DESC`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Item, 0)
	for rows.Next() {
		var i Item
		if err := rows.Scan(&i.ID, &i.ListID, &i.KodiID, &i.MediaType, &i.Title, &i.Year, &i.Poster, &i.Runtime, &i.EpisodeCount, &i.Season, &i.Rating, &i.SortOrder, &i.AddedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

func (db *DB) AddItem(i Item) (int64, error) {
	// Handle automatic positioning:
	// -1 = add to top (shift all items down)
	// 0 = add to bottom (use max + 1)
	// >0 = explicit position (use as-is)
	if i.SortOrder == -1 {
		// Add to top: perform shift and insert in a single transaction to avoid races
		tx, err := db.Begin()
		if err != nil {
			return 0, fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Shift all existing items down within the transaction
		if _, err := tx.Exec("UPDATE items SET sort_order = sort_order + 1 WHERE list_id = ?", i.ListID); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("failed to shift items: %w", err)
		}
		i.SortOrder = 0

		// Insert the new item at the top within the same transaction
		res, err := tx.Exec(`
		INSERT OR IGNORE INTO items (list_id, kodi_id, media_type, title, year, poster_path, runtime, episode_count, season, rating, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			i.ListID, i.KodiID, i.MediaType, i.Title, i.Year, i.Poster, i.Runtime, i.EpisodeCount, i.Season, i.Rating, i.SortOrder)
		if err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("failed to insert item: %w", err)
		}

		lastID, err := res.LastInsertId()
		if err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("failed to get last insert id: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("failed to commit transaction: %w", err)
		}

		return lastID, nil
	} else if i.SortOrder == 0 {
		// Add to bottom: use max + 1
		maxOrder, err := db.GetMaxSortOrder(i.ListID)
		if err != nil {
			return 0, fmt.Errorf("failed to get max sort order: %w", err)
		}
		i.SortOrder = maxOrder + 1
	}
	// else: explicit position, use as-is

	res, err := db.Exec(`
		INSERT OR IGNORE INTO items (list_id, kodi_id, media_type, title, year, poster_path, runtime, episode_count, season, rating, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ListID, i.KodiID, i.MediaType, i.Title, i.Year, i.Poster, i.Runtime, i.EpisodeCount, i.Season, i.Rating, i.SortOrder)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetMaxSortOrder(listID int64) (int, error) {
	var maxOrder int
	err := db.QueryRow("SELECT COALESCE(MAX(sort_order), -1) FROM items WHERE list_id = ?", listID).Scan(&maxOrder)
	return maxOrder, err
}

func (db *DB) DeleteItem(id int64) error {
	_, err := db.Exec("DELETE FROM items WHERE id = ?", id)
	return err
}

func (db *DB) UpdateItemOrder(id int64, sortOrder int) error {
	_, err := db.Exec("UPDATE items SET sort_order = ? WHERE id = ?", sortOrder, id)
	return err
}

// Library Cache Operations

func (db *DB) ClearLibraryCache(listID int64, mediaType string) error {
	_, err := db.Exec("DELETE FROM library_cache WHERE list_id = ? AND media_type = ?", listID, mediaType)
	return err
}

func (db *DB) AddToLibraryCache(items []CachedItem) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO library_cache (list_id, kodi_id, media_type, title, year, poster_path, runtime, episode_count, rating, plot)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, i := range items {
		_, err := stmt.Exec(i.ListID, i.KodiID, i.MediaType, i.Title, i.Year, i.Poster, i.Runtime, i.EpisodeCount, i.Rating, i.Plot)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) SearchLibraryCache(listID int64, mediaType string, query string) ([]CachedItem, error) {
	searchQuery := fmt.Sprintf("%%%s%%", query)
	// Search across all lists that share the same Kodi host to leverage shared cache
	rows, err := db.Query(`
		SELECT MAX(lc.list_id), lc.kodi_id, lc.media_type, lc.title, lc.year, lc.poster_path, lc.runtime, lc.episode_count, lc.rating, lc.plot
		FROM library_cache lc
		JOIN lists l_cache ON lc.list_id = l_cache.id
		JOIN lists l_current ON l_current.id = ?
		WHERE l_cache.kodi_host = l_current.kodi_host 
		AND lc.media_type = ? 
		AND lc.title LIKE ?
		GROUP BY lc.kodi_id
		LIMIT 50`, listID, mediaType, searchQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CachedItem
	for rows.Next() {
		var i CachedItem
		if err := rows.Scan(&i.ListID, &i.KodiID, &i.MediaType, &i.Title, &i.Year, &i.Poster, &i.Runtime, &i.EpisodeCount, &i.Rating, &i.Plot); err != nil {
			return nil, err
		}
		results = append(results, i)
	}
	return results, nil
}

func (db *DB) GetLibraryCacheCount(listID int64, mediaType string) (int, error) {
	var count int
	// Count items across all lists that share the same Kodi host
	err := db.QueryRow(`
		SELECT COUNT(DISTINCT lc.kodi_id) 
		FROM library_cache lc
		JOIN lists l_cache ON lc.list_id = l_cache.id
		JOIN lists l_current ON l_current.id = ?
		WHERE l_cache.kodi_host = l_current.kodi_host 
		AND lc.media_type = ?`, listID, mediaType).Scan(&count)
	return count, err
}
