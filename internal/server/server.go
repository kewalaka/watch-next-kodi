package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"whats-next/internal/database"
	"whats-next/internal/kodi"
)

type Server struct {
	db         *database.DB
	config     database.Config
	httpClient *http.Client
}

func NewServer(db *database.DB, config database.Config) *Server {
	return &Server{
		db:     db,
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/lists", s.handleLists)
	mux.HandleFunc("/lists/", s.handleListRoutes)
	mux.HandleFunc("/items/", s.handleItemRoutes)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/sync", s.handleSyncLibrary)
	mux.HandleFunc("/tv/seasons", s.handleGetSeasons)
	mux.HandleFunc("/tv/episodes", s.handleGetEpisodes)

	// Serve posters from local storage
	// Ensure directory exists
	os.MkdirAll("data/posters", 0755)
	mux.Handle("/posters/", http.StripPrefix("/posters/", http.FileServer(http.Dir("data/posters"))))

	mux.HandleFunc("/config", s.handleGetConfig)

	return mux
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"subtitle": s.config.Subtitle,
		"footer":   s.config.Footer,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) getKodiClient(listID int64) (*kodi.Client, error) {
	lists, err := s.db.GetAllLists()
	if err != nil {
		return nil, fmt.Errorf("failed to get lists from database: %w", err)
	}
	var list *database.List
	for _, l := range lists {
		if l.ID == listID {
			list = &l
			break
		}
	}
	if list == nil {
		return nil, fmt.Errorf("list not found: %d", listID)
	}
	host := list.KodiHost
	user := list.Username
	pass := list.Password
	if os.Getenv("MOCK_KODI") == "true" {
		host = "mock"
	}
	return kodi.NewClient(host, user, pass), nil
}

func slugify(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, strings.ToLower(s))
}

// flightMap stores per-file mutexes used to synchronize concurrent downloads.
// To avoid unbounded growth, we periodically clear entries that are no longer needed.
var flightMap sync.Map // Map of fileName -> *sync.Mutex

func init() {
	// Periodically clean up the flightMap to prevent unbounded memory growth.
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		for range ticker.C {
			flightMap.Range(func(key, _ any) bool {
				flightMap.Delete(key)
				return true
			})
		}
	}()
}
func (s *Server) downloadBestImage(client *kodi.Client, item kodi.MediaItem, mediaType string) (string, error) {
	var imageURI string
	if item.Art != nil {
		if val, ok := item.Art["poster"]; ok && val != "" {
			imageURI = val
		} else if val, ok := item.Art["thumb"]; ok && val != "" {
			imageURI = val
		}
	}

	if imageURI == "" {
		imageURI = item.Thumbnail
	}

	if imageURI == "" {
		return "", nil
	}
	if client.HostURL == "mock" {
		return imageURI, nil
	}

	fileName := fmt.Sprintf("%s_%s_%d.jpg", mediaType, slugify(item.Title), item.Year)
	localPath := filepath.Join("data/posters", fileName)
	publicURL := "/api/posters/" + fileName

	// Fast path: check if file already exists
	if _, err := os.Stat(localPath); err == nil {
		return publicURL, nil
	}

	// Double-checked locking using a per-file mutex
	muAny, _ := flightMap.LoadOrStore(fileName, &sync.Mutex{})
	mu := muAny.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Check again after acquiring lock
	if _, err := os.Stat(localPath); err == nil {
		return publicURL, nil
	}

	// Kodi serves images at [HOST]/image/[ENCODED_URI]
	encodedURI := url.QueryEscape(imageURI)
	targetURL := client.HostURL + "/image/" + encodedURI
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "http://" + targetURL
	}

	slog.Info("Downloading best image", "media_type", mediaType, "kodi_id", item.ID, "title", item.Title, "local_path", localPath)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		slog.Error("Invalid request for image", "url", targetURL, "error", err)
		return "", err
	}
	if client.Username != "" {
		req.SetBasicAuth(client.Username, client.Password)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		slog.Error("Network error downloading image", "media_type", mediaType, "kodi_id", item.ID, "error", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			slog.Warn("Image not found on Kodi (404)", "title", item.Title, "url", targetURL)
			return "", nil // Return empty, not error, to keep sync going
		}
		slog.Error("Kodi returned error status for image", "status_code", resp.StatusCode, "url", targetURL)
		return "", fmt.Errorf("kodi image error: %d", resp.StatusCode)
	}

	out, err := os.Create(localPath)
	if err != nil {
		slog.Error("File creation error", "path", localPath, "error", err)
		return "", err
	}
	defer out.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		slog.Error("Copy error", "path", localPath, "error", err)
		return "", err
	}

	slog.Info("Successfully saved image", "path", localPath, "bytes", n)
	return publicURL, nil
}

