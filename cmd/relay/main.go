package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tabular/local-pipeline/internal/relay"
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
		port         = flag.Int("port", 8080, "WebSocket server port")
		stagEndpoint = flag.String("stag-endpoint", "http://localhost:9000/ingest", "Stag service endpoint")
		logLevel     = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		showVersion  = flag.Bool("version", false, "Show version information")
		showIP       = flag.Bool("ip", false, "Show LAN IP address")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Tabular Local Relay Service\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		os.Exit(0)
	}

	if *showIP {
		ip, err := getLANIP()
		if err != nil {
			fmt.Printf("Error getting LAN IP: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("LAN IP: %s\n", ip)
		os.Exit(0)
	}

	// Initialize logger
	logger := logging.NewLogger(*logLevel)
	logger.Info("Starting Tabular Local Relay Service",
		"version", version,
		"port", *port,
		"stag_endpoint", *stagEndpoint,
		"log_level", *logLevel,
	)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Override config with command line flags
	cfg.Port = *port
	if *stagEndpoint != "http://localhost:9000/ingest" {
		cfg.RelayEndpoint = *stagEndpoint
	}
	if *logLevel != "info" {
		cfg.LogLevel = *logLevel
	}

	// Get LAN IP for display
	lanIP, err := getLANIP()
	if err != nil {
		logger.Warn("Failed to get LAN IP", "error", err)
		lanIP = "localhost"
	}

	// Create relay service
	relayService := relay.NewService(cfg, logger)

	// Setup HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      relayService.Handler(),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("🚀 Relay service started",
			"port", cfg.Port,
			"websocket_url", fmt.Sprintf("ws://%s:%d/ws/streamkit", lanIP, cfg.Port),
			"local_url", fmt.Sprintf("ws://localhost:%d/ws/streamkit", cfg.Port),
			"stag_endpoint", cfg.RelayEndpoint,
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

func getLANIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}