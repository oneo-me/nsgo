package shader_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/shader"
	"github.com/nsgo-dev/nsgo/internal/shaderir/glsl"
	"github.com/nsgo-dev/nsgo/internal/shaderir/hlsl"
	"github.com/nsgo-dev/nsgo/internal/shaderir/msl"
)

// The backends write a shader as a function for the platforms to call,
// as the files beside testdata/fragment/fragment.go hold, which
// NSGO_UPDATE_SHADERS writes anew.
func TestFragment(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "fragment", "fragment.go"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := shader.CompileFragment(src)
	if err != nil {
		t.Fatal(err)
	}
	for ext, got := range map[string]string{
		"metal": msl.Fragment(p, "shaderFragment"),
		"glsl":  glsl.Fragment(p, "shaderFragment", glsl.Options{FlipY: "flipY", FlipRows: "flipRowsOf"}),
		"hlsl":  hlsl.Fragment(p, "shaderFragment", 2, 6),
	} {
		name := filepath.Join("testdata", "fragment", "fragment.expected."+ext)
		if os.Getenv("NSGO_UPDATE_SHADERS") != "" {
			if err := os.WriteFile(name, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Errorf("%s: got\n%s", name, got)
		}
		// A uint loop steps by a uint, and HLSL makes a vector of one scalar
		// by a cast, as it does not spread one.
		if !strings.Contains(got, "+= 2u") {
			t.Errorf("%s: the uint loop does not step by 2u", name)
		}
		if ext == "hlsl" && !strings.Contains(got, "(uint3)(") {
			t.Errorf("%s: uvec3 of a scalar is not a cast", name)
		}
	}
}

// Shaders read uints and ints as two kinds of integer, which mix only in
// shifts, and constants must fit theirs.
func TestUint(t *testing.T) {
	for _, c := range []struct {
		body, err string
	}{
		{"var x uint = 0xFFFFFFFF; _ = x", ""},
		{"var a uint; x := a * 2654435769; _ = x", ""},
		{"x := uint(3) * 2654435769; _ = x", "overflows uint"},
		{"var x uint = 1 << 32; _ = x", "overflows uint"},
		{"x := uint(-1); _ = x", "cannot convert"},
		{"var a int; var b uint; _ = a + b", "types don't match"},
		{"var a uint; _ = a << 3; _ = a >> int(dst.x); _ = 1 << a", ""},
		{"var v uvec2; v.x ^= 7; v %= 3; _ = v.yx + uvec2(dst.xy)", ""},
		{"x := int(2654435769); _ = x", "overflows int"},
		{"_ = max(uint(1), 2, 3); _ = clamp(uvec2(1), uint(0), 4)", ""},
		{"x := ^uint(0); y := ^N; _, _ = x, y", ""},
		{"x := -uint(1); _ = x", "overflows uint"},
		{"i := 5; var u uint = 1; i >>= u; i <<= N; _ = i", ""},
		{"s := uint(0); for i := uint(0); i < 3000000000; i++ { s += i }; _ = s", ""},
		{"s := uint(0); for i := uint(4000000000); i > 3000000000; i -= 3000000000 { s += i }; _ = s", ""},
		{"s := uint(0); for i := uint(0); i < 5000000000; i++ { s += i }; _ = s", "overflows uint"},
		{"var n uint; s := uint(0); for i := range n { s += i }; for i := range N { s += i }; _ = s", ""},
		{"var a [4]float; var u uint; _ = a[u]", ""},
		{"var m int; var x uint = 1 << m; y := N & (1 << m); x |= 1 << m; _, _ = x, y", ""},
		{"var m int; var f float = 1 << m; _ = f", "cannot use type int as type float"},
		{"var m int; x := 1 << m; var u uint; _ = x + u", "types don't match"},
	} {
		src := "package main\n\nconst N uint = 7\n\nfunc Fragment(dst vec4, pos vec2) vec4 {\n\t" + c.body + "\n\treturn dst\n}\n"
		_, err := shader.CompileFragment([]byte(src))
		switch {
		case c.err == "" && err != nil:
			t.Errorf("%s: %v", c.body, err)
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%s: %v, want an error saying %q", c.body, err, c.err)
		}
	}
}

// Integer constants are written as uints exactly where they are: a shift
// of an untyped constant by a count that is not one takes the type of its
// context, as in Go, and int where it has none.
func TestUntypedShift(t *testing.T) {
	src := `package main

const N uint = 7

func top(m int) uint {
	return 1 << m
}

func Fragment(dst vec4, pos vec2) vec4 {
	m := int(pos.x)
	var x uint = 1 << m
	y := N & ^(1 << m)
	i := 1 << m
	return vec4(float(x), float(y), float(i), float(top(m)))
}
`
	p, err := shader.CompileFragment([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := glsl.Fragment(p, "shaderFragment", glsl.Options{FlipY: "flipY"})
	for _, want := range []string{
		"return (1u) << (l0);",
		" = (1u) << (l2);",
		" = (7u) & (~((1u) << (l2)));",
		" = (1) << (l2);",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
}

// Redeclaring a function the bridge declares for shaders is an error at
// the shader's declaration, not in the bridge after it.
func TestRedeclaredBridge(t *testing.T) {
	src := "package main\n\nfunc rectSize() vec2 {\n\treturn vec2(0)\n}\n\nfunc Fragment(dst vec4, pos vec2) vec4 {\n\treturn dst\n}\n"
	_, err := shader.CompileFragment([]byte(src))
	if err == nil || !strings.HasPrefix(err.Error(), "3:6: rectSize redeclared") {
		t.Errorf("%v, want an error at 3:6", err)
	}
}

// A shader needs its entry point.
func TestNoEntry(t *testing.T) {
	if _, err := shader.CompileFragment([]byte("package main\n\nfunc main() {}\n")); err == nil {
		t.Error("a shader without Fragment compiled")
	}
}
