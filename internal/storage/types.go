package storage

import (
	"time"
)

// Core data structures for spatial data storage

type Transform struct {
	Translation [3]float64 `json:"translation"`
	Rotation    [4]float64 `json:"rotation"`    // Quaternion [x, y, z, w]
	Scale       [3]float64 `json:"scale"`
}

type MeshData struct {
	AnchorID      string      `json:"anchor_id"`
	Vertices      []float64   `json:"vertices"`
	Faces         []uint32    `json:"faces"`
	Normals       []float64   `json:"normals,omitempty"`
	Colors        []float64   `json:"colors,omitempty"`
	TextureCoords []float64   `json:"texture_coords,omitempty"`
	Transform     *Transform  `json:"transform,omitempty"`
	Classification string     `json:"classification,omitempty"`
	Confidence    float64     `json:"confidence,omitempty"`
}

type PoseData struct {
	Transform    *Transform `json:"transform"`
	Velocity     [3]float64 `json:"velocity,omitempty"`
	Acceleration [3]float64 `json:"acceleration,omitempty"`
	AngularVelocity [3]float64 `json:"angular_velocity,omitempty"`
	Confidence   float64    `json:"confidence,omitempty"`
}

type CameraData struct {
	ImageData     []byte      `json:"image_data"`
	Width         int         `json:"width"`
	Height        int         `json:"height"`
	Format        string      `json:"format"`
	Intrinsics    [9]float64  `json:"intrinsics"`
	Distortion    []float64   `json:"distortion,omitempty"`
	Transform     *Transform  `json:"transform,omitempty"`
	Timestamp     time.Time   `json:"timestamp"`
	Exposure      float64     `json:"exposure,omitempty"`
	ISO           int         `json:"iso,omitempty"`
	FocalLength   float64     `json:"focal_length,omitempty"`
}

type DepthData struct {
	Data          []float64   `json:"data"`
	Width         int         `json:"width"`
	Height        int         `json:"height"`
	Confidence    []float64   `json:"confidence,omitempty"`
	Transform     *Transform  `json:"transform,omitempty"`
	MinRange      float64     `json:"min_range,omitempty"`
	MaxRange      float64     `json:"max_range,omitempty"`
	Timestamp     time.Time   `json:"timestamp"`
}

type PointCloudData struct {
	Points        []float64   `json:"points"`       // [x, y, z, x, y, z, ...]
	Colors        []float64   `json:"colors,omitempty"`    // [r, g, b, r, g, b, ...]
	Normals       []float64   `json:"normals,omitempty"`   // [x, y, z, x, y, z, ...]
	Confidence    []float64   `json:"confidence,omitempty"`
	Transform     *Transform  `json:"transform,omitempty"`
	Timestamp     time.Time   `json:"timestamp"`
}

type LightingData struct {
	AmbientIntensity    float64     `json:"ambient_intensity"`
	DirectionalLight    [3]float64  `json:"directional_light"`
	SphericalHarmonics  []float64   `json:"spherical_harmonics,omitempty"`
	ColorTemperature    float64     `json:"color_temperature,omitempty"`
	Transform           *Transform  `json:"transform,omitempty"`
	Timestamp           time.Time   `json:"timestamp"`
}

