package stag

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/tabular/local-pipeline/internal/logging"
	"github.com/tabular/local-pipeline/internal/performance"
	"github.com/tabular/local-pipeline/internal/storage"
)

type Service struct {
	store           storage.Storage
	logger          *logging.Logger
	StartTime       time.Time
	batchProcessor  *performance.BatchProcessor
	processingMutex sync.RWMutex
	healthChecker   *HealthChecker
}

type HealthChecker struct {
	mu              sync.RWMutex
	stagHealth      map[string]*StagHealth
	lastHealthCheck time.Time
}

type StagHealth struct {
	ID              string
	Healthy         bool
	LastActivity    time.Time
	AnchorCount     int
	ErrorCount      int
	LastError       error
	LastErrorTime   time.Time
	ProcessingRate  float64
	LastChecked     time.Time
}

func NewService(store storage.Storage, logger *logging.Logger) *Service {
	s := &Service{
		store:     store,
		logger:    logger,
		StartTime: time.Now(),
		healthChecker: &HealthChecker{
			stagHealth:      make(map[string]*StagHealth),
			lastHealthCheck: time.Now(),
		},
	}
	
	// Initialize batch processor for performance
	s.batchProcessor = performance.NewBatchProcessor(
		logger,
		50, // batch size
		100*time.Millisecond, // flush timeout
		s.processBatch,
	)
	
	// Start health monitoring
	go s.startHealthMonitoring()
	
	return s
}

