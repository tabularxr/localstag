package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/tabular/local-pipeline/internal/config"
	"github.com/tabular/local-pipeline/internal/logging"
	"github.com/tabular/local-pipeline/internal/storage"
)

type Service struct {
	config     *config.Config
	logger     *logging.Logger
	upgrader   websocket.Upgrader
	clients    map[string]*Client
	clientsMux sync.RWMutex
	StartTime  time.Time
}

type Client struct {
	ID         string
	Conn       *websocket.Conn
	SessionID  string
	DeviceID   string
	RemoteAddr string
	StartTime  time.Time
	LastPing   time.Time
	EventCount int64
	BytesReceived int64
}

func NewService(cfg *config.Config, logger *logging.Logger) *Service {
	return &Service{
		config:    cfg,
		logger:    logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for local development
			},
		},
		clients:   make(map[string]*Client),
		StartTime: time.Now(),
	}
}

func (s *Service) Handler() http.Handler {
	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// WebSocket endpoint
	router.HandleFunc("/ws/streamkit", s.handleWebSocket).Methods("GET")

	// Stats endpoint
	router.HandleFunc("/stats", s.handleStats).Methods("GET")

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
			s.logger.Info("HTTP request",
				"method", r.Method,
				"url", r.URL.String(),
				"duration", time.Since(start).String(),
				"remote_addr", r.RemoteAddr,
			)
		})
	})

	return router
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.clientsMux.RLock()
	activeClients := len(s.clients)
	s.clientsMux.RUnlock()

	health := map[string]interface{}{
		"status":             "healthy",
		"timestamp":          time.Now().Format(time.RFC3339),
		"version":            "1.0.0",
		"uptime":             time.Since(s.StartTime).String(),
		"active_connections": activeClients,
		"stag_endpoint":      s.config.RelayEndpoint,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Service) handleStats(w http.ResponseWriter, r *http.Request) {
	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	var totalEvents int64
	var totalBytes int64
	clients := make([]map[string]interface{}, 0, len(s.clients))

	for _, client := range s.clients {
		clients = append(clients, map[string]interface{}{
			"id":             client.ID,
			"session_id":     client.SessionID,
			"device_id":      client.DeviceID,
			"remote_addr":    client.RemoteAddr,
			"start_time":     client.StartTime.Format(time.RFC3339),
			"last_ping":      client.LastPing.Format(time.RFC3339),
			"event_count":    client.EventCount,
			"bytes_received": client.BytesReceived,
			"uptime":         time.Since(client.StartTime).String(),
		})
		totalEvents += client.EventCount
		totalBytes += client.BytesReceived
	}

	stats := map[string]interface{}{
		"service_uptime":     time.Since(s.StartTime).String(),
		"active_connections": len(s.clients),
		"total_events":       totalEvents,
		"total_bytes":        totalBytes,
		"clients":            clients,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Service) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	sessionID := r.URL.Query().Get("session_id")
	deviceID := r.URL.Query().Get("device_id")

	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().Unix())
	}
	if deviceID == "" {
		deviceID = fmt.Sprintf("device_%d", time.Now().Unix())
	}

	// Upgrade connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}

	clientID := fmt.Sprintf("%s_%s_%d", sessionID, deviceID, time.Now().Unix())
	client := &Client{
		ID:         clientID,
		Conn:       conn,
		SessionID:  sessionID,
		DeviceID:   deviceID,
		RemoteAddr: r.RemoteAddr,
		StartTime:  time.Now(),
		LastPing:   time.Now(),
	}

	s.clientsMux.Lock()
	s.clients[clientID] = client
	s.clientsMux.Unlock()

	s.logger.Info("WebSocket client connected",
		"client_id", clientID,
		"session_id", sessionID,
		"device_id", deviceID,
		"remote_addr", r.RemoteAddr,
	)

	defer func() {
		s.clientsMux.Lock()
		delete(s.clients, clientID)
		s.clientsMux.Unlock()
		
		conn.Close()
		s.logger.Info("WebSocket client disconnected",
			"client_id", clientID,
			"session_id", sessionID,
			"device_id", deviceID,
			"events_processed", client.EventCount,
			"bytes_received", client.BytesReceived,
		)
	}()

	// Handle messages
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error("WebSocket read error", "client_id", clientID, "error", err)
			}
			break
		}

		client.LastPing = time.Now()
		client.BytesReceived += int64(len(data))

		switch messageType {
		case websocket.BinaryMessage:
			if err := s.processBinaryMessage(client, data); err != nil {
				s.logger.Error("Failed to process binary message", 
					"client_id", clientID, 
					"error", err,
				)
			}
		case websocket.TextMessage:
			if err := s.processTextMessage(client, data); err != nil {
				s.logger.Error("Failed to process text message", 
					"client_id", clientID, 
					"error", err,
				)
			}
		case websocket.PingMessage:
			conn.WriteMessage(websocket.PongMessage, []byte{})
		}
	}
}

