package chat

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui/chat/locales"
)

// Actions are what a message offers a reader: copy with a receipt, retry a
// reply, and edit a message the user sent. The app says where each applies
// and own its own actions may follow; the words are ui/chat's, through its
// catalog.
type Actions struct {
	// Text is what Copy copies.
	Text func() string
	// OnCopy replaces copying Text, for a message that is not all prose; the
	// receipt shows either way.
	OnCopy func()
	// OnRetry shows a retry, which CanRetry keeps from a reply the app may
	// not ask for again; nil CanRetry leaves it always available.
	OnRetry  func()
	CanRetry func() bool
	// OnEdit shows an edit, which CanEdit keeps from a message something has
	// superseded; nil CanEdit leaves it always available.
	OnEdit  func()
	CanEdit func() bool
	// Receipt is how long the copied receipt shows, 1600 ms when 0; pressing
	// copy again restarts it.
	Receipt time.Duration
	// Children are the app's own actions, after these.
	Children []core.Node
	TestID   string
}

// Render implements core.Node.
func (a Actions) Render(cx *core.Scope) core.Node {
	colors := tonesOf(cx)
	copied := core.NewSignal(cx, false)
	// gen counts copies, so a receipt that a later copy replaced hides
	// nothing.
	gen := core.NewSignal(cx, 0)
	receipt := a.Receipt
	if receipt <= 0 {
		receipt = 1600 * time.Millisecond
	}
	copies := func() {
		if a.OnCopy != nil {
			a.OnCopy()
		} else if a.Text != nil {
			cx.SetClipboard(a.Text())
		}
		copied.Set(true)
		gen.Update(func(n int) int { return n + 1 })
		n := gen.GetUntracked()
		cx.Post(func() {
			cx.AfterFunc(receipt, func() {
				if gen.GetUntracked() == n {
					copied.Set(false)
				}
			})
		})
	}
	allowed := func(fn func() bool) func() bool {
		return func() bool { return fn == nil || fn() }
	}
	var children []core.Node
	if a.Text != nil || a.OnCopy != nil {
		children = append(children, core.Dynamic(func(cx *core.Scope) core.Node {
			if copied.Get() {
				return IconAction{Label: locales.Copied(cx), Icon: lucide.Check, Active: func() bool { return true },
					Tint: colors.Accent, OnPress: copies}
			}
			return IconAction{Label: locales.Copy(cx), Icon: lucide.Copy, Tint: colors.Accent, OnPress: copies}
		}))
	}
	if a.OnRetry != nil {
		children = append(children, IconAction{Label: locales.Retry(cx), Icon: lucide.RefreshCw,
			Disabled: func() bool { return !allowed(a.CanRetry)() }, OnPress: a.OnRetry})
	}
	if a.OnEdit != nil {
		children = append(children, IconAction{Label: locales.Edit(cx), Icon: lucide.Pencil,
			Disabled: func() bool { return !allowed(a.CanEdit)() }, OnPress: a.OnEdit})
	}
	children = append(children, a.Children...)
	return core.View{TestID: a.TestID, Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 4}, Children: children}
}
