package be_indexer

import (
	"sort"

	"github.com/echoface/be_indexer/util"
)

// QueryFieldStat contains metadata used to estimate the cost of evaluating a
// query field. Selectivity must be in (0, 1]; lower values are more selective.
type QueryFieldStat struct {
	Field                BEField
	Cardinality          uint64
	EstimatedSelectivity float64
	PostingCount         uint64
	IsRange              bool
	Weight               float64
}

// QueryPlan describes a heuristic ordering of fields for a query.
type QueryPlan struct {
	Fields   []BEField
	Selected []QueryFieldStat
}

// QueryPlanner chooses an evaluation order for query fields.
type QueryPlanner interface {
	Plan(assigns Assignments) QueryPlan
}

// DefaultQueryPlanner orders fields by estimated selectivity. Without registered
// statistics it falls back to deterministic field-name ordering; it does not claim
// to estimate real posting-list costs in that case.
type DefaultQueryPlanner struct {
	fieldStats map[BEField]QueryFieldStat
}

func NewDefaultQueryPlanner() *DefaultQueryPlanner {
	return &DefaultQueryPlanner{fieldStats: make(map[BEField]QueryFieldStat)}
}

// RegisterFieldStat adds or replaces a planning hint for a field.
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

// IndexStats contains a point-in-time summary of an index runtime.
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

type QueryMetrics struct {
	TotalQueries  uint64
	AvgLatencyMS  float64
	P99LatencyMS  float64
	FailedQueries uint64
}

type RuntimeStats struct {
	IndexStats IndexStats
	Query      QueryMetrics
}

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

// SegmentInfo is metadata for a published immutable index segment.
type SegmentInfo struct {
	ID            string
	SchemaVersion uint64
	BuildVersion  uint64
	DocCount      int
	FieldCount    int
}
