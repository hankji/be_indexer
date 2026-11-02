package be_indexer

import (
	"sort"

	"github.com/echoface/be_indexer/util"
)

// QueryFieldStat defines a field-specific planning hint for the cost-based
// query planner. These values are intentionally lightweight so they can be
// computed from metadata without touching the full postings during the hot path.
type QueryFieldStat struct {
	Field BEField

	// Cardinality is an approximate number of distinct values stored in this field.
	Cardinality uint64

	// EstimatedSelectivity is a normalized value in [0, 1]. Smaller values mean
	// more selective fields, which should be evaluated earlier.
	EstimatedSelectivity float64

	// PostingCount is the approximate number of posting list entries for this field.
	PostingCount uint64

	// IsRange marks whether the query uses range semantics on the field.
	IsRange bool

	// Weight is a planner weight used to prioritize earlier filtering.
	Weight float64
}

// QueryPlan describes the ordered field execution strategy for a query.
type QueryPlan struct {
	Fields   []BEField
	Selected []QueryFieldStat
}

// QueryPlanner chooses the execution order of fields during retrieval. This is a
// small production-oriented abstraction that does not break existing callers.
type QueryPlanner interface {
	Plan(assigns Assignments) QueryPlan
}

// DefaultQueryPlanner is the Phase 1 planner implementation.
//
// The implementation is intentionally heuristic-based: it prefers fields with the
// smallest selectivity (fewest candidates) to reduce the candidate set quickly.
type DefaultQueryPlanner struct {
	fieldStats map[BEField]QueryFieldStat
}

// NewDefaultQueryPlanner creates a planner instance that can be embedded into a
// production runtime without changing the existing BEIndex API.
func NewDefaultQueryPlanner() *DefaultQueryPlanner {
	return &DefaultQueryPlanner{
		fieldStats: make(map[BEField]QueryFieldStat),
	}
}

// RegisterFieldStat registers a static metadata hint for a field. Runtime code
// can update these values when cardinality becomes known, for example after a
// segment is built or a field-level stats snapshot is loaded.
func (p *DefaultQueryPlanner) RegisterFieldStat(stat QueryFieldStat) {
	if p.fieldStats == nil {
		p.fieldStats = make(map[BEField]QueryFieldStat)
	}
	p.fieldStats[stat.Field] = stat
}

// Plan returns the ordered field list that should be evaluated first.
func (p *DefaultQueryPlanner) Plan(assigns Assignments) QueryPlan {
	if len(assigns) == 0 {
		return QueryPlan{}
	}

	stats := make([]QueryFieldStat, 0, len(assigns))
	for field, values := range assigns {
		if util.NilInterface(values) {
			continue
		}

		stat := QueryFieldStat{
			Field:                 field,
			Cardinality:           1,
			EstimatedSelectivity: 1.0,
			PostingCount:          1,
			IsRange:              false,
			Weight:               1.0,
		}
		if cached, ok := p.fieldStats[field]; ok {
			stat = cached
		}
		if stat.EstimatedSelectivity <= 0 {
			stat.EstimatedSelectivity = 1.0
		}
		if stat.Weight <= 0 {
			stat.Weight = 1.0
		}
		stats = append(stats, stat)
	}
	if len(stats) == 0 {
		return QueryPlan{}
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].EstimatedSelectivity == stats[j].EstimatedSelectivity {
			return stats[i].Field < stats[j].Field
		}
		return stats[i].EstimatedSelectivity < stats[j].EstimatedSelectivity
	})

	result := QueryPlan{
		Fields:   make([]BEField, 0, len(stats)),
		Selected: stats,
	}
	for _, stat := range stats {
		result.Fields = append(result.Fields, stat.Field)
	}
	return result
}

// IndexStats are production-oriented runtime metrics for a predicate engine.
type IndexStats struct {
	Version        uint64
	DocCount       uint64
	FieldCount     int
	WildcardCount  int
	QueryCount     uint64
	AvgLatencyMS   float64
	P99LatencyMS   float64
	MemoryUsageMB  float64
	SegmentVersion uint64
}

// QueryMetrics records snapshot-style metrics for query execution.
type QueryMetrics struct {
	TotalQueries  uint64
	AvgLatencyMS  float64
	P99LatencyMS  float64
	FailedQueries uint64
}

// RuntimeStats is a general runtime aggregation for a predicate engine.
type RuntimeStats struct {
	IndexStats IndexStats
	Query      QueryMetrics
}

// ReaderHandle is an abstraction for a snapshot-aware reader. It is intentionally
// small and safe to introduce without breaking the existing BEIndex API.
type ReaderHandle interface {
	Close() error
	SnapshotVersion() uint64
}

// IndexStatsProvider is an optional interface for runtime components that expose
// live stats.
type IndexStatsProvider interface {
	GetIndexStats() IndexStats
}

// SnapshotProvider is an optional interface for index implementations that can
// report the current published snapshot version.
type SnapshotProvider interface {
	GetSnapshotVersion() uint64
}

// ReaderProvider exposes runtime-safe access to a reader handle when a concrete
// implementation can support concurrent snapshot reads.
type ReaderProvider interface {
	AcquireReader() ReaderHandle
}

// SegmentInfo is a lightweight description of an immutable segment that can be
// reused by a segment-based production runtime.
type SegmentInfo struct {
	ID            string
	SchemaVersion uint64
	BuildVersion  uint64
	DocCount      int
	FieldCount    int
}

// SegmentRuntime describes the production runtime model for the next phase.
// It deliberately stays separate from BEIndex so the existing API remains intact.
type SegmentRuntime interface {
	CurrentSegment() SegmentInfo
	Publish() error
	Invalidate() error
}
