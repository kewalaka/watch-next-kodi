package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	db *database.DB
}

func NewServer(db *database.DB) *Server {
	return &Server{db: db}
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

// Global config store (simple in-memory for now)
var CurrentConfig database.Config

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"subtitle": CurrentConfig.Subtitle,
		"footer":   CurrentConfig.Footer,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) getKodiClient(listID int64) (*kodi.Client, error) {
	lists, _ := s.db.GetAllLists()
	var list *database.List
	for _, l := range lists {
		if l.ID == listID {
			list = &l
			break
		}
	}
	if list == nil {
		return nil, http.ErrNoLocation
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

	log.Printf("Downloading best image for %s %d: %s -> %s\n", mediaType, item.ID, item.Title, localPath)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		log.Printf("Invalid request for %s: %v\n", targetURL, err)
		return "", err
	}
	if client.Username != "" {
		req.SetBasicAuth(client.Username, client.Password)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Network error downloading image for %s %d: %v\n", mediaType, item.ID, err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			log.Printf("Image not found on Kodi (404) for %s: %s\n", item.Title, targetURL)
			return "", nil // Return empty, not error, to keep sync going
		}
		log.Printf("Kodi returned %d for image: %s\n", resp.StatusCode, targetURL)
		return "", fmt.Errorf("kodi image error: %d", resp.StatusCode)
	}

	out, err := os.Create(localPath)
	if err != nil {
		log.Printf("File creation error for %s: %v\n", localPath, err)
		return "", err
	}
	defer out.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		log.Printf("Copy error for %s: %v\n", localPath, err)
		return "", err
	}

	log.Printf("Successfully saved image %s (%d bytes)\n", localPath, n)
	return publicURL, nil
}

func (s *Server) handleLists(w http.ResponseWriter, r *http.Request) {
	lists, err := s.db.GetAllLists()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(lists)
}

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/lists/"), "/")
	if len(pathParts) < 2 || pathParts[1] != "items" {
		http.NotFound(w, r)
		return
	}
	listID, _ := strconv.ParseInt(pathParts[0], 10, 64)

	if r.Method == http.MethodGet {
		items, _ := s.db.GetItems(listID)
		json.NewEncoder(w).Encode(items)
		return
	}

	if r.Method == http.MethodPost {
		var item database.Item
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		item.ListID = listID

		// Ensure we have a local poster if it's a remote URL
		if strings.HasPrefix(item.Poster, "image://") || strings.HasPrefix(item.Poster, "http") {
			client, err := s.getKodiClient(listID)
			if err == nil {
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
				if err == nil && localURL != "" {
					item.Poster = localURL
				}
			}
		}

		id, err := s.db.AddItem(item)
		if err != nil {
			log.Printf("Error adding item: %v", err)
		}
		item.ID = id
		json.NewEncoder(w).Encode(item)
		return
	}
}

