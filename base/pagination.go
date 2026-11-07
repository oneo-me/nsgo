package base

import (
	"slices"
	"strconv"

	"github.com/oneo-me/nsgo/core"
)

// minVisiblePages is the fewest pages a Pagination shows.
const minVisiblePages = 5

// PaginationItemKind is what a Pagination's item is.
type PaginationItemKind uint8

const (
	PaginationPage PaginationItemKind = iota
	PaginationPrevious
	PaginationEllipsis
	PaginationNext
)

// PaginationItem is an item as RenderItem draws it.
type PaginationItem struct {
	Kind PaginationItemKind
	// Page is a page's number; for Previous and Next the page they go to,
	// the current one at the ends; for an ellipsis the first it leaves out.
	Page int
	// Start and End are an ellipsis' pages, End past the last.
	Start, End int
	// Current is the current page.
	Current bool
	// Disabled takes no press: everything of a disabled pagination,
	// Previous on the first page and Next on the last.
	Disabled bool
}

// paginationItems are what shows on page of total pages, showing at most
// visible pages, the first and the last around the current one and its
// neighbours, with an ellipsis for each run left out: gpui-base's
// calculate_items.
func paginationItems(page, total, visible int) []PaginationItem {
	total = max(total, 1)
	current := min(max(page, 1), total)
	if total <= 1 {
		return nil
	}
	visible = max(visible, minVisiblePages)
	pageItem := func(p int) PaginationItem { return PaginationItem{Kind: PaginationPage, Page: p} }
	var items []PaginationItem
	if total <= visible {
		for p := 1; p <= total; p++ {
			items = append(items, pageItem(p))
		}
		return items
	}
	side := (visible - 3) / 2
	start, end := current-side, current+side
	switch {
	case current <= side+1:
		start = 2
	case current > total-side-1:
		start = total - side - 1
	}
	switch {
	case current >= total-side:
		end = total - 1
	case current <= side+1:
		end = side + 2
	}
	items = append(items, pageItem(1))
	if start > 2 {
		items = append(items, PaginationItem{Kind: PaginationEllipsis, Page: 2, Start: 2, End: start})
	}
	for p := start; p <= end; p++ {
		items = append(items, pageItem(p))
	}
	if end < total-1 {
		items = append(items, PaginationItem{Kind: PaginationEllipsis, Page: end + 1, Start: end + 1, End: total})
	}
	return append(items, pageItem(total))
}

