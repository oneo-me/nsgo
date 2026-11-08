fn hash(p: vec2f) -> f32 {
	var q = bitcast<vec2u>(vec2i(floor(p)));
	q = q * vec2u(1597334673u, 3812015801u);
	let n = (q.x ^ q.y) * 1597334673u;
	return f32(n >> 8u) / 16777216.0;
}

fn noise(p: vec2f) -> f32 {
	let i = floor(p);
	let f = fract(p);
	let u = f * f * (3.0 - 2.0 * f);
	return mix(mix(hash(i), hash(i + vec2f(1.0, 0.0)), u.x), mix(hash(i + vec2f(0.0, 1.0)), hash(i + 1.0), u.x), u.y);
}

fn shade(uv: vec2f) -> vec4f {
	var sum = 0.0;
	var amp = 0.5;
	var p = uv * nsh.size / 40.0;
	for (var i = 0; i < 5; i++) {
		sum += amp * noise(p);
		p *= 2.0;
		amp *= 0.5;
	}
	return vec4f(vec3f(sum), 1.0);
}
