//go:build !nsgo_noshader

package core

import (
	"testing"

	"github.com/oneo-me/nsgo/internal/shader/wgsl"
)

// TestUncompiledBuiltinLanes keeps shader_nocompile.go's count of the
// built-in lanes, which cannot import wgsl, the compiler's.
func TestUncompiledBuiltinLanes(t *testing.T) {
	if wgsl.BuiltinLanes != 3 {
		t.Fatalf("wgsl.BuiltinLanes = %d; set builtinLanes in shader_nocompile.go to it", wgsl.BuiltinLanes)
	}
}