func (s *Service) processTextMessage(client *Client, data []byte) error {
	// Handle JSON messages (session info, etc.)
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to parse JSON message: %w", err)
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return fmt.Errorf("message missing type field")
	}

	switch msgType {
	case "session_info":
		s.logger.Info("Received session info", 
			"client_id", client.ID,
			"session_id", client.SessionID,
			"streams", msg["streams"],
		)
	case "ping":
		// Send pong
		response := map[string]interface{}{
			"type": "pong",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		responseData, _ := json.Marshal(response)
		client.Conn.WriteMessage(websocket.TextMessage, responseData)
	default:
		s.logger.Debug("Unknown message type", "type", msgType, "client_id", client.ID)
	}

	return nil
}

func (s *Service) processBinaryMessage(client *Client, data []byte) error {
	// Parse binary StreamKit packet
	packet, err := s.parseStreamKitPacket(data)
	if err != nil {
		return fmt.Errorf("failed to parse StreamKit packet: %w", err)
	}

	// Convert to spatial events
	events := s.convertToSpatialEvents(packet, client)

	// Create ingest batch
	batch := &storage.IngestBatch{
		BatchID:    fmt.Sprintf("batch_%s_%d", client.ID, time.Now().Unix()),
		Events:     events,
		Timestamp:  time.Now(),
		RelayID:    "local-relay",
		ProcessingInfo: storage.ProcessingInfo{
			ReceivedAt:  time.Now(),
			ProcessedAt: time.Now(),
			Relay:       "local-relay",
		},
	}

	// Forward to Stag service
	if err := s.forwardToStag(batch); err != nil {
		return fmt.Errorf("failed to forward to stag: %w", err)
	}

	client.EventCount += int64(len(events))

	s.logger.Info("Processed StreamKit packet",
		"client_id", client.ID,
		"session_id", client.SessionID,
		"frame_number", packet.FrameNumber,
		"events", len(events),
	)

	return nil
}

func (s *Service) parseStreamKitPacket(data []byte) (*StreamKitPacket, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("packet too short")
	}

	// Simple packet parsing - in real implementation, this would be more sophisticated
	packet := &StreamKitPacket{
		Magic:       string(data[:4]),
		Version:     1,
		FrameNumber: uint64(time.Now().Unix()),
		Timestamp:   time.Now(),
		Streams:     []StreamData{},
	}

	// For now, create a mock mesh stream
	stream := StreamData{
		Type: "mesh",
		Data: data[16:], // Rest of the data
	}
	packet.Streams = append(packet.Streams, stream)

	return packet, nil
}

func (s *Service) convertToSpatialEvents(packet *StreamKitPacket, client *Client) []storage.SpatialEvent {
	events := []storage.SpatialEvent{}

	for _, stream := range packet.Streams {
		event := storage.SpatialEvent{
			EventID:     fmt.Sprintf("event_%s_%d_%s", client.ID, packet.FrameNumber, stream.Type),
			EventType:   stream.Type,
			Timestamp:   packet.Timestamp,
			ServerTime:  time.Now(),
			SessionID:   client.SessionID,
			ClientID:    client.ID,
			DeviceID:    client.DeviceID,
			FrameNumber: packet.FrameNumber,
			Metadata:    make(map[string]interface{}),
			ProcessingInfo: storage.ProcessingInfo{
				ReceivedAt:  time.Now(),
				ProcessedAt: time.Now(),
				Relay:       "local-relay",
			},
		}

		// Add stream-specific data
		switch stream.Type {
		case "mesh":
			event.MeshData = &storage.MeshData{
				AnchorID:   fmt.Sprintf("anchor_%s_%d", client.DeviceID, packet.FrameNumber),
				Vertices:   []float64{0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0},
				Faces:      []uint32{0, 1, 2},
				Transform:  &storage.Transform{Translation: [3]float64{0, 0, 0}, Rotation: [4]float64{0, 0, 0, 1}, Scale: [3]float64{1, 1, 1}},
			}
		case "pose":
			event.PoseData = &storage.PoseData{
				Transform: &storage.Transform{Translation: [3]float64{0, 0, 0}, Rotation: [4]float64{0, 0, 0, 1}, Scale: [3]float64{1, 1, 1}},
			}
		case "camera":
			event.CameraData = &storage.CameraData{
				ImageData:  stream.Data,
				Width:      640,
				Height:     480,
				Format:     "jpeg",
				Intrinsics: [9]float64{500, 0, 320, 0, 500, 240, 0, 0, 1},
				Transform:  &storage.Transform{Translation: [3]float64{0, 0, 0}, Rotation: [4]float64{0, 0, 0, 1}, Scale: [3]float64{1, 1, 1}},
				Timestamp:  packet.Timestamp,
			}
		}

		events = append(events, event)
	}

	return events
}

func (s *Service) forwardToStag(batch *storage.IngestBatch) error {
	// Serialize batch
	jsonData, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", s.config.RelayEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "tabular-relay/1.0.0")

	// Send request with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stag service returned status %d: %s", resp.StatusCode, string(body))
	}

	s.logger.Debug("Forwarded batch to stag",
		"batch_id", batch.BatchID,
		"events", len(batch.Events),
		"endpoint", s.config.RelayEndpoint,
	)

	return nil
}

type StreamKitPacket struct {
	Magic       string
	Version     uint16
	FrameNumber uint64
	Timestamp   time.Time
	Streams     []StreamData
}

type StreamData struct {
	Type string
	Data []byte
}