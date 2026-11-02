package be_indexer

import (
	"fmt"
	"sync"
	"time"
)

// SegmentSnapshot describes the immutable published state of a predicate engine.
// It is intentionally lightweight and can be extended for segment persistence,
// compaction, or remote snapshot propagation.
type SegmentSnapshot struct {
	ID            string
	SchemaVersion uint64
	BuildVersion  uint64
	DocCount      int
	FieldCount    int
	PublishedAt   time.Time
	Valid         bool
}

// SegmentSnapshotStore is the minimal storage abstraction used by a production
// runtime. A concrete implementation can persist snapshots to memory, files,
// Redis, or a remote storage backend.
type SegmentSnapshotStore interface {
	LoadLatest() (*SegmentSnapshot, error)
	Save(snapshot *SegmentSnapshot) error
	Clear() error
}

// DefaultSegmentSnapshotStore is an in-memory snapshot store used for Phase 2
// runtime scaffolding. It keeps the implementation simple and safe while the
// actual storage backend is still being designed.
type DefaultSegmentSnapshotStore struct {
	mu       sync.RWMutex
	snapshot *SegmentSnapshot
}

func NewDefaultSegmentSnapshotStore() *DefaultSegmentSnapshotStore {
	return &DefaultSegmentSnapshotStore{}
}

func (s *DefaultSegmentSnapshotStore) LoadLatest() (*SegmentSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot == nil {
		return nil, fmt.Errorf("no segment snapshot available")
	}
	copySnap := *s.snapshot
	return &copySnap, nil
}

func (s *DefaultSegmentSnapshotStore) Save(snapshot *SegmentSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("nil snapshot")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = snapshot
	return nil
}

func (s *DefaultSegmentSnapshotStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = nil
	return nil
}

// SegmentRuntimeState is the minimal runtime state required to manage snapshot
// publication and invalidation.
type SegmentRuntimeState struct {
	mu       sync.RWMutex
	snapshot *SegmentSnapshot
	store    SegmentSnapshotStore
}

// NewSegmentRuntimeState creates a production-friendly runtime state object used
// by the next phase of the refactor.
func NewSegmentRuntimeState(store SegmentSnapshotStore) *SegmentRuntimeState {
	if store == nil {
		store = NewDefaultSegmentSnapshotStore()
	}
	return &SegmentRuntimeState{store: store}
}

// CurrentSegment returns the current published segment metadata.
func (s *SegmentRuntimeState) CurrentSegment() SegmentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot == nil {
		return SegmentInfo{ID: "unpublished", SchemaVersion: 0, BuildVersion: 0, DocCount: 0, FieldCount: 0}
	}
	return SegmentInfo{
		ID:            s.snapshot.ID,
		SchemaVersion: s.snapshot.SchemaVersion,
		BuildVersion:  s.snapshot.BuildVersion,
		DocCount:      s.snapshot.DocCount,
		FieldCount:    s.snapshot.FieldCount,
	}
}

// Refresh loads the latest snapshot from the store and updates the in-memory
// runtime state without requiring a full rebuild.
func (s *SegmentRuntimeState) Refresh() error {
	if s.store == nil {
		return fmt.Errorf("segment snapshot store is nil")
	}
	latest, err := s.store.LoadLatest()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = latest
	return nil
}

// Publish writes a new snapshot and updates the runtime state.
func (s *SegmentRuntimeState) Publish(info SegmentInfo) error {
	if s.store == nil {
		return fmt.Errorf("segment snapshot store is nil")
	}
	snapshot := &SegmentSnapshot{
		ID:            info.ID,
		SchemaVersion: info.SchemaVersion,
		BuildVersion:  info.BuildVersion,
		DocCount:      info.DocCount,
		FieldCount:    info.FieldCount,
		PublishedAt:   time.Now().UTC(),
		Valid:         true,
	}
	if err := s.store.Save(snapshot); err != nil {
		return err
	}
	s.mu.Lock()
	s.snapshot = snapshot
	s.mu.Unlock()
	return nil
}

// Invalidate marks the current snapshot as invalid and clears the store.
func (s *SegmentRuntimeState) Invalidate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store != nil {
		if err := s.store.Clear(); err != nil {
			return err
		}
	}
	s.snapshot = nil
	return nil
}

// Version returns the currently published build version.
func (s *SegmentRuntimeState) Version() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot == nil {
		return 0
	}
	return s.snapshot.BuildVersion
}

// SegmentRuntime is an extension of the earlier runtime scaffolding and gives the
// next phase a concrete, testable runtime object that can back a snapshot-based
// production system.
type SegmentRuntime interface {
	CurrentSegment() SegmentInfo
	Publish() error
	Invalidate() error
	Version() uint64
}

// DefaultSegmentRuntime is the concrete implementation used by the next phase of
// the refactor. It is intentionally small and safe to evolve.
type DefaultSegmentRuntime struct {
	state *SegmentRuntimeState
	info  SegmentInfo
}

func NewDefaultSegmentRuntime(store SegmentSnapshotStore) *DefaultSegmentRuntime {
	return &DefaultSegmentRuntime{
		state: NewSegmentRuntimeState(store),
	}
}

func (r *DefaultSegmentRuntime) CurrentSegment() SegmentInfo {
	return r.state.CurrentSegment()
}

func (r *DefaultSegmentRuntime) Publish() error {
	return r.state.Publish(r.info)
}

func (r *DefaultSegmentRuntime) Invalidate() error {
	return r.state.Invalidate()
}

func (r *DefaultSegmentRuntime) Version() uint64 {
	return r.state.Version()
}

// SetInfo sets the next segment metadata before publishing.
func (r *DefaultSegmentRuntime) SetInfo(info SegmentInfo) {
	r.info = info
}
