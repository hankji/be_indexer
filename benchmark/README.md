# Production Benchmark Harness

This folder contains lightweight benchmark scaffolding for the predicate matching core.

## Goals

- verify query planner cost ordering stays stable
- estimate retrieval overhead under realistic boolean conditions
- establish a baseline before deeper storage and segment optimization

## Commands

```bash
go test ./benchmark/... -bench . -benchmem
```

## Notes

The benchmarks intentionally keep the legacy API as the default path so they can be used
as a compatibility guard while Phase 1 production runtime utilities are introduced.
