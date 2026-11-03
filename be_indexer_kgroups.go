package be_indexer

import (
	"container/heap"
	"fmt"
	"sort"
	"strings"

	"github.com/echoface/be_indexer/util"
)

type (
	EntriesContainer struct {
		defaultHolder EntriesHolder
		fieldHolder   map[BEField]EntriesHolder
	}

	KGroupsBEIndex struct {
		indexBase
		kSizeContainers []*EntriesContainer
	}
)

func newFieldEntriesContainer() *EntriesContainer {
	return &EntriesContainer{defaultHolder: NewEntriesHolder(HolderNameDefault), fieldHolder: map[BEField]EntriesHolder{}}
}

func (c *EntriesContainer) compileEntries() (err error) {
	if err = c.defaultHolder.CompileEntries(); err != nil {
		return err
	}
	for _, holder := range c.fieldHolder {
		if err = holder.CompileEntries(); err != nil {
			return err
		}
	}
	return nil
}

func (c *EntriesContainer) getFieldHolder(desc *FieldDesc) EntriesHolder {
	if desc.Container == HolderNameDefault {
		return c.defaultHolder
	}
	return c.fieldHolder[desc.Field]
}

func (c *EntriesContainer) GetHolder(desc *FieldDesc) EntriesHolder {
	return c.getFieldHolder(desc)
}

func (c *EntriesContainer) CreateHolder(desc *FieldDesc) EntriesHolder {
	if holder := c.GetHolder(desc); holder != nil {
		return holder
	}
	util.PanicIf(!HasHolderBuilder(desc.Container), "field:%s container:%s not found, plz register it", desc.Field, desc.Container)
	holder := NewEntriesHolder(desc.Container)
	c.fieldHolder[desc.Field] = holder
	return holder
}

func (c *EntriesContainer) DumpEntries(buf *strings.Builder) {
	c.defaultHolder.DumpEntries(buf)
	for field, holder := range c.fieldHolder {
		buf.WriteString(fmt.Sprintf("\nfield:%s holder:\n", field))
		holder.DumpEntries(buf)
	}
}

func (c *EntriesContainer) DumpInfo(buf *strings.Builder) {
	buf.WriteString("holder information:")
	c.defaultHolder.DumpInfo(buf)
	for field, holder := range c.fieldHolder {
		buf.WriteString(fmt.Sprintf("  >field:%s ", field))
		holder.DumpInfo(buf)
		buf.WriteString("\n")
	}
}

func NewKGroupsBEIndex() BEIndex {
	return &KGroupsBEIndex{
		indexBase:       indexBase{fieldsData: make(map[BEField]*FieldDesc)},
		kSizeContainers: make([]*EntriesContainer, 0),
	}
}

func (bi *KGroupsBEIndex) maxK() int { return len(bi.kSizeContainers) - 1 }

func (bi *KGroupsBEIndex) newContainer(k int) *EntriesContainer {
	for k >= len(bi.kSizeContainers) {
		bi.kSizeContainers = append(bi.kSizeContainers, newFieldEntriesContainer())
	}
	return bi.kSizeContainers[k]
}

func (bi *KGroupsBEIndex) compileIndexer() (err error) {
	for _, sizeEntries := range bi.kSizeContainers {
		if err = sizeEntries.compileEntries(); err != nil {
			return err
		}
	}
	if bi.wildcardEntries.Len() > 0 {
		sort.Sort(bi.wildcardEntries)
	}
	return nil
}

func (bi *KGroupsBEIndex) getKSizeEntries(k int) *EntriesContainer {
	if k >= len(bi.kSizeContainers) {
		panic(fmt.Errorf("k:[%d] out of range", k))
	}
	return bi.kSizeContainers[k]
}

func (bi *KGroupsBEIndex) initCursors(ctx *retrieveContext, k int) (FieldCursors, error) {
	fCursors := make(FieldCursors, 0, len(bi.fieldsData))
	if k == 0 && len(bi.wildcardEntries) > 0 {
		fCursors = append(fCursors, NewFieldCursor(NewEntriesCursor(wildcardQKey, bi.wildcardEntries)))
	}
	kSizeContainer := bi.getKSizeEntries(k)
	plan := buildExecutionPlan(ctx.assigns)
	orderedFields := plan.orderedFields
	if len(orderedFields) == 0 {
		orderedFields = orderedAssignmentFields(ctx.assigns, QueryPlan{})
	}
	for _, field := range orderedFields {
		values := ctx.assigns[field]
		desc, ok := bi.fieldsData[field]
		if !ok {
			continue
		}
		holder := kSizeContainer.getFieldHolder(desc)
		if holder == nil {
			continue
		}
		entriesList, err := holder.GetEntries(desc, values)
		if err != nil {
			Logger.Errorf("fetch entries from holder fail:%s, field:%s\n", err.Error(), desc.Field)
			return nil, err
		}
		if len(entriesList) > 0 {
			fCursors = append(fCursors, NewFieldCursor(entriesList...))
			Logger.Debugf("<%s,%v> fetch %d posting list\n", desc.Field, values, len(entriesList))
		} else {
			Logger.Debugf("<%s,%v> nothing matched from entries holder\n", desc.Field, values)
		}
	}
	return fCursors, nil
}

