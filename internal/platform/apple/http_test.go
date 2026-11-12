//go:build darwin

package apple

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/platform/imageload/imageloadtest"
)

func TestHTTP(t *testing.T) { imageloadtest.CheckHTTP(t, httpGet) }
