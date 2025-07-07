package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

const (
	StagPort  = 9000
	RelayPort = 8080
)

var (
	version = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "Show version information")
		initDB      = flag.Bool("init", false, "Initialize a new database")
		clean       = flag.Bool("clean", false, "Clean database (remove all data)")
		listStags   = flag.Bool("list", false, "List all stags")
		stats       = flag.Bool("stats", false, "Show system statistics")
		dbPath      = flag.String("db", "./stag-data", "Database directory path")
		logLevel    = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		stagOnly    = flag.Bool("stag-only", false, "Start only the Stag service")
		relayOnly   = flag.Bool("relay-only", false, "Start only the Relay service")
		test        = flag.Bool("test", false, "Run test client simulation")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("🚀 Tabular Local Pipeline\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		os.Exit(0)
	}

	// Build services if needed
	if err := buildServices(); err != nil {
		log.Fatalf("Failed to build services: %v", err)
	}

	// Handle CLI operations
	if *initDB || *clean || *listStags || *stats {
		if err := runStagCLI(*initDB, *clean, *listStags, *stats, *dbPath, *logLevel); err != nil {
			log.Fatalf("CLI operation failed: %v", err)
		}
		return
	}

	// Get LAN IP
	lanIP, err := getLANIP()
	if err != nil {
		log.Printf("Warning: Could not determine LAN IP: %v", err)
		lanIP = "localhost"
	}

	fmt.Printf("🦾 Starting Tabular Local Pipeline...\n\n")

	// Start services
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Stag service
	if !*relayOnly {
		go func() {
			if err := startStagService(ctx, *dbPath, *logLevel); err != nil {
				log.Printf("Stag service error: %v", err)
			}
		}()
		
		// Wait for Stag to be ready
		if err := waitForService(fmt.Sprintf("http://localhost:%d/health", StagPort)); err != nil {
			log.Fatalf("Stag service failed to start: %v", err)
		}
	}

	// Start Relay service
	if !*stagOnly {
		go func() {
			if err := startRelayService(ctx, *logLevel); err != nil {
				log.Printf("Relay service error: %v", err)
			}
		}()
		
		// Wait for Relay to be ready
		if err := waitForService(fmt.Sprintf("http://localhost:%d/health", RelayPort)); err != nil {
			log.Fatalf("Relay service failed to start: %v", err)
		}
	}

	// Display connection information
	fmt.Printf("✅ Services Ready!\n\n")
	
	if !*relayOnly {
		fmt.Printf("🌍 Stag Service: http://localhost:%d\n", StagPort)
		fmt.Printf("   - Health: http://localhost:%d/health\n", StagPort)
		fmt.Printf("   - API: http://localhost:%d/api/v1/\n", StagPort)
		fmt.Printf("   - Database: %s\n", *dbPath)
		fmt.Printf("\n")
	}
	
	if !*stagOnly {
		fmt.Printf("🌐 Relay Service: ws://%s:%d/ws/streamkit\n", lanIP, RelayPort)
		fmt.Printf("   - Local URL: ws://localhost:%d/ws/streamkit\n", RelayPort)
		fmt.Printf("   - Health: http://localhost:%d/health\n", RelayPort)
		fmt.Printf("   - Stats: http://localhost:%d/stats\n", RelayPort)
		fmt.Printf("\n")
	}

	fmt.Printf("📱 StreamKit Connection:\n")
	fmt.Printf("   URL: ws://%s:%d/ws/streamkit\n", lanIP, RelayPort)
	fmt.Printf("   Parameters: ?session_id=your_session&device_id=your_device\n")
	fmt.Printf("\n")

	// Run test client if requested
	if *test {
		fmt.Printf("🧪 Running test client...\n")
		go func() {
			time.Sleep(2 * time.Second) // Give services time to start
			if err := runTestClient(); err != nil {
				log.Printf("Test client error: %v", err)
			}
		}()
	}

	fmt.Printf("💡 Commands:\n")
	fmt.Printf("   - View logs: tail -f local-pipeline.log\n")
	fmt.Printf("   - List stags: ./local-pipeline -list\n")
	fmt.Printf("   - Show stats: ./local-pipeline -stats\n")
	fmt.Printf("   - Clean data: ./local-pipeline -clean\n")
	fmt.Printf("   - Run test: ./local-pipeline -test\n")
	fmt.Printf("\n")
	fmt.Printf("🛑 Press Ctrl+C to stop all services\n")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Printf("\n🔄 Shutting down services...\n")
	cancel()
	time.Sleep(2 * time.Second)
	fmt.Printf("✅ Services stopped successfully\n")
}

func buildServices() error {
	// Check if binaries exist
	stagBin := "./bin/stag"
	relayBin := "./bin/relay"
	
	if _, err := os.Stat(stagBin); os.IsNotExist(err) {
		fmt.Printf("🔨 Building Stag service...\n")
		if err := os.MkdirAll("./bin", 0755); err != nil {
			return fmt.Errorf("failed to create bin directory: %w", err)
		}
		
		cmd := exec.Command("go", "build", "-o", stagBin, "./cmd/stag")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build stag: %w", err)
		}
	}
	
	if _, err := os.Stat(relayBin); os.IsNotExist(err) {
		fmt.Printf("🔨 Building Relay service...\n")
		if err := os.MkdirAll("./bin", 0755); err != nil {
			return fmt.Errorf("failed to create bin directory: %w", err)
		}
		
		cmd := exec.Command("go", "build", "-o", relayBin, "./cmd/relay")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build relay: %w", err)
		}
	}
	
	return nil
}

