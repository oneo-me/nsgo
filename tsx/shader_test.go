package tsx_test

import (
	"testing"

	"github.com/oneo-me/nsgo/nsgotest"
)

// The shader elements compile their WGSL and send it with their uniforms:
// a fill reads a color and an animated value, an effect its progress,
// and a transition runs as its value changes.
func TestShaderElements(t *testing.T) {
	app := run(t, `import { mount, signal, createAnimatedValue, Shader, ShaderEffect, ShaderTransition, ScreenEffect, Text, View } from "nsgo";
const FILL = `+"`"+`struct Uniforms { tint: vec4f, glow: f32 }
fn shade(uv: vec2f) -> vec4f { return uniforms.tint * (uniforms.glow + 0.5 * sin(nsh.time)); }`+"`"+`;
const BLUR = `+"`"+`fn shade(uv: vec2f) -> vec4f { return sample_content(uv) * nsh.progress; }`+"`"+`;
function App() {
	const glow = createAnimatedValue(0.25);
	const page = signal(0);
	return <View>
		<Shader testID="fill" source={FILL} uniforms={{ tint: "#ff0000", glow, unknown: 1 }} style={{ height: 40 }}>
			<Text>over</Text>
		</Shader>
		<ShaderEffect testID="fx" source={BLUR} progress={0.5} maxSampleOffset={3}><Text>blurred</Text></ShaderEffect>
		<ShaderTransition testID="tr" value={page()} duration={200}><Text>{"page " + page()}</Text></ShaderTransition>
		<ScreenEffect source={BLUR} />
		<Text testID="next" onPress={() => page.set(page() + 1)}>next</Text>
	</View>;
}
mount(App);`)
	fill := app.Find("id:fill")
	sh := fill.Shader
	if sh == nil || !sh.WGSL || sh.Role != nsgotest.ShaderFill {
		t.Fatalf("fill shader %+v", sh)
	}
	// Time, progress, overflow, then tint and glow.
	if len(sh.Lanes) != 8 || sh.Lanes[3] != 1 || sh.Lanes[6] != 1 {
		t.Errorf("lanes %v", sh.Lanes)
	}
	if len(sh.Nodes) != 2 || sh.Nodes[0].Lane != 0 || sh.Nodes[1].Lane != 7 {
		t.Errorf("nodes %v, want the clock and glow", sh.Nodes)
	}
	app.Find("text:over")
	if fx := app.Find("id:fx").Shader; fx == nil || fx.Role != nsgotest.ShaderEffect || fx.Lanes[1] != 0.5 || fx.Overflow != 3 {
		t.Errorf("effect %+v", fx)
	}
	tr := app.Find("id:tr")
	if tr.Shader == nil || tr.Shader.Role != nsgotest.ShaderTransition || tr.Shader.Running {
		t.Fatalf("transition %+v", tr.Shader)
	}
	app.Press("id:next")
	app.Settle()
	tr = app.Find("id:tr")
	if tr.ShaderStarts != 1 {
		t.Errorf("the transition started %d times", tr.ShaderStarts)
	}
	app.Find("text:page 1")
}

// A page's transitionShader runs over its stack as it is pushed.
func TestNavPageTransitionShader(t *testing.T) {
	app := run(t, `import { mount, Text } from "nsgo";
import { createNavStack, NavStack, NavPage } from "nsgo/base";
const SWIPE = `+"`"+`fn shade(uv: vec2f) -> vec4f { return select(sample_from(uv), sample_to(uv), uv.x < nsh.progress); }`+"`"+`;
const nav = createNavStack(["home"]);
mount(() => <NavStack state={nav}>{(route) =>
	<NavPage title={route} transitionShader={{ source: SWIPE, duration: 400 }}>
		<Text testID={"go-" + route} onPress={() => nav.push("detail")}>{route}</Text>
	</NavPage>}</NavStack>);`)
	app.Press("id:go-home")
	app.Settle()
	var stack *nsgotest.View
	for v := app.Find("text:detail"); v != nil; v = v.Parent {
		if v.Shader != nil && v.Shader.Role == nsgotest.ShaderNav {
			stack = v
		}
	}
	// Settling runs the transition to its end.
	if stack == nil || stack.ShaderStarts != 1 || stack.Shader.Running {
		t.Fatalf("no page shader ran: %+v", stack)
	}
}
