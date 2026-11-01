package tsxtest

import (
	"os"
	"testing"
)

func TestRunsTestFiles(t *testing.T) { Run(t, os.DirFS("testdata/app")) }
