package richeditor

import (
	"slices"
	"time"
)

// historyKey is the history plugin's key.
const historyKey = "history"

// HistoryOptions tune the history.
type HistoryOptions struct {
	// Depth is the most undo steps kept, 100 when 0.
	Depth int
	// NewGroupDelay is the pause that starts a new undo step, 500 ms
	// when 0.
	NewGroupDelay time.Duration
}

const (
	maxEmptyItems = 500
	depthOverflow = 20
)

// histItem is a step of a branch: a step and its map, or a map alone for a
// change outside the history; a selection marks the start of an event.
type histItem struct {
	m            *StepMap
	step         Step
	selection    Bookmark
	mirrorOffset int
}

func (i *histItem) merge(o *histItem) *histItem {
	if i.step != nil && o.step != nil && o.selection == nil {
		if step := o.step.Merge(i.step); step != nil {
			return &histItem{m: step.StepMap().Invert(), step: step, selection: i.selection}
		}
	}
	return nil
}

// branch is the done or undone side.
type branch struct {
	items      []*histItem
	eventCount int
}

type popped struct {
	remaining *branch
	transform *Transaction
	selection Bookmark
}

func (b *branch) popEvent(s *State, preserve bool) *popped {
	if b.eventCount == 0 {
		return nil
	}
	end := len(b.items)
	for ; ; end-- {
		if b.items[end-1].selection != nil {
			end--
			break
		}
	}
	var remap *Mapping
	mapFrom := 0
	if preserve {
		remap = b.remapping(end, len(b.items))
		mapFrom = len(remap.Maps)
	}
	tr := s.Tr()
	var selection Bookmark
	var remaining *branch
	var addAfter, addBefore []*histItem
	for i := len(b.items) - 1; i >= 0; i-- {
		item := b.items[i]
		if item.step == nil {
			if remap == nil {
				remap = b.remapping(end, i+1)
				mapFrom = len(remap.Maps)
			}
			mapFrom--
			addBefore = append(addBefore, item)
			continue
		}
		if remap != nil {
			addBefore = append(addBefore, &histItem{m: item.m})
			step := item.step.Map(remap.Slice(mapFrom, len(remap.Maps)))
			var m *StepMap
			if step != nil {
				tr.Transform.Step(step)
				if tr.Err == nil {
					m = tr.Mapping.Maps[len(tr.Mapping.Maps)-1]
					addAfter = append(addAfter, &histItem{m: m})
				}
			}
			mapFrom--
			if m != nil {
				remap.AppendMap(m, mapFrom)
			}
		} else {
			tr.Transform.Step(item.step)
		}
		if item.selection != nil {
			if remap != nil {
				selection = item.selection.Map(remap.Slice(mapFrom, len(remap.Maps)))
			} else {
				selection = item.selection
			}
			items := slices.Clone(b.items[:end])
			slices.Reverse(addBefore)
			items = append(items, addBefore...)
			slices.Reverse(addAfter)
			items = append(items, addAfter...)
			remaining = &branch{items, b.eventCount - 1}
			break
		}
	}
	if tr.Err != nil {
		return nil
	}
	return &popped{remaining, tr, selection}
}

func (b *branch) addTransform(tr *Transaction, selection Bookmark, depth int, preserve bool) *branch {
	var newItems []*histItem
	events := b.eventCount
	old := b.items
	var last *histItem
	if !preserve && len(old) > 0 {
		last = old[len(old)-1]
	}
	for i := range tr.Steps {
		step := tr.Steps[i].Invert(tr.Docs[i])
		item := &histItem{m: tr.Mapping.Maps[i], step: step, selection: selection}
		if last != nil {
			if merged := last.merge(item); merged != nil {
				item = merged
				if i > 0 {
					newItems = newItems[:len(newItems)-1]
				} else {
					old = old[:len(old)-1]
				}
			}
		}
		newItems = append(newItems, item)
		if selection != nil {
			events++
			selection = nil
		}
		if !preserve {
			last = item
		}
	}
	overflow := events - depth
	if overflow > depthOverflow {
		old = cutOffEvents(old, overflow)
		events -= overflow
	}
	return &branch{append(slices.Clone(old), newItems...), events}
}