func runStagCLI(initDB, clean, listStags, stats bool, dbPath, logLevel string) error {
	args := []string{
		"-db", dbPath,
		"-log-level", logLevel,
	}
	
	if initDB {
		args = append(args, "-init")
	}
	if clean {
		args = append(args, "-clean")
	}
	if listStags {
		args = append(args, "-list")
	}
	if stats {
		args = append(args, "-stats")
	}
	
	cmd := exec.Command("./bin/stag", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startStagService(ctx context.Context, dbPath, logLevel string) error {
	fmt.Printf("🌍 Starting Stag service on port %d...\n", StagPort)
	
	cmd := exec.CommandContext(ctx, "./bin/stag",
		"-port", fmt.Sprintf("%d", StagPort),
		"-db", dbPath,
		"-log-level", logLevel,
	)
	
	// Create log file
	logFile, err := os.OpenFile("stag.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()
	
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	return cmd.Run()
}

func startRelayService(ctx context.Context, logLevel string) error {
	fmt.Printf("🚀 Starting Relay service on port %d...\n", RelayPort)
	
	cmd := exec.CommandContext(ctx, "./bin/relay",
		"-port", fmt.Sprintf("%d", RelayPort),
		"-stag-endpoint", fmt.Sprintf("http://localhost:%d/api/v1/ingest", StagPort),
		"-log-level", logLevel,
	)
	
	// Create log file
	logFile, err := os.OpenFile("relay.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()
	
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	return cmd.Run()
}

func waitForService(healthURL string) error {
	for i := 0; i < 30; i++ {
		if resp, err := http.Get(healthURL); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("service did not become ready within 30 seconds")
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

func runTestClient() error {
	// Create a simple test client
	testClientPath := "./bin/test-client"
	
	// Build test client if it doesn't exist
	if _, err := os.Stat(testClientPath); os.IsNotExist(err) {
		fmt.Printf("🔨 Building test client...\n")
		if err := buildTestClient(); err != nil {
			return fmt.Errorf("failed to build test client: %w", err)
		}
	}
	
	cmd := exec.Command(testClientPath,
		"-url", fmt.Sprintf("ws://localhost:%d/ws/streamkit", RelayPort),
		"-session", "test-session",
		"-device", "test-device",
		"-count", "10",
	)
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

func buildTestClient() error {
	// Create a simple test client
	testClientSource := `package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	var (
		urlFlag = flag.String("url", "ws://localhost:8080/ws/streamkit", "WebSocket URL")
		session = flag.String("session", "test-session", "Session ID")
		device  = flag.String("device", "test-device", "Device ID")
		count   = flag.Int("count", 10, "Number of test messages to send")
	)
	flag.Parse()

	u, err := url.Parse(*urlFlag)
	if err != nil {
		log.Fatal("Invalid URL:", err)
	}

	q := u.Query()
	q.Set("session_id", *session)
	q.Set("device_id", *device)
	u.RawQuery = q.Encode()

	fmt.Printf("🧪 Test Client connecting to %s\n", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Dial error:", err)
	}
	defer c.Close()

	fmt.Printf("✅ Connected successfully\n")

	// Send session info
	sessionInfo := map[string]interface{}{
		"type":       "session_info",
		"sessionID":  *session,
		"streams":    []map[string]interface{}{{"type": "mesh", "compression": "none"}},
		"targetFPS":  30,
		"sdkVersion": "test-1.0.0",
	}

	if err := c.WriteJSON(sessionInfo); err != nil {
		log.Fatal("Write error:", err)
	}

	// Send test packets
	for i := 0; i < *count; i++ {
		// Create a mock binary packet
		testData := make([]byte, 100)
		copy(testData[:4], []byte("TEST"))
		
		if err := c.WriteMessage(websocket.BinaryMessage, testData); err != nil {
			log.Fatal("Write error:", err)
		}
		
		fmt.Printf("📦 Sent test packet %d/%d\n", i+1, *count)
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("✅ Test completed successfully\n")
}`

	testClientPath := "./cmd/test-client/main.go"
	if err := os.MkdirAll(filepath.Dir(testClientPath), 0755); err != nil {
		return fmt.Errorf("failed to create test client directory: %w", err)
	}

	if err := os.WriteFile(testClientPath, []byte(testClientSource), 0644); err != nil {
		return fmt.Errorf("failed to write test client source: %w", err)
	}

	cmd := exec.Command("go", "mod", "download", "github.com/gorilla/websocket")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download websocket dependency: %w", err)
	}

	cmd = exec.Command("go", "build", "-o", "./bin/test-client", "./cmd/test-client")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build test client: %w", err)
	}

	return nil
}