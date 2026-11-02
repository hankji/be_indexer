package be_indexer

import (
	"fmt"
	"sync"
	"time"
)

// SchemaVersion tracks schema compatibility between index segments and the
// runtime model. This is the foundational abstraction used by the next Phase 2
// refactor to ensure any field or rule-schema change results in a version bump.
type SchemaVersion struct {
	Major uint64
	Minor uint64
}

// SegmentManifest is a lightweight manifest describing the exact immutable
// published index snapshot. It is intentionally small and can be persisted to a
// local file, Redis, or remote metadata service in a later production phase.
type SegmentManifest struct {
	SegmentID     string
	SchemaVersion SchemaVersion
	BuildVersion  uint64
	PublishedAt   time.Time
	Valid         bool
}

// PostingIter is a streaming iterator over a posting list. It is intentionally
// tiny so it can support a later compressed postings implementation without
// forcing callers to materialize a full slice.
type PostingIter interface {
	Next() (uint64, bool)
}

// PostingStore abstracts the storage layer for term-to-docID posting lists. The
// initial implementation keeps the same semantics as the old map-based holder but
// exposes a clean boundary for replacing the storage layer with a compressed or
// bitmap-backed representation.
type PostingStore interface {
	Add(term uint64, docID uint64)
	Lookup(term uint64) []uint64
	Iterate(term uint64) PostingIter
	Size() int
	MemoryFootprint() int64
}

// DefaultPostingIter is the simplest iterator implementation used by the default
// storage layer. It is safe to replace with a compressed iterator in a later
// phase.
type DefaultPostingIter struct {
	items []uint64
	idx   int
}

func NewDefaultPostingIter(items []uint64) *DefaultPostingIter {
	cloned := append([]uint64(nil), items...)
	return &DefaultPostingIter{items: cloned}
}

func (it *DefaultPostingIter) Next() (uint64, bool) {
	if it == nil || it.idx >= len(it.items) {
		return 0, false
	}
	v := it.items[it.idx]
	it.idx++
	return v, true
}

// DefaultPostingStore keeps the current semantics and acts as a compatibility
// adapter while the storage backend is being migrated to compressed postings.
type DefaultPostingStore struct {
	mu      sync.RWMutex
	postings map[uint64][]uint64
}

func NewDefaultPostingStore() *DefaultPostingStore {
	return &DefaultPostingStore{postings: make(map[uint64][]uint64)}
}

func (s *DefaultPostingStore) Add(term uint64, docID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.postings[term]; !ok {
		s.postings[term] = make([]uint64, 0, 8)
	}
	list := s.postings[term]
	for _, id := range list {
		if id == docID {
			return
		}
	}
	s.postings[term] = append(list, docID)
}

func (s *DefaultPostingStore) Lookup(term uint64) []uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list, ok := s.postings[term]
	if !ok {
		return nil
	}
	out := make([]uint64, len(list))
	copy(out, list)
	return out
}

func (s *DefaultPostingStore) Iterate(term uint64) PostingIter {
	return NewDefaultPostingIter(s.Lookup(term))
}

func (s *DefaultPostingStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.postings)
}

func (s *DefaultPostingStore) MemoryFootprint() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var total int64
	for _, list := range s.postings {
		total += int64(len(list)) * 8
	}
	return total
}

// VersionedSegmentRuntime is the first concrete runtime object that binds the
// runtime state to a manifest-aware, schema-versioned snapshot phase.
type VersionedSegmentRuntime struct {
	manifest *SegmentManifest
	store    SegmentSnapshotStore
	mu       sync.RWMutex
}

func NewVersionedSegmentRuntime(store SegmentSnapshotStore) *VersionedSegmentRuntime {
	if store == nil {
		store = NewDefaultSegmentSnapshotStore()
	}
	return &VersionedSegmentRuntime{store: store}
}

func (r *VersionedSegmentRuntime) CurrentManifest() SegmentManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.manifest == nil {
		return SegmentManifest{SegmentID: "unpublished", Valid: false}
	}
	copyManifest := *r.manifest
	return copyManifest
}

func (r *VersionedSegmentRuntime) PublishManifest(manifest SegmentManifest) error {
	if r.store == nil {
		return fmt.Errorf("segment snapshot store is nil")
	}
	if manifest.SegmentID == "" {
		return fmt.Errorf("segment id is empty")
	}
	manifest.Valid = true
	if manifest.PublishedAt.IsZero() {
		manifest.PublishedAt = time.Now().UTC()
	}
	if err := r.store.Save(&SegmentSnapshot{
		ID:            manifest.SegmentID,
		SchemaVersion: manifest.SchemaVersion.Major,
		BuildVersion:  manifest.BuildVersion,
		DocCount:      0,
		FieldCount:    0,
		PublishedAt:   manifest.PublishedAt,
		Valid:         true,
	}); err != nil {
		return err
	}
	r.mu.Lock()
	r.manifest = &manifest
	r.mu.Unlock()
	return nil
}

func (r *VersionedSegmentRuntime) Invalidate() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.store != nil {
		if err := r.store.Clear(); err != nil {
			return err
		}
	}
	r.manifest = nil
	return nil
}

func (r *VersionedSegmentRuntime) CurrentVersion() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.manifest == nil {
		return 0
	}
	return r.manifest.BuildVersion
}
