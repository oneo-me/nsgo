package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// The demos of gallery-ui's own composed components, which Onexa copies:
// what the gallery shows of them beside base's.

// toggleCard is ToggleButton in a formatting toolbar.
func toggleCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		toggle := func(name string, icon ui.IconName, on bool) core.Node {
			s := core.NewSignal(cx, on)
			b := ui.ToggleButton{Text: name, TestID: "toggle-" + name, On: s.Get, OnChange: func(v bool) {
				s.Set(v)
				last.Set(fmt.Sprintf("%s: %s", name, onOff(tr, v)))
			}}
			if icon != "" {
				b.Text, b.Icon, b.Label = "", icon, name
			}
			return b
		}
		return ui.Card{Title: tr.LibFormatting(), Description: tr.LibFormattingNote(), Children: []core.Node{
			ui.Wrap{Children: []core.Node{
				toggle(tr.LibBold(), "", true), toggle(tr.LibItalic(), "", false), toggle(tr.LibUnderline(), "", false),
				toggle(tr.LibStarred(), ui.IconStar, false),
				ui.ToggleButton{Text: tr.LibLocked(), On: func() bool { return true }, Disabled: func() bool { return true }},
			}},
		}}
	}
}

// holdCard is HoldToConfirm in each tone, with a reset.
func holdCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		var resets []func()
		hold := func(text, done string, icon ui.IconName, tone ui.HoldTone, id string) core.Node {
			s := core.NewSignal(cx, false)
			resets = append(resets, func() { s.Set(false) })
			return ui.HoldToConfirm{Text: text, ConfirmedText: done, Icon: icon, Tone: tone, Hint: tr.LibHold(), TestID: id,
				Confirmed: s.Get, OnConfirm: func() { s.Set(true); last.Set(done) }}
		}
		holds := []core.Node{
			hold(tr.LibHoldToDelete(), tr.LibDeleted(), "", ui.HoldDanger, "hold-delete"),
			hold(tr.LibHoldToArchive(), tr.LibArchived(), ui.IconFile, ui.HoldAccent, "hold-archive"),
			hold(tr.LibHoldToSend(), tr.LibSent(), ui.IconSend, ui.HoldNeutral, "hold-send"),
		}
		return ui.Card{Title: tr.LibHold(), Description: tr.LibHoldNote(), Children: []core.Node{
			ui.Wrap{Children: holds},
			ui.HStack{Children: []core.Node{ui.Button{Text: tr.ControlsReset(), Variant: ui.ButtonGhost, TestID: "hold-reset", OnPress: func() {
				for _, r := range resets {
					r()
				}
			}}}},
		}}
	}
}

// stepStatus says a step's status in the gallery's language.
func stepStatus(tr locales.Messages) func(ui.StepStatus) string {
	return func(s ui.StepStatus) string {
		switch s {
		case ui.StepCurrent:
			return tr.LibStepCurrent()
		case ui.StepComplete:
			return tr.LibStepComplete()
		case ui.StepFailed:
			return tr.LibStepFailed()
		}
		return tr.LibStepUpcoming()
	}
}

// checkoutSteps are a checkout's steps.
func checkoutSteps(tr locales.Messages) []ui.StepperStep {
	return []ui.StepperStep{{Label: tr.LibCart(), Description: tr.LibCartDetail()}, {Label: tr.LibShipping(), Description: tr.LibShippingDetail()},
		{Label: tr.LibPayment(), Description: tr.LibPaymentDetail()}, {Label: tr.LibReview(), Description: tr.LibReviewDetail()}}
}

