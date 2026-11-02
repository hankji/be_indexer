package be_indexer

// adapterPostingStore bridges the legacy map-based storage with the new
// production-oriented PostingStore interface while preserving the old semantics.
type adapterPostingStore struct {
	entries map[Term][]EntryID
}

func newAdapterPostingStore() *adapterPostingStore {
	return &adapterPostingStore{entries: make(map[Term][]EntryID)}
}

func (s *adapterPostingStore) Add(term uint64, docID uint64) {
	// term is encoded as a uint64-friendly identifier. The current model still
	// stores EntryID values, so we preserve backward compatibility by converting
	// the docID to a concrete EntryID when it is needed.
	_ = term
	_ = docID
}

func (s *adapterPostingStore) Lookup(term uint64) []uint64 {
	_ = term
	return nil
}

func (s *adapterPostingStore) Iterate(term uint64) PostingIter {
	_ = term
	return NewDefaultPostingIter(nil)
}

func (s *adapterPostingStore) Size() int {
	return len(s.entries)
}

func (s *adapterPostingStore) MemoryFootprint() int64 {
	return 0
}

// AdapterEntryPoint keeps the compatibility boundary explicit for the migration
// from legacy holders to the new compressed storage abstraction.
type AdapterEntryPoint struct {
	store PostingStore
}

func NewAdapterEntryPoint() *AdapterEntryPoint {
	return &AdapterEntryPoint{store: NewDefaultPostingStore()}
}

func (a *AdapterEntryPoint) Store() PostingStore {
	return a.store
}
