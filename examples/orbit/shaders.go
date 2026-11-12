package main

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/core"
)

var (
	dialProgram      = core.MustCompileWGSL(dialSource)
	resonanceProgram = core.MustCompileWGSL(resonanceSource)
	loupeProgram     = core.MustCompileWGSL(loupeSource)
)

// dial fills style with paper and, on the floor the cards stand on, a
// calibrated scale that turns with the ring under a fixed signal mark.
// The scale's angle is the ring's, so it follows a drag and a spring on
// the platform's frame clock. Behind the ring a halftone wall drifts and
// slides with the ring, slower, for depth.
func dial(cx *core.Scope, r *ring, style core.Style) core.Node {
	return core.Shader{Program: dialProgram, Style: style,
		Uniforms: core.Uniforms{"turn": r.angle, "paper": paper, "ink": ink, "signal": signal}}
}

// dialSource projects each pixel back onto the floor plane under the
// stage, the plane through the cards' lower edges, as the stage's
// perspective sees it, and draws the scale in the ring's own polar
// coordinates there: a hairline circle under the cards, minor ticks every
// 3 degrees and major ones every 30, fading into the distance. Lines are
// measured in screen points through fwidth, so they stay a hairline at
// any depth.
var dialSource = fmt.Sprintf(`struct Uniforms { turn: f32, paper: vec3f, ink: vec3f, signal: vec3f }

const P = %[1]d.0;
const R = %[2]d.0;
const B = %[3]d.0;
const LIFT = %[4]d.0;
// CELL is the print screen's pitch in points, and SLIDE how far the wall
// slides per degree the ring turns: a fifth of the ring's own pace at its
// front, so it reads as far behind.
const CELL = 7.0;
const SLIDE = 1.15;

fn hash(p: vec2f) -> f32 {
	return fract(sin(dot(p, vec2f(127.1, 311.7))) * 43758.5453);
}

fn noise(p: vec2f) -> f32 {
	let i = floor(p);
	let f = fract(p);
	let u = f * f * (3.0 - 2.0 * f);
	return mix(mix(hash(i), hash(i + vec2f(1.0, 0.0)), u.x), mix(hash(i + vec2f(0.0, 1.0)), hash(i + vec2f(1.0, 1.0)), u.x), u.y);
}

// halftone is how much of the pixel at p a 45-degree print screen of
// CELL-point cells covers, its dots sized for a tone of v in 0 to 1.
fn halftone(p: vec2f, v: f32) -> f32 {
	let g = vec2f(p.x + p.y, p.y - p.x) * 0.70710678 / CELL;
	let d = length(fract(g) - 0.5) * CELL;
	let r = sqrt(clamp(v, 0.0, 1.0)) * CELL * 0.62;
	let aa = max(fwidth(d), 1e-4);
	return (1.0 - smoothstep(r - aa, r + aa, d)) * step(0.02, v);
}

// line is how much of a line of half-width w, d points from its middle,
// covers the pixel.
fn line(d: f32, w: f32) -> f32 {
	let aa = max(fwidth(d), 1e-4);
	return 1.0 - smoothstep(w - aa, w + aa, d);
}

fn shade(uv: vec2f) -> vec4f {
	let pt = uv * nsh.size;
	let s = pt - (nsh.size * 0.5 - vec2f(0.0, LIFT));
	let below = max(s.y, 4.0);

	// The floor point under the pixel, from the viewer at P: its depth
	// toward the viewer and its sideways offset.
	let z = P - B * P / below;
	let q = vec2f(s.x * B / below, z + R);
	let r = length(q);
	let raw = atan2(q.x, q.y) * 57.29578;
	let a = raw - uniforms.turn;

	// How far a degree runs along the circle, in floor points.
	let arc = r * 0.0174533;
	let minor = abs(fract(a / 3.0 + 0.5) - 0.5) * 3.0 * arc;
	let major = abs(fract(a / 30.0 + 0.5) - 0.5) * 30.0 * arc;

	var ink = 0.0;
	ink = max(ink, line(abs(r - R), 0.5) * 0.35);
	ink = max(ink, line(abs(r - R - 70.0), 0.5) * 0.2);
	let band = step(R + 28.0, r) * step(r, R + 46.0);
	let longBand = step(R + 20.0, r) * step(r, R + 58.0);
	ink = max(ink, line(minor, 0.45) * band * 0.4);
	ink = max(ink, line(major, 0.9) * longBand * 0.75);

	// The signal mark sits at the front, still, while the scale turns.
	let mark = line(abs(raw) * arc, 1.0) * step(R + 62.0, r) * step(r, R + 80.0);

	// The floor fades with distance and toward the horizon.
	let fog = exp(min(z, 0.0) / 900.0) * smoothstep(6.0, 70.0, s.y);
	var col = uniforms.paper;
	col = mix(col, uniforms.ink, ink * fog);
	col = mix(col, uniforms.signal, mark * smoothstep(6.0, 70.0, s.y));

	// The wall behind the ring is printed in halftone: a tone that drifts
	// slowly as weather would, deepening toward the window's edges and
	// leaving a clearing behind the ring. It slides with the ring, slower,
	// and gives way to the floor at the horizon.
	let wp = pt + vec2f(uniforms.turn * SLIDE, 0.0);
	let drift = vec2f(nsh.time * 2.2, nsh.time * -1.1);
	let weather = noise((wp + drift) / 260.0) * 0.65 + noise((wp - drift * 1.7) / 110.0) * 0.35;
	let away = (uv - vec2f(0.5, 0.42)) * vec2f(1.15, 1.5);
	let tone = (smoothstep(0.18, 0.95, length(away)) * 0.75 + 0.15) * smoothstep(0.35, 0.75, weather) * 1.6;
	let wall = 1.0 - smoothstep(-90.0, 30.0, s.y);
	col = mix(col, uniforms.ink, halftone(wp, tone * wall) * 0.13);

	// A light from above: the paper darkens a little toward its edges.
	let e = (uv - vec2f(0.5, 0.4)) * vec2f(1.0, 1.3);
	col = col * (1.0 - 0.07 * dot(e, e));
	// Paper grain, a pixel at a time.
	col = col + (hash(floor(pt * nsh.scale)) - 0.5) * 0.014;
	return vec4f(col, 1.0);
}
`, perspective, radius, cardHeight/2, stageLift)

