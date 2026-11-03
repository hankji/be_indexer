package be_indexer

import "sort"

// queryExecutionPlan is the concrete execution-order metadata for a retrieval
// request. It is intentionally small and used as a compatibility layer before the
// production planner evolves into a deeper query optimization engine.
type queryExecutionPlan struct {
	orderedFields []BEField
}

// orderedAssignmentFields honors the planner's field order, removes duplicates,
// and appends any query fields omitted by the planner in deterministic order.
// This ensures that an incomplete/custom plan never silently drops predicates.
func orderedAssignmentFields(queries Assignments, plan QueryPlan) []BEField {
	ordered := make([]BEField, 0, len(queries))
	seen := make(map[BEField]struct{}, len(queries))
	for _, field := range plan.Fields {
		if _, ok := queries[field]; !ok {
			continue
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		ordered = append(ordered, field)
	}

	missing := make([]BEField, 0, len(queries)-len(seen))
	for field := range queries {
		if _, exists := seen[field]; !exists {
			missing = append(missing, field)
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	return append(ordered, missing...)
}

// buildExecutionPlan computes a query execution order based on the planner.
func buildExecutionPlan(queries Assignments) queryExecutionPlan {
	plan := NewDefaultQueryPlanner().Plan(queries)
	return queryExecutionPlan{orderedFields: orderedAssignmentFields(queries, plan)}
}

// orderedAssignments returns a new assignment map with exactly the original
// assignments. The map itself does not preserve iteration order; execution code
// must use orderedFields directly when order matters.
func orderedAssignments(queries Assignments, plan queryExecutionPlan) Assignments {
	ordered := make(Assignments, len(queries))
	for _, field := range orderedAssignmentFields(queries, QueryPlan{Fields: plan.orderedFields}) {
		ordered[field] = queries[field]
	}
	return ordered
}
