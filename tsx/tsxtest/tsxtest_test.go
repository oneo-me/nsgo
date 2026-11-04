package tsxtest

import (
	"os"
	"testing"
)

func TestRunsTestFiles(t *testing.T) { Run(t, os.DirFS("testdata/app")) }

func TestRunsSteps(t *testing.T) {
	RunSteps(t, os.DirFS("testdata/app"), "App.tsx", `await app.press("id:count");
expect(app.shows("Pressed 1")).toBe(true);`)
}