func (s *Server) handleItemRoutes(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/items/"), "/")
	id, _ := strconv.ParseInt(pathParts[0], 10, 64)
	if r.Method == http.MethodDelete {
		s.db.DeleteItem(id)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(pathParts) == 2 && pathParts[1] == "reorder" {
		var req struct {
			SortOrder int `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}
		s.db.UpdateItemOrder(id, req.SortOrder)
		w.WriteHeader(http.StatusOK)
		return
	}
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	listIDStr := r.URL.Query().Get("list_id")
	searchType := r.URL.Query().Get("type")
	lID, _ := strconv.ParseInt(listIDStr, 10, 64)

	cacheType := "movie"
	if searchType == "tv" {
		cacheType = "show"
	}

	count, _ := s.db.GetLibraryCacheCount(lID, cacheType)
	if count > 0 {
		cached, _ := s.db.SearchLibraryCache(lID, cacheType, query)
		var results []kodi.MediaItem
		for _, c := range cached {
			results = append(results, kodi.MediaItem{
				ID: c.KodiID, Title: c.Title, Label: c.Title, Year: c.Year, Thumbnail: c.Poster, Runtime: c.Runtime, EpisodeCount: c.EpisodeCount, Rating: c.Rating, Plot: c.Plot,
			})
		}
		json.NewEncoder(w).Encode(results)
		return
	}

	client, _ := s.getKodiClient(lID)
	var allItems []kodi.MediaItem
	if searchType == "movies" || searchType == "" {
		m, _ := client.GetMovies()
		allItems = append(allItems, m...)
	}
	if searchType == "tv" {
		shows, _ := client.GetTVShows()
		allItems = append(allItems, shows...)
	}
	matches := kodi.FuzzySearch(allItems, query)
	json.NewEncoder(w).Encode(matches)
}

func (s *Server) handleSyncLibrary(w http.ResponseWriter, r *http.Request) {
	listID, _ := strconv.ParseInt(r.URL.Query().Get("list_id"), 10, 64)
	syncType := r.URL.Query().Get("type")

	client, err := s.getKodiClient(listID)
	if err != nil {
		http.Error(w, "Client error", http.StatusInternalServerError)
		return
	}

	dbType := "movie"
	if syncType == "tv" {
		dbType = "show"
	}

	var itemsToCache []database.CachedItem
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	switch syncType {
	case "movies":
		movies, err := client.GetMovies()
		if err != nil {
			log.Printf("Error getting movies from Kodi: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Starting parallel sync for %d movies...\n", len(movies))
		for _, m := range movies {
			wg.Add(1)
			go func(m kodi.MediaItem) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				poster, _ := s.downloadBestImage(client, m, "movie")

				mu.Lock()
				itemsToCache = append(itemsToCache, database.CachedItem{
					ListID: listID, KodiID: m.ID, MediaType: "movie", Title: m.Title, Year: m.Year, Poster: poster, Runtime: m.Runtime, Rating: m.Rating, Plot: m.Plot,
				})
				mu.Unlock()
			}(m)
		}
	case "tv":
		shows, err := client.GetTVShows()
		if err != nil {
			log.Printf("Error getting TV shows from Kodi: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Starting parallel sync for %d shows...\n", len(shows))
		for _, v := range shows {
			wg.Add(1)
			go func(v kodi.MediaItem) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				poster, _ := s.downloadBestImage(client, v, "show")

				mu.Lock()
				itemsToCache = append(itemsToCache, database.CachedItem{
					ListID: listID, KodiID: v.ID, MediaType: "show", Title: v.Title, Year: v.Year, Poster: poster, Runtime: v.Runtime, EpisodeCount: v.EpisodeCount, Rating: v.Rating, Plot: v.Plot,
				})
				mu.Unlock()
			}(v)
		}
	}

	wg.Wait()
	log.Printf("Finished parallel sync. Saving %d items to database...\n", len(itemsToCache))

	s.db.ClearLibraryCache(listID, dbType)
	s.db.AddToLibraryCache(itemsToCache)

	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "count": len(itemsToCache)})
}

func (s *Server) handleGetSeasons(w http.ResponseWriter, r *http.Request) {
	showID, _ := strconv.Atoi(r.URL.Query().Get("tvshowid"))
	listID, _ := strconv.ParseInt(r.URL.Query().Get("list_id"), 10, 64)
	client, _ := s.getKodiClient(listID)
	seasons, _ := client.GetSeasons(showID)
	json.NewEncoder(w).Encode(seasons)
}

func (s *Server) handleGetEpisodes(w http.ResponseWriter, r *http.Request) {
	showID, _ := strconv.Atoi(r.URL.Query().Get("tvshowid"))
	season, _ := strconv.Atoi(r.URL.Query().Get("season"))
	listID, _ := strconv.ParseInt(r.URL.Query().Get("list_id"), 10, 64)
	client, _ := s.getKodiClient(listID)
	episodes, _ := client.GetEpisodes(showID, season)
	json.NewEncoder(w).Encode(episodes)
}
