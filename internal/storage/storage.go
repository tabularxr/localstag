package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

const (
	StagsBucket    = "stags"
	AnchorsBucket  = "anchors"
	VersionsBucket = "versions"
	StatsBucket    = "stats"
	SessionsBucket = "sessions"
)

type Storage interface {
	// Stag operations
	CreateStag(stag *Stag) error
	GetStag(stagID string) (*Stag, error)
	UpdateStag(stag *Stag) error
	UpdateStagStats(stagID string, stats StagStats) error
	ListStags() ([]*Stag, error)
	DeleteStag(stagID string) error

	// Anchor operations
	CreateAnchor(anchor *Anchor) error
	GetAnchor(stagID, anchorID string) (*Anchor, error)
	UpdateAnchor(anchor *Anchor) error
	ListAnchors(stagID string) ([]*Anchor, error)
	DeleteAnchor(stagID, anchorID string) error

	// Version operations
	AddAnchorVersion(stagID, anchorID string, version *AnchorVersion) error
	GetAnchorVersions(stagID, anchorID string) ([]AnchorVersion, error)
	GetAnchorVersionsWithPaging(stagID, anchorID string, offset, limit int) ([]AnchorVersion, int, error)

	// Query operations
	GetSpatialGraph(stagID string) (*SpatialGraph, error)
	GetAnchorHistory(stagID, anchorID string, offset, limit int) (*AnchorHistory, error)

	// Statistics operations
	GetSystemStats() (*SystemStats, error)
	UpdateSystemStats(stats *SystemStats) error

	// Utility operations
	Close() error
}

type BoltStorage struct {
	db *bbolt.DB
}

func NewBoltStorage(dbPath string) (*BoltStorage, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &BoltStorage{db: db}

	// Initialize buckets
	if err := storage.initBuckets(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return storage, nil
}

func (s *BoltStorage) initBuckets() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		buckets := []string{StagsBucket, AnchorsBucket, VersionsBucket, StatsBucket, SessionsBucket}
		for _, bucket := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
}

func (s *BoltStorage) Close() error {
	return s.db.Close()
}

// Stag operations

func (s *BoltStorage) CreateStag(stag *Stag) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(StagsBucket))
		
		// Check if stag already exists
		if bucket.Get([]byte(stag.ID)) != nil {
			return fmt.Errorf("stag with ID %s already exists", stag.ID)
		}

		// Set timestamps
		now := time.Now()
		stag.CreatedAt = now
		stag.UpdatedAt = now

		// Initialize maps if nil
		if stag.Anchors == nil {
			stag.Anchors = make(map[string]*Anchor)
		}
		if stag.Metadata == nil {
			stag.Metadata = make(map[string]interface{})
		}

		// Serialize and store
		data, err := json.Marshal(stag)
		if err != nil {
			return fmt.Errorf("failed to marshal stag: %w", err)
		}

		return bucket.Put([]byte(stag.ID), data)
	})
}

func (s *BoltStorage) GetStag(stagID string) (*Stag, error) {
	var stag *Stag
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(StagsBucket))
		data := bucket.Get([]byte(stagID))
		if data == nil {
			return fmt.Errorf("stag with ID %s not found", stagID)
		}

		stag = &Stag{}
		return json.Unmarshal(data, stag)
	})
	
	if err != nil {
		return nil, err
	}

	// Load anchors
	anchors, err := s.ListAnchors(stagID)
	if err != nil {
		return nil, fmt.Errorf("failed to load anchors for stag %s: %w", stagID, err)
	}

	stag.Anchors = make(map[string]*Anchor)
	for _, anchor := range anchors {
		stag.Anchors[anchor.ID] = anchor
	}

	return stag, nil
}

func (s *BoltStorage) UpdateStag(stag *Stag) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(StagsBucket))
		
		// Check if stag exists
		if bucket.Get([]byte(stag.ID)) == nil {
			return fmt.Errorf("stag with ID %s does not exist", stag.ID)
		}

		// Update timestamp
		stag.UpdatedAt = time.Now()

		// Serialize and store
		data, err := json.Marshal(stag)
		if err != nil {
			return fmt.Errorf("failed to marshal stag: %w", err)
		}

		return bucket.Put([]byte(stag.ID), data)
	})
}

func (s *BoltStorage) UpdateStagStats(stagID string, stats StagStats) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(StagsBucket))
		
		data := bucket.Get([]byte(stagID))
		if data == nil {
			return fmt.Errorf("stag with ID %s not found", stagID)
		}

		var stag Stag
		if err := json.Unmarshal(data, &stag); err != nil {
			return fmt.Errorf("failed to unmarshal stag: %w", err)
		}

		stag.Stats = stats
		stag.UpdatedAt = time.Now()

		newData, err := json.Marshal(&stag)
		if err != nil {
			return fmt.Errorf("failed to marshal stag: %w", err)
		}

		return bucket.Put([]byte(stagID), newData)
	})
}

