//go:build windows

package windows

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/platform/imageload/imageloadtest"
)

func TestHTTP(t *testing.T) { imageloadtest.CheckHTTP(t, httpGet) }