// resonanceSource draws two sets of ring lines, wavelength points apart,
// running outward from two sources; where they cross, moiré shows their
// interference. The sources are marked in the signal color.
const resonanceSource = `struct Uniforms { wavelength: f32, ink: vec3f, paper: vec3f, signal: vec3f }

fn rings(d: f32, t: f32) -> f32 {
	let w = uniforms.wavelength;
	let x = abs(fract(d / w - t) - 0.5) * w;
	let aa = max(fwidth(d), 1e-4);
	return 1.0 - smoothstep(0.45 - aa, 0.45 + aa, x);
}

fn shade(uv: vec2f) -> vec4f {
	let p = (uv - 0.5) * nsh.size;
	let off = vec2f(nsh.size.x * 0.17, 0.0);
	let a = length(p + off);
	let b = length(p - off);
	let t = nsh.time * 0.35;
	let lines = max(rings(a, t), rings(b, t));
	var col = mix(uniforms.paper, uniforms.ink, lines * 0.62);
	let dot = max(1.0 - smoothstep(2.5, 3.5, a), 1.0 - smoothstep(2.5, 3.5, b));
	col = mix(col, uniforms.signal, dot);
	return vec4f(col, 1.0);
}
`

// loupeSource passes a round glass slowly over the content, magnifying
// what is under it with a little barrel at its edge, and draws its rim.
const loupeSource = `struct Uniforms { ink: vec3f }

fn shade(uv: vec2f) -> vec4f {
	let p = uv * nsh.size;
	let t = nsh.time * 0.45;
	let c = nsh.size * 0.5 + vec2f(cos(t), sin(t * 1.3)) * nsh.size * vec2f(0.3, 0.22);
	let d = p - c;
	let r = length(d);
	let lens = 42.0;
	let k = r / lens;
	let inside = 1.0 - smoothstep(lens - 0.75, lens + 0.75, r);
	let scale = mix(1.0, 0.62 + 0.3 * k * k, inside);
	var col = sample_content((c + d * scale) / nsh.size);
	let rim = 1.0 - smoothstep(0.0, 1.0, abs(r - lens));
	col = mix(col, vec4f(uniforms.ink, 1.0), rim * 0.85);
	let shade = (1.0 - smoothstep(lens, lens + 10.0, r)) * (1.0 - inside) * 0.12;
	col = vec4f(col.rgb * (1.0 - shade), col.a);
	return col;
}
`