func (b *branch) remapping(from, to int) *Mapping {
	maps := NewMapping()
	for i := from; i < to; i++ {
		item := b.items[i]
		mirror := -1
		if item.mirrorOffset != 0 && i-item.mirrorOffset >= from {
			mirror = len(maps.Maps) - item.mirrorOffset
		}
		maps.AppendMap(item.m, mirror)
	}
	return maps
}

func (b *branch) addMaps(maps []*StepMap) *branch {
	if b.eventCount == 0 {
		return b
	}
	items := slices.Clone(b.items)
	for _, m := range maps {
		items = append(items, &histItem{m: m})
	}
	return &branch{items, b.eventCount}
}

func (b *branch) emptyItemCount() int {
	n := 0
	for _, i := range b.items {
		if i.step == nil {
			n++
		}
	}
	return n
}

func (b *branch) compress(upto int) *branch {
	remap := b.remapping(0, upto)
	mapFrom := len(remap.Maps)
	var items []*histItem
	events := 0
	for i := len(b.items) - 1; i >= 0; i-- {
		item := b.items[i]
		switch {
		case i >= upto:
			items = append(items, item)
			if item.selection != nil {
				events++
			}
		case item.step != nil:
			step := item.step.Map(remap.Slice(mapFrom, len(remap.Maps)))
			var m *StepMap
			if step != nil {
				m = step.StepMap()
			}
			mapFrom--
			if m != nil {
				remap.AppendMap(m, mapFrom)
			}
			if step != nil {
				sel := item.selection
				if sel != nil {
					sel = sel.Map(remap.Slice(mapFrom, len(remap.Maps)))
					events++
				}
				newItem := &histItem{m: m.Invert(), step: step, selection: sel}
				if n := len(items) - 1; n >= 0 {
					if merged := newItem.merge(items[n]); merged != nil {
						items[n] = merged
						continue
					}
				}
				items = append(items, newItem)
			}
		default:
			mapFrom--
		}
	}
	slices.Reverse(items)
	return &branch{items, events}
}

func cutOffEvents(items []*histItem, n int) []*histItem {
	for i := range items {
		if items[i].selection != nil {
			n--
			if n == 0 {
				return items[i:]
			}
		}
	}
	return nil
}

// HistoryState is the history plugin's field.
type HistoryState struct {
	done, undone *branch
	prevRanges   []int
	prevTime     time.Time
	// prevComposition groups an input method's commits.
	prevComposition any
}

type historyMeta struct {
	redo         bool
	historyState *HistoryState
}

const closeHistoryKey metaKey = "closeHistory"
const historyMetaKey metaKey = "history$"

// CloseHistory has the next change start a new undo step.
func CloseHistory(tr *Transaction) *Transaction { return tr.SetMeta(closeHistoryKey, true) }

func rangesFor(maps []*StepMap) []int {
	var out []int
	for i := len(maps) - 1; i >= 0 && len(out) == 0; i-- {
		maps[i].ForEach(func(_, _, from, to int) { out = append(out, from, to) })
	}
	return out
}

func mapRanges(ranges []int, m *Mapping) []int {
	var out []int
	for i := 0; i+1 < len(ranges); i += 2 {
		from, to := m.Map(ranges[i], 1), m.Map(ranges[i+1], -1)
		if from <= to {
			out = append(out, from, to)
		}
	}
	return out
}

func isAdjacentTo(tr *Transaction, prev []int) bool {
	if prev == nil {
		return false
	}
	if !tr.DocChanged() {
		return true
	}
	adjacent := false
	tr.Mapping.Maps[0].ForEach(func(start, end, _, _ int) {
		for i := 0; i+1 < len(prev); i += 2 {
			if start <= prev[i+1] && end >= prev[i] {
				adjacent = true
			}
		}
	})
	return adjacent
}