func (s *Service) HandleIngest(w http.ResponseWriter, r *http.Request) {
	stopTimer := s.logger.StartTimer()
	defer func() {
		duration := stopTimer()
		s.logger.Debug("Ingest request completed", "duration", duration)
	}()
	
	// Parse request body
	var batch storage.IngestBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		s.logger.Error("Failed to decode ingest batch", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	traceID := logging.GenerateTraceID()
	ctx := &logging.PipelineContext{
		TraceID:   traceID,
		BatchID:   batch.BatchID,
		Component: "stag-ingest",
	}

	s.logger.PipelineInfo(ctx, "🚀 Ingest batch received", 
		"event_count", len(batch.Events),
		"relay_id", batch.RelayID,
	)

	// Add events to batch processor for performance
	processed := 0
	for i := range batch.Events {
		batch.Events[i].ProcessingInfo.ReceivedAt = time.Now()
		batch.Events[i].ProcessingInfo.ProcessedAt = time.Now()
		batch.Events[i].ProcessingInfo.Relay = batch.RelayID
		
		// Add trace ID to event metadata
		if batch.Events[i].Metadata == nil {
			batch.Events[i].Metadata = make(map[string]interface{})
		}
		batch.Events[i].Metadata["trace_id"] = traceID
		
		s.batchProcessor.Add(&batch.Events[i])
		processed++
	}

	// Update system stats
	stats, err := s.store.GetSystemStats()
	if err != nil {
		s.logger.PipelineWarn(ctx, "Failed to get system stats", "error", err)
		stats = &storage.SystemStats{
			StartTime: s.StartTime,
		}
	}

	stats.EventCount += processed
	stats.LastIngestTime = time.Now()
	
	if err := s.store.UpdateSystemStats(stats); err != nil {
		s.logger.PipelineError(ctx, "Failed to update system stats", "error", err)
	}

	// Response
	response := map[string]interface{}{
		"batch_id":   batch.BatchID,
		"processed":  processed,
		"errors":     0, // Errors reported asynchronously
		"queued":     true,
		"trace_id":   traceID,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	s.logger.PipelineInfo(ctx, "✅ Ingest batch queued", "processed", processed)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// New batch processing method for performance
func (s *Service) processBatch(events []*storage.SpatialEvent) error {
	if len(events) == 0 {
		return nil
	}
	
	ctx := &logging.PipelineContext{
		Component: "stag-batch-processor",
	}
	
	s.logger.PipelineInfo(ctx, "🔄 Processing event batch", "batch_size", len(events))
	
	processed := 0
	errors := 0
	
	for _, event := range events {
		if err := s.processEvent(event); err != nil {
			eventCtx := &logging.PipelineContext{
				TraceID:   fmt.Sprintf("%v", event.Metadata["trace_id"]),
				EventType: event.EventType,
				ClientID:  event.ClientID,
				StagID:    event.SessionID,
				Component: "stag-batch-processor",
			}
			s.logger.PipelineError(eventCtx, "Event processing failed", "error", err)
			errors++
		} else {
			processed++
		}
	}
	
	s.logger.PipelineInfo(ctx, "✅ Batch processing completed", 
		"processed", processed, 
		"errors", errors,
		"success_rate", fmt.Sprintf("%.1f%%", float64(processed)/float64(len(events))*100),
	)
	
	return nil
}

func (s *Service) processEvent(event *storage.SpatialEvent) error {
	// Map session ID to stag ID
	stagID := event.SessionID
	if stagID == "" {
		stagID = "default"
	}

	// Get or create stag
	stag, err := s.store.GetStag(stagID)
	if err != nil {
		// Create new stag
		stag = &storage.Stag{
			ID:          stagID,
			Name:        fmt.Sprintf("Session %s", stagID),
			Description: fmt.Sprintf("Automatically created from session %s", stagID),
			Anchors:     make(map[string]*storage.Anchor),
			Stats:       storage.StagStats{FirstActivity: time.Now()},
			Metadata:    make(map[string]interface{}),
		}
		
		if err := s.store.CreateStag(stag); err != nil {
			return fmt.Errorf("failed to create stag: %w", err)
		}
		
		s.logger.Info("Created new stag", "stag_id", stagID)
	}

	// Process different event types
	switch event.EventType {
	case "mesh":
		return s.processMeshEvent(stag, event)
	case "pose":
		return s.processPoseEvent(stag, event)
	case "camera":
		return s.processCameraEvent(stag, event)
	case "depth":
		return s.processDepthEvent(stag, event)
	case "pointCloud":
		return s.processPointCloudEvent(stag, event)
	case "lighting":
		return s.processLightingEvent(stag, event)
	default:
		return s.processGenericEvent(stag, event)
	}
}

func (s *Service) processMeshEvent(stag *storage.Stag, event *storage.SpatialEvent) error {
	if event.MeshData == nil {
		return fmt.Errorf("mesh event missing mesh data")
	}

	anchorID := event.MeshData.AnchorID
	if anchorID == "" {
		anchorID = fmt.Sprintf("mesh_%s_%d", event.ClientID, event.FrameNumber)
	}

	return s.processAnchorEvent(stag, anchorID, event)
}

func (s *Service) processPoseEvent(stag *storage.Stag, event *storage.SpatialEvent) error {
	if event.PoseData == nil {
		return fmt.Errorf("pose event missing pose data")
	}

	anchorID := fmt.Sprintf("pose_%s", event.ClientID)
	return s.processAnchorEvent(stag, anchorID, event)
}

func (s *Service) processCameraEvent(stag *storage.Stag, event *storage.SpatialEvent) error {
	if event.CameraData == nil {
		return fmt.Errorf("camera event missing camera data")
	}

	anchorID := fmt.Sprintf("camera_%s", event.ClientID)
	return s.processAnchorEvent(stag, anchorID, event)
}

func (s *Service) processDepthEvent(stag *storage.Stag, event *storage.SpatialEvent) error {
	if event.DepthData == nil {
		return fmt.Errorf("depth event missing depth data")
	}

	anchorID := fmt.Sprintf("depth_%s", event.ClientID)
	return s.processAnchorEvent(stag, anchorID, event)
}

func (s *Service) processPointCloudEvent(stag *storage.Stag, event *storage.SpatialEvent) error {
	if event.PointCloudData == nil {
		return fmt.Errorf("pointCloud event missing point cloud data")
	}

	anchorID := fmt.Sprintf("pointcloud_%s_%d", event.ClientID, event.FrameNumber)
	return s.processAnchorEvent(stag, anchorID, event)
}

func (s *Service) processLightingEvent(stag *storage.Stag, event *storage.SpatialEvent) error {
	if event.LightingData == nil {
		return fmt.Errorf("lighting event missing lighting data")
	}

	anchorID := fmt.Sprintf("lighting_%s", event.ClientID)
	return s.processAnchorEvent(stag, anchorID, event)
}

func (s *Service) processGenericEvent(stag *storage.Stag, event *storage.SpatialEvent) error {
	anchorID := fmt.Sprintf("generic_%s_%d", event.ClientID, event.FrameNumber)
	return s.processAnchorEvent(stag, anchorID, event)
}

func (s *Service) processAnchorEvent(stag *storage.Stag, anchorID string, event *storage.SpatialEvent) error {
	// Use optimized hashing for performance
	hasher := performance.GetHasher()
	defer performance.PutHasher(hasher)
	contentHash := hasher.CalculateEventHash(event)
	
	// Create context for logging
	ctx := &logging.PipelineContext{
		TraceID:     fmt.Sprintf("%v", event.Metadata["trace_id"]),
		StagID:      stag.ID,
		AnchorID:    anchorID,
		EventType:   event.EventType,
		ClientID:    event.ClientID,
		SessionID:   event.SessionID,
		FrameNumber: event.FrameNumber,
		Component:   "stag-anchor-processor",
	}
	
	// Get or create anchor
	anchor, err := s.store.GetAnchor(stag.ID, anchorID)
	if err != nil {
		// Create new anchor
		anchor = &storage.Anchor{
			ID:            anchorID,
			StagID:        stag.ID,
			CurrentHash:   contentHash,
			Versions:      []storage.AnchorVersion{},
			LastSessionID: event.SessionID,
			LastClientID:  event.ClientID,
			LastDeviceID:  event.DeviceID,
			Metadata:      make(map[string]interface{}),
		}
		
		if err := s.store.CreateAnchor(anchor); err != nil {
			return fmt.Errorf("failed to create anchor: %w", err)
		}
		
		s.logger.PipelineInfo(ctx, "🆕 Created new anchor")
		
		// Update stag health
		s.updateStagHealth(stag.ID, true, nil)
	}

	// Check if content has changed
	if anchor.CurrentHash == contentHash {
		s.logger.PipelineDebug(ctx, "🔄 Content unchanged, skipping version", "hash", contentHash[:8])
		return nil
	}
	
	// For mesh data, also check geometric signature
	if event.EventType == "mesh" && event.MeshData != nil {
		geomSig := performance.CalculateGeometrySignature(event.MeshData)
		if anchor.Metadata["geom_signature"] == geomSig {
			s.logger.PipelineDebug(ctx, "🔄 Geometry unchanged, skipping version", "geom_sig", geomSig)
			return nil
		}
		anchor.Metadata["geom_signature"] = geomSig
	}

	// Create new version
	version := &storage.AnchorVersion{
		VersionID:      fmt.Sprintf("v%d", time.Now().Unix()),
		Hash:           contentHash,
		Timestamp:      event.Timestamp,
		ChangeType:     "update",
		Transform:      event.Transform,
		MeshData:       event.MeshData,
		PoseData:       event.PoseData,
		CameraData:     event.CameraData,
		DepthData:      event.DepthData,
		PointCloudData: event.PointCloudData,
		LightingData:   event.LightingData,
		EventID:        event.EventID,
		SessionID:      event.SessionID,
		ClientID:       event.ClientID,
		DeviceID:       event.DeviceID,
		FrameNumber:    event.FrameNumber,
		Metadata:       event.Metadata,
	}

	if len(anchor.Versions) == 0 {
		version.ChangeType = "create"
	}

	if err := s.store.AddAnchorVersion(stag.ID, anchorID, version); err != nil {
		return fmt.Errorf("failed to add anchor version: %w", err)
	}

	// Update anchor
	anchor.CurrentHash = contentHash
	anchor.LastSessionID = event.SessionID
	anchor.LastClientID = event.ClientID
	anchor.LastDeviceID = event.DeviceID
	anchor.Versions = append(anchor.Versions, *version)

	if err := s.store.UpdateAnchor(anchor); err != nil {
		return fmt.Errorf("failed to update anchor: %w", err)
	}

	// Update stag stats
	stag.Stats.LastActivity = time.Now()
	stag.Stats.EventCount++
	stag.Stats.VersionCount++

	// Track unique sessions, clients, devices
	sessionKey := fmt.Sprintf("session_%s", event.SessionID)
	clientKey := fmt.Sprintf("client_%s", event.ClientID)
	deviceKey := fmt.Sprintf("device_%s", event.DeviceID)
	
	if stag.Metadata[sessionKey] == nil {
		stag.Metadata[sessionKey] = true
		stag.Stats.SessionCount++
	}
	if stag.Metadata[clientKey] == nil {
		stag.Metadata[clientKey] = true
		stag.Stats.ClientCount++
	}
	if stag.Metadata[deviceKey] == nil {
		stag.Metadata[deviceKey] = true
		stag.Stats.DeviceCount++
	}

	if err := s.store.UpdateStagStats(stag.ID, stag.Stats); err != nil {
		return fmt.Errorf("failed to update stag stats: %w", err)
	}

	s.logger.PipelineInfo(ctx, "✅ Updated anchor", 
		"version_id", version.VersionID,
		"change_type", version.ChangeType,
		"hash", contentHash[:8],
	)
	
	// Update stag health
	s.updateStagHealth(stag.ID, true, nil)

	return nil
}

// Health monitoring methods
func (s *Service) updateStagHealth(stagID string, healthy bool, err error) {
	s.healthChecker.mu.Lock()
	defer s.healthChecker.mu.Unlock()
	
	health, exists := s.healthChecker.stagHealth[stagID]
	if !exists {
		health = &StagHealth{
			ID:           stagID,
			LastActivity: time.Now(),
		}
		s.healthChecker.stagHealth[stagID] = health
	}
	
	health.Healthy = healthy
	health.LastActivity = time.Now()
	health.LastChecked = time.Now()
	
	if err != nil {
		health.ErrorCount++
		health.LastError = err
		health.LastErrorTime = time.Now()
		health.Healthy = false
	}
	
	// Get anchor count
	if stag, stagErr := s.store.GetStag(stagID); stagErr == nil {
		health.AnchorCount = len(stag.Anchors)
	}
}

func (s *Service) startHealthMonitoring() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		s.performHealthCheck()
	}
}

func (s *Service) performHealthCheck() {
	s.healthChecker.mu.Lock()
	defer s.healthChecker.mu.Unlock()
	
	now := time.Now()
	for stagID, health := range s.healthChecker.stagHealth {
		// Check if stag is stale (no activity for 5 minutes)
		if now.Sub(health.LastActivity) > 5*time.Minute {
			health.Healthy = false
		}
		
		// Log health status
		s.logger.LogStagHealth(stagID, health.Healthy, health.AnchorCount, health.LastActivity)
	}
	
	s.healthChecker.lastHealthCheck = now
	
	// Log performance metrics
	s.logger.LogPerformanceMetrics()
}

// Query handlers

func (s *Service) HandleListStags(w http.ResponseWriter, r *http.Request) {
	ctx := &logging.PipelineContext{
		Component: "stag-query",
	}
	
	stopTimer := s.logger.StartTimer()
	defer func() {
		duration := stopTimer()
		s.logger.PipelineDebug(ctx, "List stags query completed", "duration", duration)
	}()
	
	stags, err := s.store.ListStags()
	if err != nil {
		s.logger.PipelineError(ctx, "Failed to list stags", "error", err)
		http.Error(w, "Failed to list stags", http.StatusInternalServerError)
		return
	}

	s.logger.PipelineInfo(ctx, "📋 Listed stags", "count", len(stags))
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stags)
}

