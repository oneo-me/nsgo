//go:build !nsgo_noshader

package core

import (
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/shader"
	"github.com/oneo-me/nsgo/internal/shader/wgsl"
	"github.com/oneo-me/nsgo/internal/shaderir"
)

// builtinLanes are the lanes a WGSL program's built-ins take first.
const builtinLanes = wgsl.BuiltinLanes

// crossfadeSource is the transitions' default program.
const crossfadeSource = wgsl.Crossfade

func compileFragment(src string) (*ShaderProgram, error) {
	p, err := shader.CompileFragment([]byte(src))
	if err != nil {
		return nil, err
	}
	sp := &ShaderProgram{p: &driver.ShaderProgram{IR: p}, uniforms: map[string]uniformSlot{}}
	lane := 0
	for i, name := range p.UniformNames {
		if name == shader.RectSizeUniform || name == shader.ScaleUniform || name == shader.ImageSizesUniform {
			continue
		}
		t := p.Uniforms[i]
		n := t.DwordCount()
		if t.Main == shaderir.Array {
			t = t.Sub[0]
		}
		sp.uniforms[name] = uniformSlot{lane: lane, n: n, vec3: t.Main == shaderir.Vec3}
		lane += n
	}
	sp.lanes = lane
	return sp, nil
}

func compileWGSL(src string) (*ShaderProgram, error) {
	c, err := wgsl.Compile(src)
	if err != nil {
		return nil, err
	}
	sp := &ShaderProgram{p: &driver.ShaderProgram{IR: c.IR, WGSL: true}, uniforms: map[string]uniformSlot{},
		animated: c.Animated, pictures: driver.Pictures(c.Inputs)}
	lane := wgsl.BuiltinLanes
	for _, u := range c.Uniforms {
		sp.uniforms[u.Name] = uniformSlot{lane: lane, n: u.Lanes, vec3: u.Vec3}
		lane += u.Lanes
	}
	sp.lanes = lane
	return sp, nil
}
