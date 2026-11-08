struct Uniforms { tint: vec4f, glow: vec3f, speed: f32, steps: i32, seed: u32, stops: array<vec4f, 3>, at: vec2f }

fn shade(uv: vec2f) -> vec4f {
	let wave = 0.5 + 0.5 * sin(uv.x * 12.0 + nsh.time * uniforms.speed);
	var c = uniforms.tint.rgb * wave + uniforms.glow * f32(uniforms.steps) * 0.01;
	c += uniforms.stops[1].rgb * f32(uniforms.seed & 1u);
	return vec4f(c, distance(uv, uniforms.at));
}
