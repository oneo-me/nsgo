package base

import (
	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// Dialog is a modal layer over the app: an optional backdrop and a
// popup centered on it.
type Dialog struct {
	Open         func() bool
	DefaultOpen  bool
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	OnCancel     func()

	Animation              core.ModalAnimation
	NoCloseOnEscape        bool
	NoCloseOnBackdropPress bool
	// Presentation and NoCloseOnSwipe are iOS's, IOS and Android hold the
	// props only one of them has, and StatusBar is set over the app's
	// while the dialog is open, as core.Modal's are.
	Presentation   core.ModalPresentation
	NoCloseOnSwipe bool
	IOS            core.IOSModalProps
	Android        core.AndroidModalProps
	StatusBar      core.StatusBarSetting
	StatusBarFunc  func() core.StatusBarSetting
	// BackdropClass, BackdropStyle or BackdropStyleFunc, when set, draws a
	// backdrop whose press asks to close; BackdropTestID names it.
	BackdropClass     string
	BackdropStyle     core.Style
	BackdropStyleFunc func() core.Style
	BackdropTestID    string
	PopupClass        string
	PopupStyle        core.Style
	PopupStyleFunc    func() core.Style
	// LayerClass and LayerStyle style the layer that holds the backdrop
	// and the popup, over its default, which centers the popup.
	LayerClass string
	LayerStyle core.Style
	Children   []core.Node
	Label      string
	TestID     string
}

// Render implements core.Node.
func (d Dialog) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, d.Open, d.DefaultOpen, d.OnOpenChange, d.OnOpen, d.OnClose)
	st.onCancel = d.OnCancel
	if d.Open != nil {
		// Closed by the user, it stays closed while Open stays true, until
		// the app sets it false and true again.
		st.userClosed = core.NewSignal(cx, false)
		cx.Effect(func() {
			if !d.Open() {
				st.userClosed.Set(false)
			}
		})
		st.open = func() bool { return d.Open() && !st.userClosed.Get() }
	}
	// The platform presents the modal: OnOpen and OnClose follow once its
	// animation would have ended, or as soon as a platform reports it
	// shown or gone.
	var stop func() bool
	report := func(open bool) func() {
		return func() {
			if stop != nil {
				stop()
				stop = nil
			}
			if f := map[bool]func(){true: d.OnOpen, false: d.OnClose}[open]; f != nil {
				cx.Untrack(f)
			}
		}
	}
	was := false
	cx.Effect(func() {
		open := st.open()
		if open == was {
			return
		}
		was = open
		if stop != nil {
			stop()
			stop = nil
		}
		if wait := d.Animation.Duration(); wait > 0 {
			stop = cx.AfterFunc(wait, report(open))
			return
		}
		report(open)()
	})
	m := core.Modal{Animation: d.Animation, Presentation: d.Presentation, NoCloseOnSwipe: d.NoCloseOnSwipe,
		NoCloseOnEscape: d.NoCloseOnEscape, NoCloseOnBackdropPress: d.NoCloseOnBackdropPress,
		IOS: d.IOS, Android: d.Android, StatusBar: d.StatusBar, StatusBarFunc: d.StatusBarFunc, Class: d.LayerClass,
		OnShown: report(true), OnDismissed: report(false),
		Style: core.Style{JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter}.Merge(d.LayerStyle)}
	return modalLayer(st, m, parts.Of(d.BackdropClass, d.BackdropStyle, d.BackdropStyleFunc), d.BackdropTestID, d.NoCloseOnBackdropPress,
		d.NoCloseOnEscape, parts.Of(d.PopupClass, d.PopupStyle, d.PopupStyleFunc), d.Children, d.Label, d.TestID)
}
