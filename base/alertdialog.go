package base

import "github.com/oneo-me/nsgo/core"

// AlertDialog is the system's alert or action sheet: a title, a
// description and its actions, which the platform places and draws by
// their roles.
type AlertDialog struct {
	Title       string
	Description string
	Actions     []core.AlertAction
	// Presentation is an alert in the middle, or a sheet of actions from
	// the bottom (UIAlertController's action sheet, a list dialog on
	// Android, stacked buttons elsewhere).
	Presentation AlertPresentation
	// CloseOnEscape lets Escape and back close it without an action, and
	// CloseOnBackdropPress a press outside; both are off for an alert and
	// on for an action sheet when unset.
	CloseOnEscape, CloseOnBackdropPress core.Toggle
}

// AlertPresentation is how an AlertDialog presents itself.
type AlertPresentation uint8

const (
	// AlertPresentAlert is an alert in the middle.
	AlertPresentAlert AlertPresentation = iota
	// AlertPresentActionSheet is a sheet of actions from the bottom.
	AlertPresentActionSheet
)

// ShowAlertDialog shows d and calls done with the label of the action
// chosen, or "" when it was dismissed without one.
func ShowAlertDialog(cx *core.Scope, d AlertDialog, done func(label string)) {
	sheet := d.Presentation == AlertPresentActionSheet
	on := func(t core.Toggle) bool { return t == core.ToggleOn || t != core.ToggleOff && sheet }
	c := core.AlertConfig{Title: d.Title, Message: d.Description, Actions: d.Actions, Sheet: sheet,
		Cancelable: on(d.CloseOnEscape) || on(d.CloseOnBackdropPress)}
	cx.ShowAlert(c, func(i int) {
		if done == nil {
			return
		}
		if i < 0 || i >= len(d.Actions) {
			done("")
			return
		}
		done(d.Actions[i].Label)
	})
}
