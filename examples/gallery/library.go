package main

import (
	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
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

// constant is s as text that may change.
func constant(s string) func() string { return func() string { return s } }
