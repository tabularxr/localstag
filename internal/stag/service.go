package stag

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/tabular/local-pipeline/internal/logging"
	"github.com/tabular/local-pipeline/internal/storage"
)

type Service struct {
	store     storage.Storage
	logger    *logging.Logger
	StartTime time.Time
}

func NewService(store storage.Storage, logger *logging.Logger) *Service {
	return &Service{
		store:     store,
		logger:    logger,
		StartTime: time.Now(),
	}
}

func (s *Service) HandleIngest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	// Parse request body
	var batch storage.IngestBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		s.logger.Error("Failed to decode ingest batch", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.logger.Info("Received ingest batch", 
		"batch_id", batch.BatchID,
		"event_count", len(batch.Events),
		"relay_id", batch.RelayID,
	)

	// Process events
	processed := 0
	errors := 0
	
	for _, event := range batch.Events {
		if err := s.processEvent(&event); err != nil {
			s.logger.Error("Failed to process event", 
				"event_id", event.EventID,
				"error", err,
			)
			errors++
		} else {
			processed++
		}
	}

	// Update system stats
	stats, err := s.store.GetSystemStats()
	if err != nil {
		s.logger.Error("Failed to get system stats", "error", err)
		stats = &storage.SystemStats{
			StartTime: s.StartTime,
		}
	}

	stats.EventCount += processed
	stats.LastIngestTime = time.Now()
	
	if err := s.store.UpdateSystemStats(stats); err != nil {
		s.logger.Error("Failed to update system stats", "error", err)
	}

	// Response
	processingTime := time.Since(startTime)
	response := map[string]interface{}{
		"batch_id":        batch.BatchID,
		"processed":       processed,
		"errors":          errors,
		"processing_time": processingTime.String(),
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	s.logger.Info("Processed ingest batch",
		"batch_id", batch.BatchID,
		"processed", processed,
		"errors", errors,
		"processing_time", processingTime,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
	// Calculate content hash for change detection
	contentHash := s.calculateEventHash(event)
	
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
		
		s.logger.Info("Created new anchor", 
			"stag_id", stag.ID,
			"anchor_id", anchorID,
			"event_type", event.EventType,
		)
	}

	// Check if content has changed
	if anchor.CurrentHash == contentHash {
		s.logger.Debug("Anchor content unchanged, skipping version",
			"stag_id", stag.ID,
			"anchor_id", anchorID,
			"hash", contentHash,
		)
		return nil
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

	s.logger.Info("Updated anchor", 
		"stag_id", stag.ID,
		"anchor_id", anchorID,
		"event_type", event.EventType,
		"version_id", version.VersionID,
		"change_type", version.ChangeType,
	)

	return nil
}

func (s *Service) calculateEventHash(event *storage.SpatialEvent) string {
	// Create a hash of the event content for change detection
	hasher := md5.New()
	
	// Include relevant fields in hash
	hasher.Write([]byte(event.EventType))
	hasher.Write([]byte(fmt.Sprintf("%d", event.FrameNumber)))
	
	if event.Transform != nil {
		hasher.Write([]byte(fmt.Sprintf("%v", event.Transform)))
	}
	
	if event.MeshData != nil {
		hasher.Write([]byte(fmt.Sprintf("%v", event.MeshData.Vertices)))
		hasher.Write([]byte(fmt.Sprintf("%v", event.MeshData.Faces)))
	}
	
	if event.PoseData != nil {
		hasher.Write([]byte(fmt.Sprintf("%v", event.PoseData.Transform)))
	}
	
	if event.CameraData != nil {
		hasher.Write([]byte(fmt.Sprintf("%d", len(event.CameraData.ImageData))))
		hasher.Write([]byte(fmt.Sprintf("%v", event.CameraData.Intrinsics)))
	}
	
	if event.DepthData != nil {
		hasher.Write([]byte(fmt.Sprintf("%v", event.DepthData.Data)))
	}
	
	if event.PointCloudData != nil {
		hasher.Write([]byte(fmt.Sprintf("%v", event.PointCloudData.Points)))
	}
	
	if event.LightingData != nil {
		hasher.Write([]byte(fmt.Sprintf("%v", event.LightingData.AmbientIntensity)))
	}
	
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// Query handlers

func (s *Service) HandleListStags(w http.ResponseWriter, r *http.Request) {
	stags, err := s.store.ListStags()
	if err != nil {
		s.logger.Error("Failed to list stags", "error", err)
		http.Error(w, "Failed to list stags", http.StatusInternalServerError)
		return
	}

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