// stepperCards are Stepper through a checkout, in a row and compact,
// moved by its buttons or a press on a completed step, and down a
// column with a failed step.
func stepperCards(last *core.Signal[string], step *core.Signal[int]) []cardFunc {
	checkout := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		steps := checkoutSteps(tr)
		move := func(by int) func() {
			return func() {
				n := min(max(step.GetUntracked()+by, 0), len(steps))
				step.Set(n)
				if n < len(steps) {
					last.Set(steps[n].Label)
				} else {
					last.Set(tr.LibOrderPlaced())
				}
			}
		}
		return ui.Card{Title: tr.LibCheckout(), Description: tr.LibCheckoutNote(), Children: []core.Node{
			ui.Stepper{TestID: "checkout", Label: tr.LibCheckout(), Steps: steps, Current: step.Get, StatusText: stepStatus(tr),
				OnSelect: func(i int) { step.Set(i); last.Set(steps[i].Label) }},
			ui.HStack{Gap: 8, Children: []core.Node{
				ui.Button{Text: tr.LibBack(), Variant: ui.ButtonGhost, TestID: "stepper-back", Disabled: func() bool { return step.Get() == 0 }, OnPress: move(-1)},
				ui.Button{Text: tr.LibNext(), TestID: "stepper-next", Disabled: func() bool { return step.Get() == len(steps) }, OnPress: move(1)},
			}},
		}}
	}
	compact := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Card{Title: tr.LibCompact(), Description: tr.LibCompactNote(), Children: []core.Node{
			ui.Stepper{Label: tr.LibCheckout(), Steps: checkoutSteps(tr), Current: step.Get, Compact: true,
				CompleteLabel: tr.LibOrderPlaced(), StatusText: stepStatus(tr)},
		}}
	}
	setup := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		steps := []ui.StepperStep{{Label: tr.LibCreateAccount()}, {Label: tr.LibConnectRepository(), Description: tr.LibConnectRepositoryDetail()},
			{Label: tr.LibInstall(), Error: tr.LibInstallError()}, {Label: tr.LibDeploy(), Description: tr.LibDeployDetail()}}
		return ui.Card{Title: tr.LibSetup(), Description: tr.LibSetupNote(), Children: []core.Node{
			ui.Stepper{Label: tr.LibSetup(), Steps: steps, Current: func() int { return 2 }, Vertical: true, CurrentDescription: true,
				StatusText: stepStatus(tr)},
		}}
	}
	return []cardFunc{checkout, setup, compact}
}

// stepperDemo is the Stepper cards over the last step chosen; the row and
// the compact markers share the checkout's step.
func stepperDemo(cx *core.Scope) core.Node {
	last := core.NewSignal(cx, "—")
	return demoPage(cx, pageProps{}, append(stepperCards(last, core.NewSignal(cx, 1)), lastEventCard(last))...)
}

// fileIconCard is FileIcon beside a project's files.
func fileIconCard(*core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		var files []core.Node
		for _, name := range []string{"go.mod", "go.sum", "main.go", "main_test.go", "README.md", "LICENSE", "package.json", "App.tsx",
			"index.ts", "style.css", "build.sh", "nsgo.yaml", "logo.svg", "photo.png", ".gitignore", ".env.local", "Dockerfile", "notes"} {
			files = append(files, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Width: core.Pt(132)},
				Children: []core.Node{ui.FileIcon{Name: name}, ui.Text(name, 13, ui.Regular, colors.Text)}})
		}
		return ui.Card{Title: tr.LibFiles(), Description: tr.LibFilesNote(), Children: []core.Node{ui.Wrap{Children: files}}}
	}
}

// styledToastsCard is gallery-ui's Toasts, beside the gallery's own: a
// toaster of its own, stacked at the top of the window.
func styledToastsCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		toaster := ui.NewToaster(cx)
		kinds := []struct {
			name, title string
			kind        ui.ToastType
		}{{tr.LibPlain(), tr.LibMessageSent(), ui.ToastPlain}, {tr.LibInfo(), tr.LibNewVersion(), ui.ToastInfo},
			{tr.LibSuccess(), tr.LibChangesSaved(), ui.ToastSuccess}, {tr.LibWarning(), tr.LibStorageFull(), ui.ToastWarning},
			{tr.LibError(), tr.LibUploadFailed(), ui.ToastError}}
		var buttons []core.Node
		for i, k := range kinds {
			buttons = append(buttons, ui.Button{Text: k.name, Variant: ui.ButtonSecondary, TestID: fmt.Sprintf("styled-toast-%d", i),
				OnPress: func() { toaster.Show(ui.ToastContent{Type: k.kind, Title: k.title}) }})
		}
		buttons = append(buttons,
			ui.Button{Text: tr.LibWithAction(), Variant: ui.ButtonSecondary, TestID: "styled-toast-action", OnPress: func() {
				toaster.Show(ui.ToastContent{Type: ui.ToastSuccess, Title: tr.LibMovedToTrash(), Description: tr.LibThreeFiles(), Action: tr.OverlayUndo(),
					OnAction: func() {
						last.Set(tr.LibRestored())
						toaster.Show(ui.ToastContent{Title: tr.LibRestored()})
					}})
			}},
			ui.Button{Text: tr.OverlayDismissAll(), Variant: ui.ButtonGhost, OnPress: toaster.DismissAll})
		return ui.Card{Title: tr.LibToasts(), Description: tr.LibToastsNote(), Children: []core.Node{
			ui.Wrap{Children: buttons},
			ui.Toasts{Toaster: toaster, CloseLabel: tr.ToastClose(), TestID: "styled-toasts"},
		}}
	}
}
