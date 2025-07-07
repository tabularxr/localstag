package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/tabular/local-pipeline/internal/stag"
	"github.com/tabular/local-pipeline/internal/storage"
	"github.com/tabular/local-pipeline/internal/config"
	"github.com/tabular/local-pipeline/internal/logging"
)

var (
	version = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	var (
		configPath   = flag.String("config", "", "Path to configuration file")
		port         = flag.Int("port", 9000, "HTTP server port")
		dbPath       = flag.String("db", "./stag-data", "Database directory path")
		logLevel     = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		showVersion  = flag.Bool("version", false, "Show version information")
		initDB       = flag.Bool("init", false, "Initialize a new database")
		listStags    = flag.Bool("list", false, "List all stags")
		showStats    = flag.Bool("stats", false, "Show system statistics")
		cleanDB      = flag.Bool("clean", false, "Clean database (remove all data)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Tabular Local Stag Service\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		os.Exit(0)
	}

	// Initialize logger
	logger := logging.NewLogger(*logLevel)
	logger.Info("Starting Tabular Local Stag Service",
		"version", version,
		"port", *port,
		"db_path", *dbPath,
		"log_level", *logLevel,
	)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Override config with command line flags
	if *port != 9000 {
		cfg.Port = *port
	}
	if *dbPath != "./stag-data" {
		cfg.DatabasePath = *dbPath
	}
	if *logLevel != "info" {
		cfg.LogLevel = *logLevel
	}

	// Initialize storage
	store, err := storage.NewBoltStorage(cfg.DatabasePath)
	if err != nil {
		logger.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// Handle CLI operations
	if *initDB {
		logger.Info("Initializing database", "path", cfg.DatabasePath)
		if err := initializeDatabase(store, logger); err != nil {
			logger.Error("Failed to initialize database", "error", err)
			os.Exit(1)
		}
		logger.Info("Database initialized successfully")
		return
	}

	if *listStags {
		if err := listAllStags(store, logger); err != nil {
			logger.Error("Failed to list stags", "error", err)
			os.Exit(1)
		}
		return
	}

	if *showStats {
		if err := showSystemStats(store, logger); err != nil {
			logger.Error("Failed to show stats", "error", err)
			os.Exit(1)
		}
		return
	}

	if *cleanDB {
		logger.Info("Cleaning database", "path", cfg.DatabasePath)
		if err := cleanDatabase(store, logger); err != nil {
			logger.Error("Failed to clean database", "error", err)
			os.Exit(1)
		}
		logger.Info("Database cleaned successfully")
		return
	}

	// Start HTTP server
	startServer(cfg, store, logger)
}

func startServer(cfg *config.Config, store storage.Storage, logger *logging.Logger) {
	// Initialize service
	service := stag.NewService(store, logger)

	// Setup HTTP routes
	router := mux.NewRouter()
	
	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"status": "healthy",
			"timestamp": "%s",
			"version": "%s",
			"uptime": "%s"
		}`, time.Now().Format(time.RFC3339), version, time.Since(service.StartTime))
	}).Methods("GET")

	// API routes
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	
	// Ingest endpoint
	apiRouter.HandleFunc("/ingest", service.HandleIngest).Methods("POST")
	
	// Query endpoints
	apiRouter.HandleFunc("/stags", service.HandleListStags).Methods("GET")
	apiRouter.HandleFunc("/stags/{stag_id}", service.HandleGetStag).Methods("GET")
	apiRouter.HandleFunc("/stags/{stag_id}/anchors", service.HandleListAnchors).Methods("GET")
	apiRouter.HandleFunc("/stags/{stag_id}/anchors/{anchor_id}", service.HandleGetAnchor).Methods("GET")
	apiRouter.HandleFunc("/stags/{stag_id}/anchors/{anchor_id}/history", service.HandleGetAnchorHistory).Methods("GET")
	apiRouter.HandleFunc("/stats", service.HandleGetStats).Methods("GET")
	apiRouter.HandleFunc("/stats/{stag_id}", service.HandleGetStagStats).Methods("GET")

	// Enable CORS
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Request logging middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("HTTP request",
				"method", r.Method,
				"url", r.URL.String(),
				"duration", time.Since(start).String(),
				"remote_addr", r.RemoteAddr,
			)
		})
	})

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("🌍 Stag service started", 
			"port", cfg.Port,
			"endpoint", fmt.Sprintf("http://localhost:%d", cfg.Port),
			"db_path", cfg.DatabasePath,
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped successfully")
}

func initializeDatabase(store storage.Storage, logger *logging.Logger) error {
	// Initialize system stats
	stats := &storage.SystemStats{
		StartTime:     time.Now(),
		StagCount:     0,
		AnchorCount:   0,
		VersionCount:  0,
		EventCount:    0,
		LastIngestTime: time.Time{},
	}
	
	return store.UpdateSystemStats(stats)
}

func listAllStags(store storage.Storage, logger *logging.Logger) error {
	stags, err := store.ListStags()
	if err != nil {
		return err
	}

	fmt.Printf("📊 Total Stags: %d\n\n", len(stags))
	
	for _, stag := range stags {
		fmt.Printf("🎯 Stag: %s\n", stag.ID)
		fmt.Printf("   Name: %s\n", stag.Name)
		fmt.Printf("   Description: %s\n", stag.Description)
		fmt.Printf("   Created: %s\n", stag.CreatedAt.Format(time.RFC3339))
		fmt.Printf("   Updated: %s\n", stag.UpdatedAt.Format(time.RFC3339))
		fmt.Printf("   Anchors: %d\n", len(stag.Anchors))
		fmt.Printf("   Events: %d\n", stag.Stats.EventCount)
		fmt.Printf("   Sessions: %d\n", stag.Stats.SessionCount)
		fmt.Printf("   Clients: %d\n", stag.Stats.ClientCount)
		fmt.Printf("   Last Activity: %s\n", stag.Stats.LastActivity.Format(time.RFC3339))
		fmt.Printf("\n")
	}

	return nil
}

func showSystemStats(store storage.Storage, logger *logging.Logger) error {
	stats, err := store.GetSystemStats()
	if err != nil {
		return err
	}

	fmt.Printf("📈 System Statistics\n")
	fmt.Printf("==================\n")
	fmt.Printf("Start Time: %s\n", stats.StartTime.Format(time.RFC3339))
	fmt.Printf("Uptime: %s\n", time.Since(stats.StartTime))
	fmt.Printf("Stags: %d\n", stats.StagCount)
	fmt.Printf("Anchors: %d\n", stats.AnchorCount)
	fmt.Printf("Versions: %d\n", stats.VersionCount)
	fmt.Printf("Events: %d\n", stats.EventCount)
	
	if !stats.LastIngestTime.IsZero() {
		fmt.Printf("Last Ingest: %s\n", stats.LastIngestTime.Format(time.RFC3339))
		fmt.Printf("Time Since Last Ingest: %s\n", time.Since(stats.LastIngestTime))
	} else {
		fmt.Printf("Last Ingest: Never\n")
	}

	return nil
}

func cleanDatabase(store storage.Storage, logger *logging.Logger) error {
	// Get all stags first
	stags, err := store.ListStags()
	if err != nil {
		return err
	}

	// Delete all stags
	for _, stag := range stags {
		if err := store.DeleteStag(stag.ID); err != nil {
			return fmt.Errorf("failed to delete stag %s: %w", stag.ID, err)
		}
	}

	// Reset system stats
	stats := &storage.SystemStats{
		StartTime:     time.Now(),
		StagCount:     0,
		AnchorCount:   0,
		VersionCount:  0,
		EventCount:    0,
		LastIngestTime: time.Time{},
	}

	return store.UpdateSystemStats(stats)
}