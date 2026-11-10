//go:build darwin

package apple

import (
	"testing"

	"github.com/oneo-me/nsgo/internal/platform/imageload/imageloadtest"
)

func TestHTTP(t *testing.T) { imageloadtest.CheckHTTP(t, httpGet) }