func applyHistory(h *HistoryState, s *State, tr *Transaction, o HistoryOptions) *HistoryState {
	if meta, ok := tr.Meta(historyMetaKey).(*historyMeta); ok {
		return meta.historyState
	}
	if tr.Meta(closeHistoryKey) != nil {
		cp := *h
		cp.prevTime = time.Time{}
		h = &cp
	}
	appended, _ := tr.Meta(appendedKey).(*Transaction)
	switch {
	case len(tr.Steps) == 0:
		return h
	case appended != nil && appended.Meta(historyMetaKey) != nil:
		meta := appended.Meta(historyMetaKey).(*historyMeta)
		if meta.redo {
			return &HistoryState{done: h.done.addTransform(tr, nil, o.Depth, false), undone: h.undone,
				prevRanges: rangesFor(tr.Mapping.Maps), prevTime: h.prevTime}
		}
		return &HistoryState{done: h.done, undone: h.undone.addTransform(tr, nil, o.Depth, false), prevTime: h.prevTime, prevRanges: h.prevRanges}
	case tr.Meta(addToHistory) != false && !(appended != nil && appended.Meta(addToHistory) == false):
		comp := tr.Meta(compositionID)
		newGroup := h.prevTime.IsZero() || (appended == nil && (comp == nil || comp != h.prevComposition) &&
			(tr.Time.Sub(h.prevTime) > o.NewGroupDelay || !isAdjacentTo(tr, h.prevRanges)))
		prevRanges := rangesFor(tr.Mapping.Maps)
		if appended != nil {
			prevRanges = mapRanges(h.prevRanges, tr.Mapping)
		}
		var sel Bookmark
		if newGroup {
			sel = s.Selection.Bookmark()
		}
		when := h.prevTime
		if appended == nil || when.IsZero() {
			when = tr.Time
		}
		return &HistoryState{done: h.done.addTransform(tr, sel, o.Depth, false), undone: &branch{},
			prevRanges: prevRanges, prevTime: when, prevComposition: comp}
	default:
		maps := tr.Mapping.Maps
		return &HistoryState{done: h.done.addMaps(maps), undone: h.undone.addMaps(maps),
			prevRanges: mapRanges(h.prevRanges, tr.Mapping), prevTime: h.prevTime}
	}
}

// History is the undo history: changes further apart than NewGroupDelay, or
// not adjacent, start a new step, an input method's commits group, and
// changes kept out of it or made by others map its steps.
func History(o HistoryOptions) *Plugin {
	if o.Depth == 0 {
		o.Depth = 100
	}
	if o.NewGroupDelay == 0 {
		o.NewGroupDelay = 500 * time.Millisecond
	}
	return &Plugin{Key: historyKey,
		Init: func(*State) any { return &HistoryState{done: &branch{}, undone: &branch{}} },
		Apply: func(tr *Transaction, v any, old, _ *State) any {
			return applyHistory(v.(*HistoryState), old, tr, o)
		}}
}

func histTransaction(h *HistoryState, s *State, redo bool) *Transaction {
	source, other := h.done, h.undone
	if redo {
		source, other = h.undone, h.done
	}
	pop := source.popEvent(s, false)
	if pop == nil {
		return nil
	}
	selection := pop.selection.Resolve(pop.transform.Doc)
	added := other.addTransform(pop.transform, s.Selection.Bookmark(), 100, false)
	next := &HistoryState{done: pop.remaining, undone: added}
	if redo {
		next = &HistoryState{done: added, undone: pop.remaining}
	}
	tr := pop.transform.SetSelection(selection)
	tr.Origin = OriginHistory
	tr.SetMeta(historyMetaKey, &historyMeta{redo: redo, historyState: next})
	return tr
}

func historyCommand(redo bool) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		h, ok := s.PluginState(historyKey).(*HistoryState)
		if !ok {
			return false
		}
		source := h.done
		if redo {
			source = h.undone
		}
		if source.eventCount == 0 {
			return false
		}
		if dispatch != nil {
			tr := histTransaction(h, s, redo)
			if tr == nil {
				return false
			}
			dispatch(tr.ScrollIntoView())
		}
		return true
	}
}

// Undo and Redo walk the history.
var (
	Undo = historyCommand(false)
	Redo = historyCommand(true)
)

// UndoDepth and RedoDepth are the steps there are.
func UndoDepth(s *State) int {
	if h, ok := s.PluginState(historyKey).(*HistoryState); ok {
		return h.done.eventCount
	}
	return 0
}

// RedoDepth is the steps Redo can take.
func RedoDepth(s *State) int {
	if h, ok := s.PluginState(historyKey).(*HistoryState); ok {
		return h.undone.eventCount
	}
	return 0
}
