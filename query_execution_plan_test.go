package be_indexer

import (
	"reflect"
	"testing"
)

func TestOrderedAssignmentFieldsHonorsPlanAndRetainsUnplannedFields(t *testing.T) {
	queries := Assignments{
		"alpha":   NewStrValues("a"),
		"bravo":   NewStrValues("b"),
		"charlie": NewStrValues("c"),
	}
	plan := QueryPlan{Fields: []BEField{"bravo", "bravo", "unknown"}}

	got := orderedAssignmentFields(queries, plan)
	want := []BEField{"bravo", "alpha", "charlie"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orderedAssignmentFields() = %v, want %v", got, want)
	}
}

func TestBuildExecutionPlanIsDeterministicWithoutStats(t *testing.T) {
	queries := Assignments{
		"zeta":  NewStrValues("z"),
		"alpha": NewStrValues("a"),
		"mu":    NewStrValues("m"),
	}

	want := []BEField{"alpha", "mu", "zeta"}
	for i := 0; i < 10; i++ {
		if got := buildExecutionPlan(queries).orderedFields; !reflect.DeepEqual(got, want) {
			t.Fatalf("buildExecutionPlan().orderedFields = %v, want %v", got, want)
		}
	}
}

func TestOrderedAssignmentsPreservesEveryValue(t *testing.T) {
	alpha := NewStrValues("a")
	bravo := NewStrValues("b")
	queries := Assignments{"alpha": alpha, "bravo": bravo}

	got := orderedAssignments(queries, queryExecutionPlan{orderedFields: []BEField{"bravo"}})
	if len(got) != len(queries) {
		t.Fatalf("orderedAssignments() returned %d fields, want %d", len(got), len(queries))
	}
	if !reflect.DeepEqual(got["alpha"], alpha) || !reflect.DeepEqual(got["bravo"], bravo) {
		t.Fatalf("orderedAssignments() did not preserve original Values: got %#v, want alpha=%#v bravo=%#v", got, alpha, bravo)
	}
}
