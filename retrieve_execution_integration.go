package be_indexer

import "github.com/echoface/be_indexer/util"

// retrieveWithExecutionPlan is the Phase 2 compatibility execution path that
// applies the planner-derived field ordering before invoking the legacy
// k-group retrieval algorithm.
//
// This preserves backward compatibility while enabling a controlled migration to
// planner-driven execution for the hot path.
func (bi *KGroupsBEIndex) retrieveWithExecutionPlan(queries Assignments, collector ResultCollector, opts ...IndexOpt) error {
	ctx := newRetrieveCtx(queries, opts...)
	util.PanicIf(ctx.collector != nil, "can't specify collector twice")
	ctx.collector = collector

	plan := buildExecutionPlan(queries)
	if len(plan.orderedFields) > 0 {
		ctx.assigns = orderedAssignments(queries, plan)
	}

	var err error
	var fCursors FieldCursors
	for k := util.MinInt(queries.Size(), bi.maxK()); k >= 0; k-- {
		if fCursors, err = bi.initCursors(&ctx, k); err != nil {
			return err
		}
		LogInfoIf(ctx.dumpStepInfo, "start@step:%d cursors:%d", k, len(fCursors))
		if ctx.dumpEntriesDetail {
			Logger.Infof("RetrieveWithCollector k:%d initial entries:\n%s", k, fCursors.Dump())
		}

		needMatchCnt := util.MaxInt(k, 1)
		bi.retrieveK(&ctx, fCursors, needMatchCnt)
	}
	return nil
}

// executeLegacyRetrieve preserves the previous retrieve behavior and acts as the
// explicit fallback path when no planner ordering is available.
func (bi *KGroupsBEIndex) executeLegacyRetrieve(queries Assignments, collector ResultCollector, opts ...IndexOpt) error {
	ctx := newRetrieveCtx(queries, opts...)
	util.PanicIf(ctx.collector != nil, "can't specify collector twice")
	ctx.collector = collector

	var err error
	var fCursors FieldCursors
	for k := util.MinInt(queries.Size(), bi.maxK()); k >= 0; k-- {
		if fCursors, err = bi.initCursors(&ctx, k); err != nil {
			return err
		}
		LogInfoIf(ctx.dumpStepInfo, "start@step:%d cursors:%d", k, len(fCursors))
		if ctx.dumpEntriesDetail {
			Logger.Infof("RetrieveWithCollector k:%d initial entries:\n%s", k, fCursors.Dump())
		}
		needMatchCnt := util.MaxInt(k, 1)
		bi.retrieveK(&ctx, fCursors, needMatchCnt)
	}
	return nil
}
