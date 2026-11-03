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

// DefaultQueryPlanner orders fields by estimated selectivity.
type DefaultQueryPlanner struct {
	fieldStats map[BEField]QueryFieldStat
}

func NewDefaultQueryPlanner() *DefaultQueryPlanner {
	return &DefaultQueryPlanner{fieldStats: make(map[BEField]QueryFieldStat)}
}

func (p *DefaultQueryPlanner) RegisterFieldStat(stat QueryFieldStat) {
	if p.fieldStats == nil {
		p.fieldStats = make(map[BEField]QueryFieldStat)
	}
	p.fieldStats[stat.Field] = stat
}

func (p *DefaultQueryPlanner) Plan(assigns Assignments) QueryPlan {
	stats := make([]QueryFieldStat, 0, len(assigns))
	for field, values := range assigns {
		if util.NilInterface(values) {
			continue
		}
		stat, ok := p.fieldStats[field]
		if !ok {
			stat = QueryFieldStat{Field: field, EstimatedSelectivity: 1, Weight: 1}
		}
		stat.Field = field
		if stat.EstimatedSelectivity <= 0 || stat.EstimatedSelectivity > 1 {
			stat.EstimatedSelectivity = 1
		}
		if stat.Weight <= 0 {
			stat.Weight = 1
		}
		stats = append(stats, stat)
	}
	sort.Slice(stats, func(i, j int) bool {
		left := stats[i].EstimatedSelectivity / stats[i].Weight
		right := stats[j].EstimatedSelectivity / stats[j].Weight
		if left == right {
			return stats[i].Field < stats[j].Field
		}
		return left < right
	})

	plan := QueryPlan{Fields: make([]BEField, 0, len(stats)), Selected: stats}
	for _, stat := range stats {
		plan.Fields = append(plan.Fields, stat.Field)
	}
	return plan
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

// ReaderHandle is an abstraction for a snapshot-aware reader.
type ReaderHandle interface {
	Close() error
	SnapshotVersion() uint64
}

type IndexStatsProvider interface {
	GetIndexStats() IndexStats
}

type SnapshotProvider interface {
	GetSnapshotVersion() uint64
}

type ReaderProvider interface {
	AcquireReader() ReaderHandle
}

// SegmentInfo is a lightweight description of an immutable segment.
type SegmentInfo struct {
	ID            string
	SchemaVersion uint64
	BuildVersion  uint64
	DocCount      int
	FieldCount    int
}
