# Phase 1 PR Ready Plan

## Goal

This Phase 1 change introduces production-oriented runtime primitives without breaking the public `BEIndex` API. The objective is to prepare the predicate-matching engine for future segment-based, compressed, and planner-driven optimizations.

## Scope

The change intentionally focuses on the following areas:

- query planning metadata
- runtime stats and snapshot awareness
- retrieval compatibility layer for production execution planning
- benchmark scaffolding for hot-path validation

## Included changes

### 1. Query planner abstraction

A new `QueryPlanner` abstraction and `DefaultQueryPlanner` have been introduced in `phase1_runtime.go`.

Key responsibilities:

- estimate per-field selectivity
- choose a cost-aware field evaluation order
- expose a clean hook for later optimizations without breaking retrieval semantics

### 2. Runtime metrics and snapshot metadata

The Phase 1 runtime layer adds lightweight interfaces for production observability:

- `IndexStats`
- `QueryMetrics`
- `RuntimeStats`
- `ReaderHandle`
- `ReaderProvider`
- `IndexStatsProvider`
- `SnapshotProvider`
- `SegmentInfo`

These do not change the old API; they add optional runtime capabilities for future service integration.

### 3. Retrieval compatibility hook

A planner-aware retrieval hook has been added in `planner_runtime_integration.go`.

The hook:

- keeps the public behavior compatible with existing `Retrieve` callers
- allows the runtime to order field evaluation in a production-aware way
- leaves a clear extension point for future cost-based execution and segment-aware reads

### 4. Benchmark scaffolding

The benchmark harness under `benchmark/` is designed to measure:

- planner cost ordering overhead
- retrieval hot-path behavior under representative boolean conditions
- allocation pressure before deeper storage-layer optimization

## Why this is Phase 1

This is intentionally a low-risk, non-breaking preparation step. The aim is to make the matcher runtime production-aware without rewriting the core algorithm prematurely.

The next phases are expected to be:

1. replace default map-based postings with compressed stores
2. add segment snapshot and schema-aware versioning
3. introduce cost-based query execution for full execution planning
4. add multi-threaded or sharded read paths for production scale

## Risk control

- old APIs remain valid
- benchmark and planner code are additive
- no index serialization format is changed yet
- no segment rewrite or breaking storage migration is introduced in this phase

## Validation strategy

Recommended checks:

```bash
go test ./...
go test ./... -run TestBEIndex_Retrieve

# benchmark visibility
# go test ./benchmark/... -bench . -benchmem
```

## Review guidance

Reviewers should focus on: 

- compatibility with the current public API
- clarity of the runtime abstraction boundaries
- ability to evolve toward compressed storage and planner-driven execution
- absence of behavior regression in legacy retrieval paths

## Status

Phase 1 is intentionally scaffolding-focused. It is designed to prepare the codebase for later production optimizations while preserving current correctness and behavior.
