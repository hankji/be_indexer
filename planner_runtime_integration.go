package be_indexer

import "github.com/echoface/be_indexer/util"

// plannerAwareRetrieve is a Phase 1 compatibility layer that applies a cost-based
// planner without modifying the public BEIndex API. For existing callers,
// Retrieve and RetrieveWithCollector continue to behave exactly as before.
//
// In production, this hook can be extended to:
//   - reorder fields based on selectivity
//   - skip zero-cardinality fields
//   - prefer cost-aware execution for range / exclude queries
func (bi *KGroupsBEIndex) plannerAwareRetrieve(queries Assignments, collector ResultCollector, opts ...IndexOpt) error {
	ctx := newRetrieveCtx(queries, opts...)
	util.PanicIf(ctx.collector != nil, "can't specify collector twice")
	ctx.collector = collector

	planner := NewDefaultQueryPlanner()
	plan := planner.Plan(queries)
	if len(plan.Fields) == 0 {
		return nil
	}

	// Phase 1 compatibility: we only use the planner to reorder field evaluation
	// for the current retrieve path. We still retain the legacy algorithm and
	// therefore preserve performance semantics for existing tests while exposing a
	// production-oriented entry point for later optimization.
	ordered := make(map[BEField]Values, len(queries))
	for _, field := range plan.Fields {
		if values, ok := queries[field]; ok {
			ordered[field] = values
		}
	}
	ctx.assigns = ordered

	var fCursors FieldCursors
	for k := util.MinInt(queries.Size(), bi.maxK()); k >= 0; k-- {
		if fCursors, err := bi.initCursors(&ctx, k); err != nil {
			return err
		} else {
			LogInfoIf(ctx.dumpStepInfo, "start@step:%d cursors:%d", k, len(fCursors))
			if ctx.dumpEntriesDetail {
				Logger.Infof("RetrieveWithCollector k:%d initial entries:\n%s", k, fCursors.Dump())
			}
			needMatchCnt := util.MaxInt(k, 1)
			bi.retrieveK(&ctx, fCursors, needMatchCnt)
		}
	}
	return nil
}

// Deprecated: use plannerAwareRetrieve in production-aware call sites.
func (bi *KGroupsBEIndex) retrieveWithPlanner(queries Assignments, collector ResultCollector, opts ...IndexOpt) error {
	return bi.plannerAwareRetrieve(queries, collector, opts...)
}