func (s *Server) handleLists(w http.ResponseWriter, r *http.Request) {
	lists, err := s.db.GetAllLists()
	if err != nil {
		slog.Error("Failed to get lists from database", "error", err)
		http.Error(w, "Failed to retrieve lists", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lists)
}

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/lists/"), "/")
	if len(pathParts) < 2 || pathParts[1] != "items" {
		http.NotFound(w, r)
		return
	}

	listID, err := strconv.ParseInt(pathParts[0], 10, 64)
	if err != nil {
		slog.Warn("Invalid list ID in request", "path", pathParts[0], "error", err)
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		items, err := s.db.GetItems(listID)
		if err != nil {
			slog.Error("Failed to get items from database", "list_id", listID, "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
		return
	}

	if r.Method == http.MethodPost {
		var item database.Item
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			slog.Warn("Invalid request body", "error", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		item.ListID = listID

		// Ensure we have a local poster if it's a remote URL
		if strings.HasPrefix(item.Poster, "image://") || strings.HasPrefix(item.Poster, "http") {
			client, err := s.getKodiClient(listID)
			if err != nil {
				slog.Error("Failed to get Kodi client", "list_id", listID, "error", err)
			} else {
				// Convert database item to MediaItem format for downloader
				tempMedia := kodi.MediaItem{
					ID:        item.KodiID,
					Title:     item.Title,
					Year:      item.Year,
					Thumbnail: item.Poster,
				}
				// Map it correctly for filename generation
				saveType := "movie"
				if item.MediaType == "show" || item.MediaType == "season" {
					saveType = "show"
				}

				localURL, err := s.downloadBestImage(client, tempMedia, saveType)
				if err != nil {
					slog.Warn("Failed to download poster image", "error", err)
				} else if localURL != "" {
					item.Poster = localURL
				}
			}
		}

		id, err := s.db.AddItem(item)
		if err != nil {
			slog.Error("Failed to add item to database", "error", err)
			http.Error(w, "Failed to add item", http.StatusInternalServerError)
			return
		}
		item.ID = id
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleItemRoutes(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/items/"), "/")
	if len(pathParts) < 1 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(pathParts[0], 10, 64)
	if err != nil {
		slog.Warn("Invalid item ID in request", "path", pathParts[0], "error", err)
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodDelete {
		if err := s.db.DeleteItem(id); err != nil {
			slog.Error("Failed to delete item", "item_id", id, "error", err)
			http.Error(w, "Failed to delete item", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if len(pathParts) == 2 && pathParts[1] == "reorder" {
		var req struct {
			SortOrder int `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Warn("Invalid request body for reorder", "error", err)
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}
		if err := s.db.UpdateItemOrder(id, req.SortOrder); err != nil {
			slog.Error("Failed to update item order", "item_id", id, "error", err)
			http.Error(w, "Failed to update order", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	listIDStr := r.URL.Query().Get("list_id")
	searchType := r.URL.Query().Get("content_type")
	if searchType == "" {
		searchType = r.URL.Query().Get("type") // Fallback
	}

	lID, err := strconv.ParseInt(listIDStr, 10, 64)
	if err != nil {
		slog.Warn("Invalid list_id in search request", "list_id", listIDStr, "error", err)
		http.Error(w, "Invalid list_id parameter", http.StatusBadRequest)
		return
	}

	cacheType := "movie"
	if searchType == "tv" {
		cacheType = "show"
	}

	count, err := s.db.GetLibraryCacheCount(lID, cacheType)
	if err != nil {
		slog.Error("Failed to get library cache count", "list_id", lID, "type", cacheType, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if count > 0 {
		cached, err := s.db.SearchLibraryCache(lID, cacheType, query)
		if err != nil {
			slog.Error("Failed to search library cache", "list_id", lID, "query", query, "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		var results []kodi.MediaItem
		for _, c := range cached {
			results = append(results, kodi.MediaItem{
				ID: c.KodiID, Title: c.Title, Label: c.Title, Year: c.Year, Thumbnail: c.Poster, Runtime: c.Runtime, EpisodeCount: c.EpisodeCount, Rating: c.Rating, Plot: c.Plot,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
		return
	}

	client, err := s.getKodiClient(lID)
	if err != nil {
		slog.Error("Failed to get Kodi client for search", "list_id", lID, "error", err)
		http.Error(w, "Failed to connect to Kodi", http.StatusInternalServerError)
		return
	}

	var allItems []kodi.MediaItem
	if searchType == "movie" || searchType == "" {
		m, err := client.GetMovies()
		if err != nil {
			slog.Error("Failed to get movies from Kodi", "error", err)
			http.Error(w, "Failed to fetch movies", http.StatusInternalServerError)
			return
		}
		allItems = append(allItems, m...)
	}
	if searchType == "tv" {
		shows, err := client.GetTVShows()
		if err != nil {
			slog.Error("Failed to get TV shows from Kodi", "error", err)
			http.Error(w, "Failed to fetch TV shows", http.StatusInternalServerError)
			return
		}
		allItems = append(allItems, shows...)
	}

	matches := kodi.FuzzySearch(allItems, query)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
}

func (s *Server) handleSyncLibrary(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.ParseInt(r.URL.Query().Get("list_id"), 10, 64)
	if err != nil {
		slog.Warn("Invalid list_id in sync request", "list_id", r.URL.Query().Get("list_id"), "error", err)
		http.Error(w, "Invalid list_id parameter", http.StatusBadRequest)
		return
	}
	syncType := r.URL.Query().Get("content_type")
	if syncType == "" {
		syncType = r.URL.Query().Get("type") // Fallback
	}

	client, err := s.getKodiClient(listID)
	if err != nil {
		slog.Error("Failed to get Kodi client for sync", "list_id", listID, "error", err)
		http.Error(w, "Failed to connect to Kodi", http.StatusInternalServerError)
		return
	}

	var itemsToCache []database.CachedItem
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	switch syncType {
	case "movie":
		movies, err := client.GetMovies()
		if err != nil {
			slog.Error("Error getting movies from Kodi", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Info("Starting parallel sync for movies", "count", len(movies))
		for _, m := range movies {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()

				// Recover from potential panics to prevent deadlock
				defer func() {
					if r := recover(); r != nil {
						slog.Error("Panic in sync movie goroutine", "panic", r)
					}
				}()

				poster, _ := s.downloadBestImage(client, m, "movie")

				mu.Lock()
				itemsToCache = append(itemsToCache, database.CachedItem{
					ListID: listID, KodiID: m.ID, MediaType: "movie", Title: m.Title, Year: m.Year, Poster: poster, Runtime: m.Runtime, Rating: m.Rating, Plot: m.Plot,
				})
				mu.Unlock()
			})
		}
	case "tv":
		shows, err := client.GetTVShows()
		if err != nil {
			slog.Error("Error getting TV shows from Kodi", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Info("Starting parallel sync for shows", "count", len(shows))
		for _, v := range shows {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()

				// Recover from potential panics to prevent deadlock
				defer func() {
					if r := recover(); r != nil {
						slog.Error("Panic in sync show goroutine", "panic", r)
					}
				}()

				poster, _ := s.downloadBestImage(client, v, "show")

				mu.Lock()
				itemsToCache = append(itemsToCache, database.CachedItem{
					ListID: listID, KodiID: v.ID, MediaType: "show", Title: v.Title, Year: v.Year, Poster: poster, Runtime: v.Runtime, EpisodeCount: v.EpisodeCount, Rating: v.Rating, Plot: v.Plot,
				})
				mu.Unlock()
			})
		}
	}

	wg.Wait()
	slog.Info("Finished parallel sync. Saving items to database", "count", len(itemsToCache))

	dbType := "movie"
	if syncType == "tv" {
		dbType = "show"
	}
	if err := s.db.ClearLibraryCache(listID, dbType); err != nil {
		slog.Error("Failed to clear library cache", "list_id", listID, "type", dbType, "error", err)
	}
	if err := s.db.AddToLibraryCache(itemsToCache); err != nil {
		slog.Error("Failed to add items to library cache", "count", len(itemsToCache), "error", err)
		http.Error(w, "Failed to save library cache", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "count": len(itemsToCache)})
}

func (s *Server) handleGetSeasons(w http.ResponseWriter, r *http.Request) {
	showID, err := strconv.Atoi(r.URL.Query().Get("tvshowid"))
	if err != nil {
		slog.Warn("Invalid tvshowid in request", "tvshowid", r.URL.Query().Get("tvshowid"), "error", err)
		http.Error(w, "Invalid tvshowid parameter", http.StatusBadRequest)
		return
	}
	listID, err := strconv.ParseInt(r.URL.Query().Get("list_id"), 10, 64)
	if err != nil {
		slog.Warn("Invalid list_id in seasons request", "list_id", r.URL.Query().Get("list_id"), "error", err)
		http.Error(w, "Invalid list_id parameter", http.StatusBadRequest)
		return
	}
	client, err := s.getKodiClient(listID)
	if err != nil {
		slog.Error("Failed to get Kodi client for seasons", "list_id", listID, "error", err)
		http.Error(w, "Failed to connect to Kodi", http.StatusInternalServerError)
		return
	}
	seasons, err := client.GetSeasons(showID)
	if err != nil {
		slog.Error("Failed to get seasons from Kodi", "show_id", showID, "error", err)
		http.Error(w, "Failed to fetch seasons", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(seasons)
}

func (s *Server) handleGetEpisodes(w http.ResponseWriter, r *http.Request) {
	showID, err := strconv.Atoi(r.URL.Query().Get("tvshowid"))
	if err != nil {
		slog.Warn("Invalid tvshowid in episodes request", "tvshowid", r.URL.Query().Get("tvshowid"), "error", err)
		http.Error(w, "Invalid tvshowid parameter", http.StatusBadRequest)
		return
	}
	season, err := strconv.Atoi(r.URL.Query().Get("season"))
	if err != nil {
		slog.Warn("Invalid season in episodes request", "season", r.URL.Query().Get("season"), "error", err)
		http.Error(w, "Invalid season parameter", http.StatusBadRequest)
		return
	}
	listID, err := strconv.ParseInt(r.URL.Query().Get("list_id"), 10, 64)
	if err != nil {
		slog.Warn("Invalid list_id in episodes request", "list_id", r.URL.Query().Get("list_id"), "error", err)
		http.Error(w, "Invalid list_id parameter", http.StatusBadRequest)
		return
	}
	client, err := s.getKodiClient(listID)
	if err != nil {
		slog.Error("Failed to get Kodi client for episodes", "list_id", listID, "error", err)
		http.Error(w, "Failed to connect to Kodi", http.StatusInternalServerError)
		return
	}
	episodes, err := client.GetEpisodes(showID, season)
	if err != nil {
		slog.Error("Failed to get episodes from Kodi", "show_id", showID, "season", season, "error", err)
		http.Error(w, "Failed to fetch episodes", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(episodes)
}