func (s *BoltStorage) ListStags() ([]*Stag, error) {
	var stags []*Stag
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(StagsBucket))
		return bucket.ForEach(func(k, v []byte) error {
			var stag Stag
			if err := json.Unmarshal(v, &stag); err != nil {
				return fmt.Errorf("failed to unmarshal stag: %w", err)
			}
			stags = append(stags, &stag)
			return nil
		})
	})
	
	if err != nil {
		return nil, err
	}

	// Load anchors for each stag
	for _, stag := range stags {
		anchors, err := s.ListAnchors(stag.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load anchors for stag %s: %w", stag.ID, err)
		}
		
		stag.Anchors = make(map[string]*Anchor)
		for _, anchor := range anchors {
			stag.Anchors[anchor.ID] = anchor
		}
	}

	return stags, nil
}

func (s *BoltStorage) DeleteStag(stagID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		// Delete all anchors first
		anchorsBucket := tx.Bucket([]byte(AnchorsBucket))
		versionsBucket := tx.Bucket([]byte(VersionsBucket))
		
		// Find and delete all anchors for this stag
		anchorPrefix := []byte(stagID + ":")
		c := anchorsBucket.Cursor()
		for k, _ := c.Seek(anchorPrefix); k != nil && len(k) > len(anchorPrefix) && string(k[:len(anchorPrefix)]) == string(anchorPrefix); k, _ = c.Next() {
			// Delete anchor versions
			anchorKey := string(k[len(anchorPrefix):])
			versionPrefix := []byte(stagID + ":" + anchorKey + ":")
			vc := versionsBucket.Cursor()
			for vk, _ := vc.Seek(versionPrefix); vk != nil && len(vk) > len(versionPrefix) && string(vk[:len(versionPrefix)]) == string(versionPrefix); vk, _ = vc.Next() {
				if err := versionsBucket.Delete(vk); err != nil {
					return fmt.Errorf("failed to delete version %s: %w", string(vk), err)
				}
			}
			
			// Delete anchor
			if err := anchorsBucket.Delete(k); err != nil {
				return fmt.Errorf("failed to delete anchor %s: %w", string(k), err)
			}
		}

		// Delete the stag
		stagsBucket := tx.Bucket([]byte(StagsBucket))
		return stagsBucket.Delete([]byte(stagID))
	})
}

// Anchor operations

func (s *BoltStorage) CreateAnchor(anchor *Anchor) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(AnchorsBucket))
		key := []byte(anchor.StagID + ":" + anchor.ID)
		
		// Check if anchor already exists
		if bucket.Get(key) != nil {
			return fmt.Errorf("anchor with ID %s already exists in stag %s", anchor.ID, anchor.StagID)
		}

		// Set timestamps
		now := time.Now()
		anchor.CreatedAt = now
		anchor.UpdatedAt = now

		// Initialize metadata if nil
		if anchor.Metadata == nil {
			anchor.Metadata = make(map[string]interface{})
		}

		// Serialize and store
		data, err := json.Marshal(anchor)
		if err != nil {
			return fmt.Errorf("failed to marshal anchor: %w", err)
		}

		return bucket.Put(key, data)
	})
}

func (s *BoltStorage) GetAnchor(stagID, anchorID string) (*Anchor, error) {
	var anchor *Anchor
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(AnchorsBucket))
		key := []byte(stagID + ":" + anchorID)
		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("anchor with ID %s not found in stag %s", anchorID, stagID)
		}

		anchor = &Anchor{}
		return json.Unmarshal(data, anchor)
	})
	
	if err != nil {
		return nil, err
	}

	// Load versions
	versions, err := s.GetAnchorVersions(stagID, anchorID)
	if err != nil {
		return nil, fmt.Errorf("failed to load versions for anchor %s: %w", anchorID, err)
	}

	anchor.Versions = versions
	return anchor, nil
}

func (s *BoltStorage) UpdateAnchor(anchor *Anchor) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(AnchorsBucket))
		key := []byte(anchor.StagID + ":" + anchor.ID)
		
		// Check if anchor exists
		if bucket.Get(key) == nil {
			return fmt.Errorf("anchor with ID %s does not exist in stag %s", anchor.ID, anchor.StagID)
		}

		// Update timestamp
		anchor.UpdatedAt = time.Now()

		// Serialize and store
		data, err := json.Marshal(anchor)
		if err != nil {
			return fmt.Errorf("failed to marshal anchor: %w", err)
		}

		return bucket.Put(key, data)
	})
}

func (s *BoltStorage) ListAnchors(stagID string) ([]*Anchor, error) {
	var anchors []*Anchor
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(AnchorsBucket))
		prefix := []byte(stagID + ":")
		c := bucket.Cursor()
		
		for k, v := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var anchor Anchor
			if err := json.Unmarshal(v, &anchor); err != nil {
				return fmt.Errorf("failed to unmarshal anchor: %w", err)
			}
			anchors = append(anchors, &anchor)
		}
		return nil
	})
	
	if err != nil {
		return nil, err
	}

	// Load versions for each anchor
	for _, anchor := range anchors {
		versions, err := s.GetAnchorVersions(stagID, anchor.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load versions for anchor %s: %w", anchor.ID, err)
		}
		anchor.Versions = versions
	}

	return anchors, nil
}