// Pagination is the pages of a set to move among, a navigation landmark:
// a row of Previous, the pages around the current one, ellipses for the
// runs left out, its children and Next; Compact keeps Previous, the
// children and Next. Page is the app's: a press reports the page it goes
// to with OnChange. It looks like nothing: the app styles the items
// (ItemClass, its selected: on the current page) or draws them
// (RenderItem). The row is made once, a place for each item it can hold,
// so a page change changes what the places show and makes no view.
type Pagination struct {
	Page       func() int
	TotalPages func() int
	// VisiblePages is the most pages shown at once, at least 5, the
	// default.
	VisiblePages int
	Compact      func() bool
	Disabled     func() bool
	// PreviousLabel and NextLabel are what assistive technology says for
	// Previous and Next, and their content without RenderItem, else ‹ and ›.
	PreviousLabel string
	NextLabel     string
	OnChange      func(page int)
	// RenderItem draws each item's content from its state.
	RenderItem func(item PaginationItem) core.Node
	ItemClass  string
	ItemStyle  core.Style
	// ItemStyleFunc is the items' style following what it reads.
	ItemStyleFunc func() core.Style
	Class         string
	Style         core.Style
	StyleFunc     func() core.Style
	Children      []core.Node
	Label         string
	TestID        string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (p Pagination) Render(cx *core.Scope) core.Node {
	total := core.NewMemo(cx, func() int { return max(p.TotalPages(), 1) })
	current := core.NewMemo(cx, func() int { return min(max(p.Page(), 1), total.Get()) })
	items := core.NewMemoFunc(cx, func() []PaginationItem {
		return paginationItems(current.Get(), total.Get(), p.VisiblePages)
	}, slices.Equal[[]PaginationItem])
	request := func(page int) {
		if is(p.Disabled) || page == current.GetUntracked() || page < 1 || page > total.GetUntracked() {
			return
		}
		if p.OnChange != nil {
			p.OnChange(page)
		}
	}
	style := core.Style{Direction: core.Row}.Merge(p.Style)
	children := []core.Node{p.step(cx, true, current, total, request)}
	for at := range max(p.VisiblePages, minVisiblePages) + 2 {
		children = append(children, p.place(cx, at, items, current, request)...)
	}
	children = append(children, p.Children...)
	children = append(children, p.step(cx, false, current, total, request))
	return core.View{Class: p.Class, Style: style, StyleFunc: p.StyleFunc, Role: core.RoleNavigation, Label: p.Label, TestID: p.TestID, OnLayout: p.OnLayout, Tooltip: p.Tooltip,
		Children: children}
}

// content is an item's content: RenderItem's while the item shows, else
// the default.
func (p Pagination) content(shown func() bool, state func() PaginationItem, def func() string) core.Node {
	if p.RenderItem == nil {
		return core.Text{ContentFunc: def}
	}
	return core.Dynamic(func(*core.Scope) core.Node {
		if shown != nil && !shown() {
			return nil
		}
		return p.RenderItem(state())
	})
}

// step is Previous or Next.
func (p Pagination) step(cx *core.Scope, previous bool, current, total *core.Memo[int], request func(int)) core.Node {
	atEnd := func() bool {
		if previous {
			return current.Get() <= 1
		}
		return current.Get() >= total.Get()
	}
	target := func() int {
		switch {
		case atEnd():
			return current.Get()
		case previous:
			return current.Get() - 1
		}
		return current.Get() + 1
	}
	disabled := func() bool { return is(p.Disabled) || atEnd() }
	kind, label, mark := PaginationNext, p.NextLabel, "›"
	if previous {
		kind, label, mark = PaginationPrevious, p.PreviousLabel, "‹"
	}
	if label != "" {
		mark = label
	}
	state := func() PaginationItem { return PaginationItem{Kind: kind, Page: target(), Disabled: disabled()} }
	return core.View{Role: core.RoleButton, Label: label, Disabled: disabled, Class: p.ItemClass, Style: p.ItemStyle, StyleFunc: p.ItemStyleFunc,
		OnPress: func(core.PressEvent) {
			if !core.Untracked(cx, disabled) {
				request(core.Untracked(cx, target))
			}
		},
		Children: []core.Node{p.content(nil, state, func() string { return mark })}}
}

// place is place at of the row: a page view and, past the first, which
// always holds page 1, an ellipsis view, of which it shows the item it
// holds, if any.
func (p Pagination) place(cx *core.Scope, at int, items *core.Memo[[]PaginationItem], current *core.Memo[int], request func(int)) []core.Node {
	item := core.NewMemo(cx, func() PaginationItem {
		all := items.Get()
		if at < len(all) {
			return all[at]
		}
		return PaginationItem{Kind: PaginationEllipsis}
	})
	number := func() int {
		if i := item.Get(); i.Kind == PaginationPage {
			return i.Page
		}
		return 0
	}
	pageShown := func() bool { return number() > 0 && !is(p.Compact) }
	isCurrent := func() bool { return number() > 0 && number() == current.Get() }
	hide := func(shown func() bool) func() core.Style {
		return func() core.Style {
			var s core.Style
			if p.ItemStyleFunc != nil {
				s = p.ItemStyleFunc()
			}
			if !shown() {
				s.Display = core.DisplayNone
			}
			return s
		}
	}
	state := func() PaginationItem {
		return PaginationItem{Kind: PaginationPage, Page: number(), Current: isCurrent(), Disabled: is(p.Disabled)}
	}
	page := core.View{Role: core.RoleButton, Disabled: p.Disabled, Selected: isCurrent, Class: p.ItemClass, Style: p.ItemStyle, StyleFunc: hide(pageShown),
		OnPress: func(core.PressEvent) {
			if n := core.Untracked(cx, number); n > 0 {
				request(n)
			}
		},
		Children: []core.Node{p.content(pageShown, state, func() string {
			if pageShown() {
				return strconv.Itoa(number())
			}
			return ""
		})}}
	if at == 0 {
		return []core.Node{page}
	}
	ellipsisShown := func() bool {
		i := item.Get()
		return i.Kind == PaginationEllipsis && i.Start > 0 && !is(p.Compact)
	}
	gap := func() PaginationItem {
		i := item.Get()
		i.Disabled = is(p.Disabled)
		return i
	}
	ellipsis := core.View{Class: p.ItemClass, Style: p.ItemStyle, StyleFunc: hide(ellipsisShown),
		Children: []core.Node{p.content(ellipsisShown, gap, func() string {
			if ellipsisShown() {
				return "…"
			}
			return ""
		})}}
	return []core.Node{page, ellipsis}
}
