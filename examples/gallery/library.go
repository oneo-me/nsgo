package main

import (
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// What the demos of ui's composed components share.

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

// constant is s as text that may change.
func constant(s string) func() string { return func() string { return s } }
