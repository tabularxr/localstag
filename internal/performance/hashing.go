package performance

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash"
	"math"
	"sync"
	
	"github.com/tabular/local-pipeline/internal/storage"
)

// FastHasher provides optimized hashing for spatial data
type FastHasher struct {
	hasher hash.Hash
	mu     sync.Mutex
}

var hasherPool = sync.Pool{
	New: func() interface{} {
		return &FastHasher{
			hasher: md5.New(),
		}
	},
}

func GetHasher() *FastHasher {
	return hasherPool.Get().(*FastHasher)
}

func PutHasher(h *FastHasher) {
	h.hasher.Reset()
	hasherPool.Put(h)
}

func (h *FastHasher) CalculateEventHash(event *storage.SpatialEvent) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.hasher.Reset()
	
	// Hash event type and frame number
	h.hasher.Write([]byte(event.EventType))
	binary.Write(h.hasher, binary.LittleEndian, event.FrameNumber)
	
	// Hash spatial data based on type
	switch event.EventType {
	case "mesh":
		if event.MeshData != nil {
			h.hashMeshData(event.MeshData)
		}
	case "pose":
		if event.PoseData != nil {
			h.hashPoseData(event.PoseData)
		}
	case "camera":
		if event.CameraData != nil {
			h.hashCameraData(event.CameraData)
		}
	case "depth":
		if event.DepthData != nil {
			h.hashDepthData(event.DepthData)
		}
	case "pointCloud":
		if event.PointCloudData != nil {
			h.hashPointCloudData(event.PointCloudData)
		}
	case "lighting":
		if event.LightingData != nil {
			h.hashLightingData(event.LightingData)
		}
	}
	
	if event.Transform != nil {
		h.hashTransform(event.Transform)
	}
	
	return fmt.Sprintf("%x", h.hasher.Sum(nil))
}

// Optimized mesh hashing - samples vertices instead of hashing all
func (h *FastHasher) hashMeshData(mesh *storage.MeshData) {
	// Hash anchor ID
	h.hasher.Write([]byte(mesh.AnchorID))
	
	// Hash vertex count and face count
	binary.Write(h.hasher, binary.LittleEndian, int32(len(mesh.Vertices)))
	binary.Write(h.hasher, binary.LittleEndian, int32(len(mesh.Faces)))
	
	// Sample vertices for large meshes (performance optimization)
	vertexCount := len(mesh.Vertices) / 3
	if vertexCount > 1000 {
		// Sample every 10th vertex for large meshes
		for i := 0; i < len(mesh.Vertices); i += 30 {
			if i+2 < len(mesh.Vertices) {
				binary.Write(h.hasher, binary.LittleEndian, mesh.Vertices[i])
				binary.Write(h.hasher, binary.LittleEndian, mesh.Vertices[i+1])
				binary.Write(h.hasher, binary.LittleEndian, mesh.Vertices[i+2])
			}
		}
	} else {
		// Hash all vertices for small meshes
		for _, v := range mesh.Vertices {
			binary.Write(h.hasher, binary.LittleEndian, v)
		}
	}
	
	// Hash face indices (sample for large meshes)
	faceCount := len(mesh.Faces)
	if faceCount > 1000 {
		// Sample every 10th face for large meshes
		for i := 0; i < len(mesh.Faces); i += 10 {
			binary.Write(h.hasher, binary.LittleEndian, mesh.Faces[i])
		}
	} else {
		// Hash all faces for small meshes
		for _, f := range mesh.Faces {
			binary.Write(h.hasher, binary.LittleEndian, f)
		}
	}
	
	// Hash classification and confidence
	h.hasher.Write([]byte(mesh.Classification))
	binary.Write(h.hasher, binary.LittleEndian, mesh.Confidence)
}

func (h *FastHasher) hashPoseData(pose *storage.PoseData) {
	if pose.Transform != nil {
		h.hashTransform(pose.Transform)
	}
	for _, v := range pose.Velocity {
		binary.Write(h.hasher, binary.LittleEndian, v)
	}
	for _, a := range pose.Acceleration {
		binary.Write(h.hasher, binary.LittleEndian, a)
	}
	binary.Write(h.hasher, binary.LittleEndian, pose.Confidence)
}

