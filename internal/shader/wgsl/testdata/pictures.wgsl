fn shade(uv: vec2f) -> vec4f {
	let wobble = vec2f(sin(uv.y * 20.0 + nsh.time) * 0.01, 0.0);
	let c = sample_content(uv + wobble);
	let b = sample_backdrop(uv);
	return mix(b, c, nsh.progress);
}
