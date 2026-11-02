package be_indexer

// queryExecutionPlan is the concrete execution-order metadata for a retrieval
// request. It is intentionally tiny and used as a compatibility layer before the
// production planner evolves into a deeper query optimization engine.
type queryExecutionPlan struct {
	orderedFields []BEField
}

// buildExecutionPlan computes a query execution order based on the planner.
// The legacy execution path remains as an explicit fallback when the planner
// returns an empty plan or no field metadata is available.
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

// orderedAssignments returns a new assignment map in the planner-selected field
// order while preserving the original query values.
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
