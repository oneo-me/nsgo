alias Color = vec4<f32>;
const PI = 3.14159265;

fn ring(d: f32, r: f32) -> f32 { return smoothstep(r - 0.01, r, d) - smoothstep(r, r + 0.01, d); }

fn shade(uv: vec2f) -> vec4f {
	let p = uv * 2.0 - 1.0;
	let d = length(p);
	let a = atan2(p.y, p.x) / (2.0 * PI);
	var c: Color = Color(0.0);
	c.r = ring(d, 0.5) + fract(a);
	c[1] = abs(p.x) % 0.25;
	let i = vec4i(1, -2, 3, -4);
	let u = vec2u(7u, 9u) % 4u;
	c.b = f32(dot(vec2f(abs(i.xy)), vec2f(u))) * 0.01 + round(p.x) * trunc(p.y) + tanh(p.x) + asinh(p.y);
	c.a = pow(clamp(1.0 - d, 0.0, 1.0), 2.0) + degrees(radians(1.0)) * 0.0 + ldexp(1.0, -3);
	let flags = vec3<bool>(true, d > 0.5, false);
	if (flags.y && !flags.z) { c = vec4f(c.rgb * 0.5, c.a); }
	_ = nsh.scale;
	return c;
}
