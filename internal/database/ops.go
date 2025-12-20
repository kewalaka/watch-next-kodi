package database

import "fmt"

// Data structs related to DB
type List struct {
	ID        int64  `json:"id"`
	GroupName string `json:"group_name"`
	Type      string `json:"type"`
	KodiHost  string `json:"kodi_host"`
	Username  string `json:"username"`
	Password  string `json:"password"`
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
	rows, err := db.Query("SELECT id, group_name, type, kodi_host, username, password FROM lists ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []List
	for rows.Next() {
		var l List
		if err := rows.Scan(&l.ID, &l.GroupName, &l.Type, &l.KodiHost, &l.Username, &l.Password); err != nil {
			return nil, err
		}
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

	// Using INSERT OR REPLACE to update existing entries or add new ones
	// Note: In a real production system with foreign keys, you might want to be more careful
	// about IDs, but for this simple setup, matching by Group+Type covers us.

	// First, let's verify if we need to clear old ones or just upsert.
	// For simplicity and safety (preserving IDs if possible), we'll do an upsert logic.
	// SQLite 'INSERT OR REPLACE' replaces the whole row which changes ID if it's the primary key.
	// Instead, let's look up by properties or just accept that list IDs are stable if config order is stable.
	// Actually, the safest for "Declarative Config" is:
	// 1. Check if exists (by group & type), Update fields.
	// 2. If not, Insert.

	// CORRECT APPROACH: Select all, map them, update/insert.
	// But to keep it simple and robust:
	// Let's assume the user doesn't change lists often. We will just Look up by Group+Type.

	stmtFind, _ := tx.Prepare("SELECT id FROM lists WHERE group_name = ? AND type = ?")
	stmtUpdate, _ := tx.Prepare("UPDATE lists SET kodi_host=?, username=?, password=? WHERE id=?")
	stmtInsert, _ := tx.Prepare("INSERT INTO lists (group_name, type, kodi_host, username, password) VALUES (?, ?, ?, ?, ?)")

	for _, l := range lists {
		var id int64
		err := stmtFind.QueryRow(l.GroupName, l.Type).Scan(&id)
		if err == nil {
			// Found, Update
			if _, err := stmtUpdate.Exec(l.KodiHost, l.Username, l.Password, id); err != nil {
				return err
			}
		} else {
			// Not found, Insert
			if _, err := stmtInsert.Exec(l.GroupName, l.Type, l.KodiHost, l.Username, l.Password); err != nil {
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
	res, err := db.Exec(`
		INSERT OR IGNORE INTO items (list_id, kodi_id, media_type, title, year, poster_path, runtime, episode_count, season, rating, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ListID, i.KodiID, i.MediaType, i.Title, i.Year, i.Poster, i.Runtime, i.EpisodeCount, i.Season, i.Rating, i.SortOrder)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
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
	rows, err := db.Query(`
		SELECT list_id, kodi_id, media_type, title, year, poster_path, runtime, episode_count, rating, plot
		FROM library_cache
		WHERE list_id = ? AND media_type = ? AND title LIKE ?
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
	err := db.QueryRow("SELECT COUNT(*) FROM library_cache WHERE list_id = ? AND media_type = ?", listID, mediaType).Scan(&count)
	return count, err
}
