package tsx

import (
	"time"

	"github.com/oneo-me/nsgo/tsx/internal/js"

	"github.com/oneo-me/nsgo/core"
)

// Shaders (NSH's shader host elements): Shader fills its box with a WGSL
// program, ShaderEffect and ShaderTransition draw their children through
// one, ScreenEffect and ScreenTransition the surface, each a face over
// core's element of the same name. A page's and a dialog's
// transitionShader is decoded here too.

func init() {
	hostFaces["shader"] = shaderFace
	hostFaces["shadereffect"] = shaderEffectFace
	hostFaces["shadertransition"] = shaderTransitionFace
	hostFaces["screeneffect"] = screenEffectFace
	hostFaces["screentransition"] = screenTransitionFace
	moduleTable["nsgo"] = append(moduleTable["nsgo"],
		host("Shader", "shader"), host("ShaderEffect", "shadereffect"), host("ShaderTransition", "shadertransition"),
		host("ScreenEffect", "screeneffect"), host("ScreenTransition", "screentransition"))
}

// program compiles a source, or logs why it does not compile and gives
// nil, which shows no shader.
func (r *runtime) program(src string) *core.ShaderProgram {
	if src == "" {
		return nil
	}
	p, err := core.CompileWGSL(src)
	if err != nil {
		r.warn("%v", err)
		return nil
	}
	return p
}

// uniforms decodes a uniforms object for p: numbers, booleans, CSS
// colors, animated nodes and lists of them two levels deep. A name the
// program lacks is left out, once logged.
func (r *runtime) uniforms(p *core.ShaderProgram, v js.Value) core.Uniforms {
	o, ok := v.(*js.Object)
	if !ok || p == nil {
		return nil
	}
	u := core.Uniforms{}
	for _, k := range o.Keys() {
		if !p.HasUniform(k) {
			r.warn("the shader has no uniform %s", k)
			continue
		}
		if x := r.uniform(o.Get(k), 0); x != nil {
			u[k] = x
		}
	}
	return u
}

func (r *runtime) uniform(v js.Value, depth int) any {
	if !defined(v) {
		return nil
	}
	if n := r.animatedNodeOf(v); n != nil {
		return n
	}
	if o, ok := v.(*js.Object); ok && depth < 2 && o.ClassName() == "Array" {
		var out []any
		for _, k := range o.Keys() {
			if x := r.uniform(o.Get(k), depth+1); x != nil {
				out = append(out, x)
			}
		}
		return out
	}
	switch x := v.Export().(type) {
	case string:
		c, err := core.ParseColor(x)
		if err != nil {
			r.warn("the uniform's color %q: %v", x, err)
			return nil
		}
		return c
	case bool:
		return x
	}
	return v.ToFloat()
}

// progress is a progress prop: an animated node, or a number it follows.
func (p *props) progress() (core.AnimatedNode, func() float64) {
	if n := p.r.animatedNodeOf(p.peek("progress")); n != nil {
		return n, nil
	}
	if !p.has("progress") {
		return nil, nil
	}
	return nil, func() float64 { return p.float("progress") }
}

// transitionValue is a transition's value prop: a string, a number or a
// boolean, compared as they are.
func (p *props) transitionValue() func() any {
	return func() any {
		v := p.get("value")
		if !defined(v) {
			return nil
		}
		return v.Export()
	}
}

func (p *props) duration(name string) time.Duration {
	return time.Duration(p.float(name) * float64(time.Millisecond))
}

func shaderFace(p *props) core.Node {
	prog := p.r.program(p.str("source"))
	return core.Shader{
		Program:      prog,
		UniformsFunc: func() core.Uniforms { return p.r.uniforms(prog, p.get("uniforms")) },
		PausedFunc:   func() bool { return p.bool("paused") },
		Class:        p.class(),
		StyleFunc:    p.styleFunc(),
		Children:     p.children(),
		OnPress:      p.onPress("onPress"),
		OnLayout:     p.onLayout(),
		Ref:          p.n.ref,
		Label:        p.str("accessibilityLabel"),
		TestID:       p.str("testID"),
	}
}

func shaderEffectFace(p *props) core.Node {
	prog := p.r.program(p.str("source"))
	node, number := p.progress()
	return core.ShaderEffect{
		Program:         prog,
		UniformsFunc:    func() core.Uniforms { return p.r.uniforms(prog, p.get("uniforms")) },
		ProgressNode:    node,
		ProgressFunc:    number,
		MaxSampleOffset: p.float("maxSampleOffset"),
		PausedFunc:      func() bool { return p.bool("paused") },
		Class:           p.class(),
		StyleFunc:       p.styleFunc(),
		Children:        p.children(),
		OnLayout:        p.onLayout(),
		Ref:             p.n.ref,
		TestID:          p.str("testID"),
	}
}

func shaderTransitionFace(p *props) core.Node {
	prog := p.r.program(p.str("source"))
	return core.ShaderTransition{
		Program:         prog,
		NoFallback:      p.str("fallback") == "none",
		UniformsFunc:    func() core.Uniforms { return p.r.uniforms(prog, p.get("uniforms")) },
		Value:           p.transitionValue(),
		Duration:        p.duration("duration"),
		MaxSampleOffset: p.float("maxSampleOffset"),
		PausedFunc:      func() bool { return p.bool("paused") },
		Class:           p.class(),
		StyleFunc:       p.styleFunc(),
		Children:        p.children(),
		OnLayout:        p.onLayout(),
		Ref:             p.n.ref,
		TestID:          p.str("testID"),
	}
}

func screenEffectFace(p *props) core.Node {
	prog := p.r.program(p.str("source"))
	node, number := p.progress()
	return core.ScreenEffect{
		Program:         prog,
		UniformsFunc:    func() core.Uniforms { return p.r.uniforms(prog, p.get("uniforms")) },
		ProgressNode:    node,
		ProgressFunc:    number,
		MaxSampleOffset: p.float("maxSampleOffset"),
		PausedFunc:      func() bool { return p.bool("paused") },
	}
}

func screenTransitionFace(p *props) core.Node {
	prog := p.r.program(p.str("source"))
	return core.ScreenTransition{
		Program:         prog,
		NoFallback:      p.str("fallback") == "none",
		UniformsFunc:    func() core.Uniforms { return p.r.uniforms(prog, p.get("uniforms")) },
		Value:           p.transitionValue(),
		Duration:        p.duration("duration"),
		MaxSampleOffset: p.float("maxSampleOffset"),
		PausedFunc:      func() bool { return p.bool("paused") },
	}
}

// transitionShader decodes a page's or a dialog's transitionShader:
// { source, uniforms, duration }.
func (p *props) transitionShader() *core.TransitionShader {
	o, ok := p.get("transitionShader").(*js.Object)
	if !ok {
		return nil
	}
	prog := p.r.program(stringOf(o.Get("source")))
	if prog == nil {
		return nil
	}
	ts := &core.TransitionShader{Program: prog, Uniforms: p.r.uniforms(prog, o.Get("uniforms"))}
	if d := o.Get("duration"); defined(d) {
		ts.Duration = time.Duration(d.ToFloat() * float64(time.Millisecond))
	}
	return ts
}