func (h *FastHasher) hashCameraData(camera *storage.CameraData) {
	// Hash image dimensions and format
	binary.Write(h.hasher, binary.LittleEndian, int32(camera.Width))
	binary.Write(h.hasher, binary.LittleEndian, int32(camera.Height))
	h.hasher.Write([]byte(camera.Format))
	
	// Hash image data length (not content for performance)
	binary.Write(h.hasher, binary.LittleEndian, int32(len(camera.ImageData)))
	
	// Hash intrinsics
	for _, v := range camera.Intrinsics {
		binary.Write(h.hasher, binary.LittleEndian, v)
	}
	
	// Hash transform if present
	if camera.Transform != nil {
		h.hashTransform(camera.Transform)
	}
}

func (h *FastHasher) hashDepthData(depth *storage.DepthData) {
	binary.Write(h.hasher, binary.LittleEndian, int32(depth.Width))
	binary.Write(h.hasher, binary.LittleEndian, int32(depth.Height))
	binary.Write(h.hasher, binary.LittleEndian, int32(len(depth.Data)))
	
	// Sample depth data for performance
	dataLen := len(depth.Data)
	if dataLen > 1000 {
		// Sample every 10th depth value
		for i := 0; i < dataLen; i += 10 {
			binary.Write(h.hasher, binary.LittleEndian, depth.Data[i])
		}
	} else {
		for _, v := range depth.Data {
			binary.Write(h.hasher, binary.LittleEndian, v)
		}
	}
}

func (h *FastHasher) hashPointCloudData(pc *storage.PointCloudData) {
	binary.Write(h.hasher, binary.LittleEndian, int32(len(pc.Points)))
	
	// Sample point cloud data for performance
	pointCount := len(pc.Points) / 3
	if pointCount > 1000 {
		// Sample every 10th point
		for i := 0; i < len(pc.Points); i += 30 {
			if i+2 < len(pc.Points) {
				binary.Write(h.hasher, binary.LittleEndian, pc.Points[i])
				binary.Write(h.hasher, binary.LittleEndian, pc.Points[i+1])
				binary.Write(h.hasher, binary.LittleEndian, pc.Points[i+2])
			}
		}
	} else {
		for _, v := range pc.Points {
			binary.Write(h.hasher, binary.LittleEndian, v)
		}
	}
}

func (h *FastHasher) hashLightingData(lighting *storage.LightingData) {
	binary.Write(h.hasher, binary.LittleEndian, lighting.AmbientIntensity)
	for _, v := range lighting.DirectionalLight {
		binary.Write(h.hasher, binary.LittleEndian, v)
	}
	for _, v := range lighting.SphericalHarmonics {
		binary.Write(h.hasher, binary.LittleEndian, v)
	}
	binary.Write(h.hasher, binary.LittleEndian, lighting.ColorTemperature)
}

func (h *FastHasher) hashTransform(transform *storage.Transform) {
	for _, v := range transform.Translation {
		binary.Write(h.hasher, binary.LittleEndian, v)
	}
	for _, v := range transform.Rotation {
		binary.Write(h.hasher, binary.LittleEndian, v)
	}
	for _, v := range transform.Scale {
		binary.Write(h.hasher, binary.LittleEndian, v)
	}
}

// Geometry-aware change detection for mesh data
func CalculateGeometrySignature(mesh *storage.MeshData) string {
	if mesh == nil || len(mesh.Vertices) == 0 {
		return "empty"
	}
	
	// Calculate geometric properties
	vertexCount := len(mesh.Vertices) / 3
	faceCount := len(mesh.Faces)
	
	// Calculate bounding box
	var minX, minY, minZ, maxX, maxY, maxZ float64
	if len(mesh.Vertices) >= 3 {
		minX, minY, minZ = mesh.Vertices[0], mesh.Vertices[1], mesh.Vertices[2]
		maxX, maxY, maxZ = minX, minY, minZ
		
		for i := 0; i < len(mesh.Vertices); i += 3 {
			if i+2 < len(mesh.Vertices) {
				x, y, z := mesh.Vertices[i], mesh.Vertices[i+1], mesh.Vertices[i+2]
				minX = math.Min(minX, x)
				minY = math.Min(minY, y)
				minZ = math.Min(minZ, z)
				maxX = math.Max(maxX, x)
				maxY = math.Max(maxY, y)
				maxZ = math.Max(maxZ, z)
			}
		}
	}
	
	// Calculate volume
	volume := (maxX - minX) * (maxY - minY) * (maxZ - minZ)
	
	return fmt.Sprintf("%s_%d_%d_%.3f", mesh.AnchorID, vertexCount, faceCount, volume)
}