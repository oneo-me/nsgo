//go:build windows

package windows

import (
	"testing"

	"github.com/oneo-me/nsgo/internal/platform/imageload/imageloadtest"
)

func TestHTTP(t *testing.T) { imageloadtest.CheckHTTP(t, httpGet) }