func (bi *KGroupsBEIndex) retrieveK(ctx *retrieveContext, fieldCursors FieldCursors, needMatchCnt int) {
	if len(fieldCursors) < needMatchCnt {
		LogInfoIf(ctx.dumpStepInfo, "need match:%d but only:%d", needMatchCnt, len(fieldCursors))
		return
	}
	fieldCursors.Sort()
	for !fieldCursors[needMatchCnt-1].GetCurEntryID().IsNULLEntry() {
		if ctx.dumpStepInfo {
			Logger.Infof("round need match:%d continue docs:%v", needMatchCnt, ctx.collector.GetDocIDs())
		}
		if ctx.dumpEntriesDetail {
			Logger.Infof("round need match:%d continue entries\n%s", needMatchCnt, fieldCursors.Dump())
		}
		eid := fieldCursors[0].GetCurEntryID()
		endEID := fieldCursors[needMatchCnt-1].GetCurEntryID()
		conjID := eid.GetConjID()
		endConjID := endEID.GetConjID()
		nextID := NewEntryID(endConjID, false)
		if conjID == endConjID {
			nextID = NewEntryID(endEID.GetConjID(), true) + 1
			if eid.IsInclude() {
				ctx.collector.Add(conjID.DocID(), conjID)
			} else {
				for i := needMatchCnt; i < len(fieldCursors); i++ {
					if fieldCursors[i].GetCurEntryID() < nextID {
						fieldCursors[i].SkipTo(nextID)
					}
				}
			}
		}
		for i := 0; i < needMatchCnt; i++ {
			fieldCursors[i].SkipTo(nextID)
		}
		fieldCursors.Sort()
		if ctx.dumpStepInfo {
			Logger.Infof("round end need match:%d, docs:%v", needMatchCnt, ctx.collector.GetDocIDs())
		}
	}
}

func (bi *KGroupsBEIndex) Retrieve(queries Assignments, opts ...IndexOpt) (DocIDList, error) {
	collector := PickCollector()
	defer PutCollector(collector)
	if err := bi.RetrieveWithCollector(queries, collector, opts...); err != nil {
		return nil, err
	}
	return collector.GetDocIDs(), nil
}

func (bi *KGroupsBEIndex) RetrieveWithCollector(queries Assignments, collector ResultCollector, opts ...IndexOpt) error {
	ctx := newRetrieveCtx(queries, opts...)
	util.PanicIf(ctx.collector != nil, "can't specify collector twice")
	ctx.collector = collector
	plan := buildExecutionPlan(queries)
	if len(plan.orderedFields) > 0 {
		ctx.assigns = orderedAssignments(queries, plan)
	}
	for k := util.MinInt(queries.Size(), bi.maxK()); k >= 0; k-- {
		fCursors, err := bi.initCursors(&ctx, k)
		if err != nil {
			return err
		}
		LogInfoIf(ctx.dumpStepInfo, "start@step:%d cursors:%d", k, len(fCursors))
		if ctx.dumpEntriesDetail {
			Logger.Infof("RetrieveWithCollector k:%d initial entries:\n%s", k, fCursors.Dump())
		}
		bi.retrieveK(&ctx, fCursors, util.MaxInt(k, 1))
	}
	return nil
}

func (bi *KGroupsBEIndex) DumpEntries(sb *strings.Builder) {
	sb.WriteString("\n+++++++ size grouped boolean indexing entries +++++++++++ \n>>Z:\n")
	sb.WriteString(wildcardQKey.String())
	sb.WriteString(":")
	sb.WriteString(fmt.Sprintf("%v", bi.wildcardEntries.DocString()))
	sb.WriteString("\n")
	for size, container := range bi.kSizeContainers {
		sb.WriteString(fmt.Sprintf(">>K:%d\n", size))
		container.DumpEntries(sb)
		sb.WriteString("\n")
	}
	sb.WriteString("+++++++++++++size grouped index entries end +++++++++++++++++\n")
}

func (bi *KGroupsBEIndex) DumpIndexInfo(sb *strings.Builder) {
	sb.WriteString("\n+++++++ size grouped boolean indexing info +++++++++++\n")
	sb.WriteString(fmt.Sprintf("wildcard info: count:%d\n", len(bi.wildcardEntries)))
	for k, c := range bi.kSizeContainers {
		sb.WriteString(fmt.Sprintf(">> container for size k:%d\n", k))
		c.DumpInfo(sb)
		sb.WriteString("\n")
	}
	sb.WriteString("++++++++++++ size grouped index info end ++++++++++++++++++\n")
}

// retrieveK_HeapOptimized is an experimental alternative. Keep it out of the
// default retrieval path until equivalence tests and benchmarks justify enabling it.
func (bi *KGroupsBEIndex) retrieveK_HeapOptimized(ctx *retrieveContext, fieldCursors FieldCursors, needMatchCnt int) {
	if len(fieldCursors) < needMatchCnt {
		LogInfoIf(ctx.dumpStepInfo, "need match:%d but only:%d", needMatchCnt, len(fieldCursors))
		return
	}
	h := NewFieldCursorHeap(fieldCursors)
	for len(h.cursors) >= needMatchCnt {
		minCursor := h.GetMin()
		needCursor := h.GetNth(needMatchCnt - 1)
		minEID, needEID := minCursor.GetCurEntryID(), needCursor.GetCurEntryID()
		conjID, needConjID := minEID.GetConjID(), needEID.GetConjID()
		nextID := NewEntryID(needConjID, false)
		if conjID == needConjID {
			nextID = NewEntryID(needConjID, true) + 1
			if minEID.IsInclude() {
				ctx.collector.Add(conjID.DocID(), conjID)
			} else {
				for i := needMatchCnt; i < len(h.cursors); i++ {
					if h.cursors[i].GetCurEntryID() < nextID {
						h.cursors[i].SkipTo(nextID)
						heap.Fix(h, i)
					}
				}
			}
		}
		for i := 0; i < needMatchCnt; i++ {
			h.cursors[i].SkipTo(nextID)
			heap.Fix(h, i)
		}
		for len(h.cursors) > 0 && h.cursors[0].ReachEnd() {
			heap.Pop(h)
		}
	}
}
