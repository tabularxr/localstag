package main

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
}