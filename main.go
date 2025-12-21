package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"whats-next/internal/database"
	"whats-next/internal/server"
)

func main() {
	// Setup structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	// Ensure data directory exists
	os.MkdirAll("data", 0755)

	db, err := database.InitDB("data/whats-next.db")
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Load Config
	// Prioritize /config/config.json (Docker volume), fallback to local config.json
	configFile := "/config/config.json"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		configFile = "config.json"
	}

	var fullConfig database.Config
	if _, err := os.Stat(configFile); err == nil {
		slog.Info("Loading config from file", "path", configFile)
		file, _ := os.Open(configFile)
		if err := json.NewDecoder(file).Decode(&fullConfig); err == nil {
			if err := db.SyncLists(fullConfig.Lists); err != nil {
				slog.Error("Error syncing lists from config", "error", err)
			} else {
				slog.Info("Successfully synced lists from config", "count", len(fullConfig.Lists))
			}
		} else {
			// Backwards compatibility for old array-only config
			file.Seek(0, 0)
			var listConfig []database.List
			if err := json.NewDecoder(file).Decode(&listConfig); err == nil {
				slog.Info("Detected legacy list-only config format")
				db.SyncLists(listConfig)
			} else {
				slog.Error("Error decoding config file", "error", err)
			}
		}
		file.Close()
	}

	if os.Getenv("MOCK_KODI") == "true" {
		slog.Warn("*****************************************")
		slog.Warn("!!!  RUNNING IN MOCK KODI MODE (STUB) !!!")
		slog.Warn("*****************************************")
	} else {
		slog.Info("--- RUNNING IN REAL KODI MODE ---")
	}

	srv := server.NewServer(db, fullConfig)

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

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      http.DefaultServeMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Graceful shutdown handling
	go func() {
		slog.Info("Starting server", "port", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server gracefully...")

	// Give connections 30 seconds to drain
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exited")
}
