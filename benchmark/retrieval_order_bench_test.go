package be_indexer

import "testing"

func newRetrievalOrderBenchmarkFixture(b *testing.B) (*KGroupsBEIndex, Assignments) {
	b.Helper()
	builder := NewIndexerBuilder()
	builder.ConfigField("city", FieldOption{Container: HolderNameDefault})
	builder.ConfigField("device", FieldOption{Container: HolderNameDefault})
	builder.ConfigField("age", FieldOption{Container: HolderNameDefault})

	for i := 0; i < 2000; i++ {
		doc := NewDocument(DocID(i))
		doc.AddConjunction(NewConjunction().
			In("city", NewStrValues("beijing", "shanghai")).
			In("device", NewStrValues("ios", "android")).
			In("age", NewIntValues(18, 25, 30)))
		if err := builder.AddDocument(doc); err != nil {
			b.Fatal(err)
		}
	}

	index, ok := builder.BuildIndex().(*KGroupsBEIndex)
	if !ok {
		b.Fatal("builder returned a non-KGroups index")
	}
	queries := Assignments{
		"city":   NewStrValues("beijing"),
		"device": NewStrValues("ios"),
		"age":    NewIntValues(25),
	}
	return index, queries
}

// legacyRetrieveForBenchmark reproduces the pre-planner map iteration order for
// cursor initialization while keeping the retrieval algorithm and fixture equal.
func legacyRetrieveForBenchmark(b *testing.B, index *KGroupsBEIndex, queries Assignments) {
	b.Helper()
	collector := PickCollector()
	defer PutCollector(collector)

	ctx := newRetrieveCtx(queries)
	ctx.collector = collector
	for k := util.MinInt(queries.Size(), index.maxK()); k >= 0; k-- {
		fields := make([]BEField, 0, len(queries))
		for field := range queries {
			fields = append(fields, field)
		}
		cursors, err := index.initCursorsWithOrder(&ctx, k, fields)
		if err != nil {
			b.Fatal(err)
		}
		index.retrieveK(&ctx, cursors, util.MaxInt(k, 1))
	}
	_ = collector.GetDocIDs()
}

func BenchmarkKGroupsRetrieveOrderComparison(b *testing.B) {
	b.Run("legacy_map_order", func(b *testing.B) {
		index, queries := newRetrievalOrderBenchmarkFixture(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			legacyRetrieveForBenchmark(b, index, queries)
		}
	})

	b.Run("planner_order", func(b *testing.B) {
		index, queries := newRetrievalOrderBenchmarkFixture(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := index.RetrieveWithCollector(queries, PickCollector()); err != nil {
				b.Fatal(err)
			}
		}
	})
}
