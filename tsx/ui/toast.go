package ui

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// ToastsRef is what the ref of Toasts gets.
type ToastsRef struct{ toaster *ui.Toaster }

// Show shows a toast and returns its ID.
func (r *ToastsRef) Show(c ui.ToastContent) string { return r.toaster.Show(c) }

// Dismiss closes the toast of id.
func (r *ToastsRef) Dismiss(id string) { r.toaster.Dismiss(id) }

// DismissAll closes every toast.
func (r *ToastsRef) DismissAll() { r.toaster.DismissAll() }

// Toasts is ui.Toasts with a toaster of its own, which its ref drives.
type Toasts struct {
	Placement  ui.ToastPlacement
	Reverse    bool
	Swipe      ui.ToastSwipe
	Inset      core.Points
	CloseLabel string
	Ref        *ToastsRef
	TestID     string
}

// Render implements core.Node.
func (t Toasts) Render(cx *core.Scope) core.Node {
	toaster := ui.NewToaster(cx)
	if t.Ref != nil {
		t.Ref.toaster = toaster
	}
	return ui.Toasts{Toaster: toaster, Placement: t.Placement, Reverse: t.Reverse, Swipe: t.Swipe, Inset: t.Inset, CloseLabel: t.CloseLabel, TestID: t.TestID}
}
