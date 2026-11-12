package tsx

import (
	"slices"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

func init() {
	moduleTable["nsgo/base"] = append(moduleTable["nsgo/base"], faceExport("List", baseListFace))
}

// baseListFace is base.List over each's items, as <List> is core.List: JS
// has no Selection, so the rows chosen are the props selected or
// defaultSelected and onSelectedChange.
func baseListFace(p *props) core.Node {
	r := p.r
	rows := r.listRows(p)
	l := base.List{Count: rows.count, Key: rows.key, Row: rows.row, Selection: r.listSelection(p), TapSelects: p.bool("tapSelects"),
		Columns: p.int("numColumns"), EstimatedSize: p.points("estimatedItemSize"), Gap: p.points("gap"),
		Header: r.listSlot(p, "header"), Footer: r.listSlot(p, "footer"), Empty: r.listSlot(p, "empty"), OnLoadMore: p.on("onLoadMore"),
		Label: p.str("accessibilityLabel"), Ref: p.n.ref, TestID: p.str("testID"), Class: p.class(), StyleFunc: p.style,
		ListClass: p.str("listClass"), ItemClass: p.str("itemClass")}
	if p.has("listStyle") {
		l.ListStyle = r.style(p.get("listStyle"))
	}
	if p.has("itemStyle") {
		l.ItemStyleFunc = func() core.Style { return r.style(p.get("itemStyle")) }
	}
	if h := p.handler("onActivate"); h != nil {
		l.OnActivate = func(i int) {
			e := r.vm.NewObject()
			_ = e.Set("item", rows.item(i))
			_ = e.Set("index", i)
			h(e)
		}
	}
	if h := p.handler("rowLabel"); h != nil {
		l.RowLabel = func(i int) string { return stringOf(h(rows.item(i), r.vm.ToValue(i))) }
	}
	return l
}

// listSelection is the selection a base list's props describe, nil for a
// list that chooses nothing.
func (r *runtime) listSelection(p *props) *base.Selection {
	if !p.has("multiple") && !p.has("selected") && !p.has("defaultSelected") && !p.has("onSelectedChange") {
		return nil
	}
	keys := func(prop string) []any {
		out := []any{}
		for _, v := range r.arrayItems(p.get(prop)) {
			out = append(out, keyOf(v))
		}
		return out
	}
	cx := r.cx
	s := base.NewSelection(cx, p.bool("multiple"))
	controlled := p.has("selected")
	if controlled {
		cx.Effect(func() {
			if want := keys("selected"); !slices.Equal(want, core.Untracked(cx, s.Keys)) {
				s.Set(want...)
			}
		})
	} else if p.has("defaultSelected") {
		s.Set(core.Untracked(cx, func() []any { return keys("defaultSelected") })...)
	}
	h := p.handler("onSelectedChange")
	last := core.Untracked(cx, s.Keys)
	cx.Effect(func() {
		now := s.Keys()
		if slices.Equal(now, last) {
			return
		}
		last = now
		cx.Untrack(func() {
			if h != nil {
				e := r.vm.NewObject()
				out := make([]any, len(now))
				for i, k := range now {
					out[i] = k
				}
				_ = e.Set("selected", r.vm.NewArray(out...))
				h(e)
			}
			// A selected the app kept takes back what the list chose.
			if want := keys("selected"); controlled && !slices.Equal(want, now) {
				last = want
				s.Set(want...)
			}
		})
	})
	return s
}
