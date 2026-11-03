package be_indexer

import "sort"

// queryExecutionPlan is the concrete execution-order metadata for a retrieval
// request. It is intentionally small and used as a compatibility layer before the
// production planner evolves into a deeper query optimization engine.
type queryExecutionPlan struct {
	orderedFields []BEField
}

func orderedAssignmentFields(queries Assignments, plan QueryPlan) []BEField {
	if len(plan.Fields) > 0 {
		ordered := make([]BEField, 0, len(plan.Fields))
		seen := make(map[BEField]struct{}, len(plan.Fields))
		for _, field := range plan.Fields {
			if _, ok := queries[field]; ok {
				if _, exists := seen[field]; exists {
					continue
				}
				seen[field] = struct{}{}
				ordered = append(ordered, field)
			}
		}
		if len(ordered) > 0 {
			return ordered
		}
	}
	fields := make([]BEField, 0, len(queries))
	for field := range queries {
		fields = append(fields, field)
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i] < fields[j] })
	return fields
}

// buildExecutionPlan computes a query execution order based on the planner.
func buildExecutionPlan(queries Assignments) queryExecutionPlan {
	planner := NewDefaultQueryPlanner()
	plan := planner.Plan(queries)
	if len(plan.Fields) == 0 {
		return queryExecutionPlan{}
	}
	ordered := make([]BEField, 0, len(plan.Fields))
	for _, field := range plan.Fields {
		if _, ok := queries[field]; ok {
			ordered = append(ordered, field)
		}
	}
	return queryExecutionPlan{orderedFields: ordered}
}

// orderedAssignments returns a new assignment map in planner-selected order while
// preserving original values.
func orderedAssignments(queries Assignments, plan queryExecutionPlan) Assignments {
	if len(plan.orderedFields) == 0 {
		return queries
	}
	ordered := make(Assignments, len(plan.orderedFields))
	for _, field := range plan.orderedFields {
		if values, ok := queries[field]; ok {
			ordered[field] = values
		}
	}
	return ordered
}
