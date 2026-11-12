package wgsl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/shader/wgsl"
	"github.com/nsgo-dev/nsgo/internal/shaderir/glsl"
	"github.com/nsgo-dev/nsgo/internal/shaderir/hlsl"
	"github.com/nsgo-dev/nsgo/internal/shaderir/msl"
)

// testdata holds shaders that use what WGSL has, which compile into
// every language; the Apple backend's tests compile them with Metal.
func programs(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "*.wgsl"))
	if err != nil || len(files) == 0 {
		t.Fatal("no shaders in testdata", err)
	}
	out := map[string]string{"transition": wgsl.Crossfade}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[strings.TrimSuffix(filepath.Base(f), ".wgsl")] = string(b)
	}
	return out
}

func TestCompile(t *testing.T) {
	for name, src := range programs(t) {
		p, err := wgsl.Compile(src)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for lang, out := range map[string]string{
			"msl":  msl.Fragment(p.IR, "f"),
			"glsl": glsl.Fragment(p.IR, "f", glsl.Options{Block: "U"}),
			"hlsl": hlsl.Fragment(p.IR, "f", 0, 1),
		} {
			if strings.Contains(out, "?(") {
				t.Errorf("%s: %s has what it cannot write:\n%s", name, lang, out)
			}
		}
	}
}

// A program reads the pictures and the time shade reaches, through the
// functions it calls.
func TestInputs(t *testing.T) {
	programs := programs(t)
	for _, c := range []struct {
		src      string
		inputs   wgsl.Inputs
		textures int
		animated bool
	}{
		{programs["fill"], 0, 0, false},
		{programs["uniforms"], 0, 0, true},
		{programs["pictures"], wgsl.Content | wgsl.Backdrop, 2, true},
		{programs["transition"], wgsl.From | wgsl.To, 4, false},
		{`fn unused() -> vec4f { return sample_to(vec2f(0.0)) * nsh.time; }
fn helper(uv: vec2f) -> vec4f { return sample_from(uv); }
fn shade(uv: vec2f) -> vec4f { return helper(uv); }`, wgsl.From, 3, false},
	} {
		p, err := wgsl.Compile(c.src)
		if err != nil {
			t.Fatal(err)
		}
		if p.Inputs != c.inputs || p.IR.TextureCount != c.textures || p.Animated != c.animated {
			t.Errorf("%q: inputs %b, textures %d, animated %v; want %b, %d, %v", c.src, p.Inputs, p.IR.TextureCount, p.Animated, c.inputs, c.textures, c.animated)
		}
	}
}

// The uniforms follow the bridge's and the built-ins' in the IR.
func TestUniforms(t *testing.T) {
	programs := programs(t)
	p, err := wgsl.Compile(programs["uniforms"])
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"__rectSize", "__scale", "__nsh", "tint", "glow", "speed", "steps", "seed", "stops", "at"}
	if strings.Join(p.IR.UniformNames, " ") != strings.Join(want, " ") {
		t.Errorf("uniforms %v, want %v", p.IR.UniformNames, want)
	}
	var names []string
	for _, u := range p.Uniforms {
		names = append(names, u.Name)
	}
	if strings.Join(names, " ") != strings.Join(want[3:], " ") || !p.Uniforms[1].Vec3 || p.Uniforms[5].Lanes != 12 {
		t.Errorf("members %+v", p.Uniforms)
	}
	pic, err := wgsl.Compile(programs["pictures"])
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(pic.IR.UniformNames, " "); got != "__rectSize __scale __imageSizes __nsh" || pic.IR.Uniforms[2].Length != 2 {
		t.Errorf("a program reading pictures has uniforms %s", got)
	}
}