func (s *Service) HandleGetStag(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stagID := vars["stag_id"]

	stag, err := s.store.GetStag(stagID)
	if err != nil {
		s.logger.Error("Failed to get stag", "stag_id", stagID, "error", err)
		http.Error(w, "Stag not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stag)
}

func (s *Service) HandleListAnchors(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stagID := vars["stag_id"]

	anchors, err := s.store.ListAnchors(stagID)
	if err != nil {
		s.logger.Error("Failed to list anchors", "stag_id", stagID, "error", err)
		http.Error(w, "Failed to list anchors", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(anchors)
}

func (s *Service) HandleGetAnchor(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stagID := vars["stag_id"]
	anchorID := vars["anchor_id"]

	anchor, err := s.store.GetAnchor(stagID, anchorID)
	if err != nil {
		s.logger.Error("Failed to get anchor", "stag_id", stagID, "anchor_id", anchorID, "error", err)
		http.Error(w, "Anchor not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(anchor)
}

func (s *Service) HandleGetAnchorHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stagID := vars["stag_id"]
	anchorID := vars["anchor_id"]

	// Parse pagination parameters
	offset := 0
	limit := 50
	
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}
	
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	history, err := s.store.GetAnchorHistory(stagID, anchorID, offset, limit)
	if err != nil {
		s.logger.Error("Failed to get anchor history", "stag_id", stagID, "anchor_id", anchorID, "error", err)
		http.Error(w, "Failed to get anchor history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func (s *Service) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetSystemStats()
	if err != nil {
		s.logger.Error("Failed to get system stats", "error", err)
		http.Error(w, "Failed to get system stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Service) HandleGetStagStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stagID := vars["stag_id"]

	stag, err := s.store.GetStag(stagID)
	if err != nil {
		s.logger.Error("Failed to get stag", "stag_id", stagID, "error", err)
		http.Error(w, "Stag not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stag.Stats)
}