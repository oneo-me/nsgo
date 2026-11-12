//go:build nsgo_noshader

package core

import "github.com/nsgo-dev/nsgo/internal/driver"

// Builds tagged nsgo_noshader leave the shader compilers out, for targets
// that draw no shader and count their size, as WeChat mini programs do:
// every source compiles to a program the platform leaves empty, which
// takes any uniform.

// builtinLanes is wgsl.BuiltinLanes, which shader_lanes_test.go checks.
const builtinLanes = 3

const crossfadeSource = ""

func compileFragment(string) (*ShaderProgram, error) { return uncompiled(false), nil }

func compileWGSL(string) (*ShaderProgram, error) { return uncompiled(true), nil }

func uncompiled(wgsl bool) *ShaderProgram {
	return &ShaderProgram{p: &driver.ShaderProgram{WGSL: wgsl}, uniforms: map[string]uniformSlot{}, lanes: builtinLanes,
		uncompiled: true}
}
