package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"whats-next/internal/database"
	"whats-next/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	// Ensure data directory exists
	os.MkdirAll("data", 0755)

	db, err := database.InitDB("data/whats-next.db")
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	// Load Config
	// Prioritize /config/config.json (Docker volume), fallback to local config.json
	configFile := "/config/config.json"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		configFile = "config.json"
	}

	if _, err := os.Stat(configFile); err == nil {
		log.Printf("Loading config from %s...", configFile)
		file, _ := os.Open(configFile)
		var fullConfig database.Config
		if err := json.NewDecoder(file).Decode(&fullConfig); err == nil {
			// Update Global Config
			server.CurrentConfig = fullConfig

			if err := db.SyncLists(fullConfig.Lists); err != nil {
				log.Printf("Error syncing lists from config: %v", err)
			} else {
				log.Printf("Successfully synced %d lists from config", len(fullConfig.Lists))
			}
		} else {
			// Backwards compatibility for old array-only config
			file.Seek(0, 0)
			var listConfig []database.List
			if err := json.NewDecoder(file).Decode(&listConfig); err == nil {
				log.Println("Detected legacy list-only config format")
				db.SyncLists(listConfig)
			} else {
				log.Printf("Error decoding config file: %v", err)
			}
		}
		file.Close()
	}

	if os.Getenv("MOCK_KODI") == "true" {
		log.Println("*****************************************")
		log.Println("!!!  RUNNING IN MOCK KODI MODE (STUB) !!!")
		log.Println("*****************************************")
	} else {
		log.Println("--- RUNNING IN REAL KODI MODE ---")
	}

	srv := server.NewServer(db)

	// API routes
	http.Handle("/api/", http.StripPrefix("/api", srv.Routes()))

	// Static frontend
	fs := http.FileServer(http.Dir("web/dist"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If requesting a file that exists, serve it
		path := filepath.Join("web/dist", r.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) || r.URL.Path == "/" {
			http.ServeFile(w, r, "web/dist/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})

	log.Printf("Starting server on port %s...", port)
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      http.DefaultServeMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