func (s *BoltStorage) DeleteAnchor(stagID, anchorID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		// Delete all versions first
		versionsBucket := tx.Bucket([]byte(VersionsBucket))
		versionPrefix := []byte(stagID + ":" + anchorID + ":")
		c := versionsBucket.Cursor()
		
		for k, _ := c.Seek(versionPrefix); k != nil && len(k) > len(versionPrefix) && string(k[:len(versionPrefix)]) == string(versionPrefix); k, _ = c.Next() {
			if err := versionsBucket.Delete(k); err != nil {
				return fmt.Errorf("failed to delete version %s: %w", string(k), err)
			}
		}

		// Delete the anchor
		anchorsBucket := tx.Bucket([]byte(AnchorsBucket))
		anchorKey := []byte(stagID + ":" + anchorID)
		return anchorsBucket.Delete(anchorKey)
	})
}

// Version operations

func (s *BoltStorage) AddAnchorVersion(stagID, anchorID string, version *AnchorVersion) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(VersionsBucket))
		key := []byte(stagID + ":" + anchorID + ":" + version.VersionID)
		
		// Initialize metadata if nil
		if version.Metadata == nil {
			version.Metadata = make(map[string]interface{})
		}

		// Serialize and store
		data, err := json.Marshal(version)
		if err != nil {
			return fmt.Errorf("failed to marshal version: %w", err)
		}

		return bucket.Put(key, data)
	})
}

func (s *BoltStorage) GetAnchorVersions(stagID, anchorID string) ([]AnchorVersion, error) {
	var versions []AnchorVersion
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(VersionsBucket))
		prefix := []byte(stagID + ":" + anchorID + ":")
		c := bucket.Cursor()
		
		for k, v := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var version AnchorVersion
			if err := json.Unmarshal(v, &version); err != nil {
				return fmt.Errorf("failed to unmarshal version: %w", err)
			}
			versions = append(versions, version)
		}
		return nil
	})
	
	return versions, err
}

func (s *BoltStorage) GetAnchorVersionsWithPaging(stagID, anchorID string, offset, limit int) ([]AnchorVersion, int, error) {
	var versions []AnchorVersion
	var total int

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(VersionsBucket))
		prefix := []byte(stagID + ":" + anchorID + ":")
		c := bucket.Cursor()
		
		// Count total versions
		for k, _ := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			total++
		}
		
		// Get paged versions
		current := 0
		for k, v := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			if current < offset {
				current++
				continue
			}
			if len(versions) >= limit {
				break
			}
			
			var version AnchorVersion
			if err := json.Unmarshal(v, &version); err != nil {
				return fmt.Errorf("failed to unmarshal version: %w", err)
			}
			versions = append(versions, version)
			current++
		}
		return nil
	})
	
	return versions, total, err
}

// Query operations

func (s *BoltStorage) GetSpatialGraph(stagID string) (*SpatialGraph, error) {
	stag, err := s.GetStag(stagID)
	if err != nil {
		return nil, err
	}

	return &SpatialGraph{
		StagID:    stagID,
		Anchors:   stag.Anchors,
		Timestamp: time.Now(),
		Stats:     stag.Stats,
	}, nil
}

func (s *BoltStorage) GetAnchorHistory(stagID, anchorID string, offset, limit int) (*AnchorHistory, error) {
	versions, total, err := s.GetAnchorVersionsWithPaging(stagID, anchorID, offset, limit)
	if err != nil {
		return nil, err
	}

	return &AnchorHistory{
		AnchorID: anchorID,
		StagID:   stagID,
		Versions: versions,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
	}, nil
}

// Statistics operations

func (s *BoltStorage) GetSystemStats() (*SystemStats, error) {
	var stats *SystemStats
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(StatsBucket))
		data := bucket.Get([]byte("system"))
		if data == nil {
			// Return default stats if not found
			stats = &SystemStats{
				StartTime:      time.Now(),
				StagCount:      0,
				AnchorCount:    0,
				VersionCount:   0,
				EventCount:     0,
				LastIngestTime: time.Time{},
				DatabaseSize:   0,
			}
			return nil
		}

		stats = &SystemStats{}
		return json.Unmarshal(data, stats)
	})
	
	if err != nil {
		return nil, err
	}

	// Update database size
	dbStat := s.db.Stats()
	stats.DatabaseSize = int64(dbStat.TxStats.PageAlloc)

	return stats, nil
}

func (s *BoltStorage) UpdateSystemStats(stats *SystemStats) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(StatsBucket))
		
		// Update database size
		dbStat := s.db.Stats()
		stats.DatabaseSize = int64(dbStat.TxStats.PageAlloc)

		data, err := json.Marshal(stats)
		if err != nil {
			return fmt.Errorf("failed to marshal system stats: %w", err)
		}

		return bucket.Put([]byte("system"), data)
	})
}