// Errors give the place in the app's source.
func TestErrors(t *testing.T) {
	for _, c := range []struct {
		src, want string
	}{
		{"fn shade(uv: vec2f) -> vec4f {\n  return vec3f(1.0);\n}", "2:10: cannot use a vec3<f32> as a vec4<f32>"},
		{"fn shade(uv: vec2f) -> vec4f {\n  return vec4f(missing);\n}", "2:16: missing is not declared"},
		{"fn shade(x: f32) -> vec4f { return vec4f(x); }", "1:4: shade must be fn shade(uv: vec2f) -> vec4f"},
		{"fn other() {}", "shader: the shader defines no fn shade"},
		{"@fragment fn shade(uv: vec2f) -> vec4f { return vec4f(1.0); }", "1:1: a shader declares no entry points"},
		{"@group(0) @binding(3) var t: texture_2d<f32>;\nfn shade(uv: vec2f) -> vec4f { return vec4f(1.0); }", "1:1: a shader declares no resources"},
		{"struct Uniforms { m: mat4x4f }\nfn shade(uv: vec2f) -> vec4f { return vec4f(1.0); }", "1:19: a uniform is f32"},
		{"struct Uniforms { a: array<vec4f, 300> }\nfn shade(uv: vec2f) -> vec4f { return vec4f(1.0); }", "1:8: the uniforms take 4800 bytes, more than 4096"},
		{"fn shade(uv: vec2f) -> vec4f {\n  let x = 1;\n  x = 2;\n  return vec4f(1.0);\n}", "3:3: the value cannot be assigned"},
		{"fn shade(uv: vec2f) -> vec4f {\n  if (uv.x > 0.5) { return vec4f(1.0); }\n}", "1:4: shade does not return a value on every path"},
		{"fn a(x: f32) -> f32 { return b(x); }\nfn b(x: f32) -> f32 { return a(x); }\nfn shade(uv: vec2f) -> vec4f { return vec4f(a(uv.x)); }", "calls itself"},
		{"fn shade(uv: vec2f) -> vec4f { return vec4f(1.0) /* open", "the block comment is not closed"},
		{"fn shade(uv: vec2f) -> vec4f { let i = 3000000000i; return vec4f(1.0); }", "does not fit a i32"},
		{"fn shade(uv: vec2f) -> vec4f { return vec4f(uv.xyz, 1.0); }", "1:48: a vec2<f32> has no member xyz"},
		{"var<private> g: f32;\nfn shade(uv: vec2f) -> vec4f { return vec4f(g); }", "1:14: var<private> is not supported yet"},
		{"fn shade(uv: vec2f) -> vec4f { return nsh.depth; }", "nsh has no member depth"},
		{"fn shade(uv: vec2f) -> vec4f { return __texel(0, uv); }", "__texel is not declared"},
		{"fn shade(uv: vec2f) -> vec4f { return vec4f(uniforms.x); }", "uniforms needs struct Uniforms"},
	} {
		_, err := wgsl.Compile(c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: got %v, want %q", c.src, err, c.want)
		}
	}
	if _, err := wgsl.Compile(strings.Repeat(" ", wgsl.MaxSourceBytes+1)); err == nil {
		t.Error("a source over the limit compiled")
	}
}

// Constants fold as WGSL's constant expressions do.
func TestConstants(t *testing.T) {
	for _, src := range []string{
		"const A = 7 / 2; const_assert A == 3;",
		"const B = -7 % 3; const_assert B == -1;",
		"const C = 1u << 31u; const_assert C == 2147483648u;",
		"const D = 1.5 % 1.0; const_assert D == 0.5;",
		"const E = i32(2.9); const_assert E == 2;",
		"const F = vec3(1, 2, 3).z + 1; const_assert F == 4;",
		"const G = ~0u; const_assert G == 4294967295u;",
	} {
		if _, err := wgsl.Compile(src + "\nfn shade(uv: vec2f) -> vec4f { return vec4f(1.0); }"); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
	if _, err := wgsl.Compile("const_assert 1 == 2;\nfn shade(uv: vec2f) -> vec4f { return vec4f(1.0); }"); err == nil {
		t.Error("a false assertion compiled")
	}
}