type ProcessingInfo struct {
	ReceivedAt      time.Time              `json:"received_at"`
	ProcessedAt     time.Time              `json:"processed_at"`
	ProcessingTime  time.Duration          `json:"processing_time"`
	Relay           string                 `json:"relay"`
	Compressed      bool                   `json:"compressed"`
	CompressionType string                 `json:"compression_type,omitempty"`
	OriginalSize    int                    `json:"original_size,omitempty"`
	CompressedSize  int                    `json:"compressed_size,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type SpatialEvent struct {
	EventID       string                 `json:"event_id"`
	EventType     string                 `json:"event_type"`
	Timestamp     time.Time              `json:"timestamp"`
	ServerTime    time.Time              `json:"server_timestamp"`
	
	SessionID     string                 `json:"session_id"`
	ClientID      string                 `json:"client_id"`
	DeviceID      string                 `json:"device_id"`
	FrameNumber   uint64                 `json:"frame_number"`
	
	Transform     *Transform             `json:"transform,omitempty"`
	PoseData      *PoseData              `json:"pose,omitempty"`
	MeshData      *MeshData              `json:"mesh_data,omitempty"`
	CameraData    *CameraData            `json:"camera_data,omitempty"`
	DepthData     *DepthData             `json:"depth_data,omitempty"`
	PointCloudData *PointCloudData       `json:"point_cloud_data,omitempty"`
	LightingData  *LightingData          `json:"lighting_data,omitempty"`
	
	Metadata      map[string]interface{} `json:"metadata"`
	ProcessingInfo ProcessingInfo        `json:"processing_info"`
}

type AnchorVersion struct {
	VersionID     string                 `json:"version_id"`
	Hash          string                 `json:"hash"`
	Timestamp     time.Time              `json:"timestamp"`
	ChangeType    string                 `json:"change_type"` // "create", "update", "delete"
	
	Transform     *Transform             `json:"transform,omitempty"`
	MeshData      *MeshData              `json:"mesh_data,omitempty"`
	PoseData      *PoseData              `json:"pose_data,omitempty"`
	CameraData    *CameraData            `json:"camera_data,omitempty"`
	DepthData     *DepthData             `json:"depth_data,omitempty"`
	PointCloudData *PointCloudData       `json:"point_cloud_data,omitempty"`
	LightingData  *LightingData          `json:"lighting_data,omitempty"`
	
	EventID       string                 `json:"event_id"`
	SessionID     string                 `json:"session_id"`
	ClientID      string                 `json:"client_id"`
	DeviceID      string                 `json:"device_id"`
	FrameNumber   uint64                 `json:"frame_number"`
	
	Metadata      map[string]interface{} `json:"metadata"`
}

type Anchor struct {
	ID            string                 `json:"id"`
	StagID        string                 `json:"stag_id"`
	CurrentHash   string                 `json:"current_hash"`
	Versions      []AnchorVersion        `json:"versions"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	LastSessionID string                 `json:"last_session_id"`
	LastClientID  string                 `json:"last_client_id"`
	LastDeviceID  string                 `json:"last_device_id"`
	Metadata      map[string]interface{} `json:"metadata"`
}

type StagStats struct {
	AnchorCount    int       `json:"anchor_count"`
	VersionCount   int       `json:"version_count"`
	EventCount     int       `json:"event_count"`
	SessionCount   int       `json:"session_count"`
	ClientCount    int       `json:"client_count"`
	DeviceCount    int       `json:"device_count"`
	LastActivity   time.Time `json:"last_activity"`
	FirstActivity  time.Time `json:"first_activity"`
	DataSize       int64     `json:"data_size"`
}

type Stag struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Anchors     map[string]*Anchor     `json:"anchors"`
	Stats       StagStats              `json:"stats"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type SystemStats struct {
	StartTime      time.Time `json:"start_time"`
	StagCount      int       `json:"stag_count"`
	AnchorCount    int       `json:"anchor_count"`
	VersionCount   int       `json:"version_count"`
	EventCount     int       `json:"event_count"`
	LastIngestTime time.Time `json:"last_ingest_time"`
	DatabaseSize   int64     `json:"database_size"`
}

type IngestBatch struct {
	BatchID       string         `json:"batch_id"`
	Events        []SpatialEvent `json:"events"`
	Timestamp     time.Time      `json:"timestamp"`
	RelayID       string         `json:"relay_id"`
	ProcessingInfo ProcessingInfo `json:"processing_info"`
}

type SpatialGraph struct {
	StagID    string             `json:"stag_id"`
	Anchors   map[string]*Anchor `json:"anchors"`
	Timestamp time.Time          `json:"timestamp"`
	Stats     StagStats          `json:"stats"`
}

type AnchorHistory struct {
	AnchorID  string          `json:"anchor_id"`
	StagID    string          `json:"stag_id"`
	Versions  []AnchorVersion `json:"versions"`
	Total     int             `json:"total"`
	Offset    int             `json:"offset"`
	Limit     int             `json:"limit"`
}