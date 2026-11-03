package be_indexer_test

import (
	"testing"

	"github.com/echoface/be_indexer"
	_ "github.com/echoface/be_indexer/holder/rangeholder"
)

func BenchmarkQueryPlannerPlan(b *testing.B) {
	planner := be_indexer.NewDefaultQueryPlanner()
	planner.RegisterFieldStat(be_indexer.QueryFieldStat{
		Field:                 "city",
		Cardinality:           128,
		EstimatedSelectivity: 0.05,
		PostingCount:          128,
	})
	planner.RegisterFieldStat(be_indexer.QueryFieldStat{
		Field:                 "device",
		Cardinality:           8,
		EstimatedSelectivity: 0.4,
		PostingCount:          8,
	})
	planner.RegisterFieldStat(be_indexer.QueryFieldStat{
		Field:                 "age",
		Cardinality:           64,
		EstimatedSelectivity: 0.2,
		PostingCount:          64,
	})

	assigns := be_indexer.Assignments{
		"city":   be_indexer.NewStrValues("beijing", "shanghai"),
		"device": be_indexer.NewStrValues("ios", "android"),
		"age":    be_indexer.NewIntValues(18, 25, 30),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = planner.Plan(assigns)
	}
}

func BenchmarkKGroupsBEIndexRetrieve(b *testing.B) {
	builder := be_indexer.NewIndexerBuilder()
	builder.ConfigField("city", be_indexer.FieldOption{Container: be_indexer.HolderNameDefault})
	builder.ConfigField("device", be_indexer.FieldOption{Container: be_indexer.HolderNameDefault})
	builder.ConfigField("age", be_indexer.FieldOption{Container: be_indexer.HolderNameExtendRange})

	for i := 0; i < 2000; i++ {
		doc := be_indexer.NewDocument(be_indexer.DocID(i))
		doc.AddConjunction(be_indexer.NewConjunction().
			In("city", be_indexer.NewStrValues("beijing", "shanghai")).
			In("device", be_indexer.NewStrValues("ios", "android")).
			Between("age", 18, 30))
		if err := builder.AddDocument(doc); err != nil {
			b.Fatal(err)
		}
	}

	indexer := builder.BuildIndex()
	queries := be_indexer.Assignments{
		"city":   be_indexer.NewStrValues("beijing"),
		"device": be_indexer.NewStrValues("ios"),
		"age":    be_indexer.NewIntValues(20),
	}

	b.ReportAllocs()
	b.ResetTimer() // Exclude index construction and query setup from retrieval timings.
	for i := 0; i < b.N; i++ {
		if _, err := indexer.Retrieve(queries); err != nil {
			b.Fatal(err)
		}
	}
}
