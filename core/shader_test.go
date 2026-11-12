package core

import (
	"slices"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

const testShader = `package main

var Time float
var Tint vec3
var Spots [2]vec4
var Count int

func Fragment(dst vec4, pos vec2) vec4 {
	a := 0.5 + 0.5*sin(Time+pos.x/rectSize().x)
	for i := 0; i < Count; i++ {
		a += Spots[i].w
	}
	return vec4(Tint*a, a)
}
`

func shaderItem(b *driver.MountBatch) *driver.Shader {
	for _, it := range b.Items {
		if it.Kind == driver.ItemShader {
			return it.Shader
		}
	}
	return nil
}

func TestShaderSendsLanesAndNodes(t *testing.T) {
	prog := MustCompileShader(testShader)
	var count *Signal[int]
	var clock *AnimatedValue
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		count = NewSignal(cx, 1)
		clock = NewAnimatedValue(cx, 0)
		return Shader{Program: prog, Style: Style{Height: Pt(40)},
			Uniforms: Uniforms{"Tint": RGB(0xff8000), "Time": clock},
			UniformsFunc: func() Uniforms {
				return Uniforms{"Count": count.Get(), "Spots": []any{1, 2, clock, 0.5}}
			}}
	})
	sh := shaderItem(h.commit())
	if sh == nil || sh.Program != prog.p {
		t.Fatalf("no shader item: %+v", sh)
	}
	// Time, Tint (3 lanes), Spots (8), Count.
	want := []float64{0, 1, 128.0 / 255, 0, 1, 2, 0, 0.5, 0, 0, 0, 0, 1}
	if !slices.Equal(sh.Lanes, want) {
		t.Errorf("lanes = %v, want %v", sh.Lanes, want)
	}
	if nodes := []driver.ShaderNode{{Lane: 0, Node: clock.id}, {Lane: 6, Node: clock.id}}; !slices.Equal(sh.Nodes, nodes) {
		t.Errorf("nodes = %v, want %v", sh.Nodes, nodes)
	}
	if sh := shaderItem(h.commit()); sh != nil {
		t.Error("an unchanged shader was sent again")
	}
	count.Set(2)
	if sh := shaderItem(h.commit()); sh == nil || sh.Lanes[12] != 2 {
		t.Errorf("a change sent %+v", sh)
	}
}

func TestShaderRejectsUnknownUniformsAndBadSources(t *testing.T) {
	if _, err := CompileShader("package main\n\nfunc Fragment(dst vec4, pos vec2) vec4 {\n\treturn undefined\n}\n"); err == nil ||
		!strings.Contains(err.Error(), "4:") {
		t.Errorf("err = %v, want one at line 4", err)
	}
	prog := MustCompileShader(testShader)
	for _, u := range []Uniforms{{"Missing": 1}, {"Time": []float64{1, 2}}, {"Tint": "red"}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%v did not panic", u)
				}
			}()
			prog.values(u, nil)
		}()
	}
}

// A uniform UniformsFunc sets replaces the static value whole, its node
// binding too.
func TestShaderUniformsFuncReplacesAUniform(t *testing.T) {
	prog := MustCompileShader(testShader)
	var clock *AnimatedValue
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		clock = NewAnimatedValue(cx, 0)
		return Shader{Program: prog, Uniforms: Uniforms{"Time": clock, "Spots": []any{clock, clock}},
			UniformsFunc: func() Uniforms { return Uniforms{"Time": 3, "Spots": []float64{1}} }}
	})
	sh := shaderItem(h.commit())
	if sh == nil || sh.Lanes[0] != 3 || sh.Lanes[4] != 1 || sh.Lanes[5] != 0 || len(sh.Nodes) != 0 {
		t.Fatalf("shader = %+v, want Time 3 and Spots 1 with no nodes", sh)
	}
}
