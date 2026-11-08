const STEPS: i32 = 4;
const PALETTE = array<vec3f, 3>(vec3f(1.0, 0.0, 0.0), vec3f(0.0, 1.0, 0.0), vec3f(0.0, 0.0, 1.0));

fn pick(k: i32) -> vec3f {
	switch k {
		case 0: { return vec3f(0.2); }
		case 1, 2: { return vec3f(0.5); }
		default: { return PALETTE[k % 3]; }
	}
}

fn shade(uv: vec2f) -> vec4f {
	var n = 0;
	var x = uv.x;
	while (x > 0.1) {
		x = x * 0.5;
		n++;
		if (n > 10) { break; }
	}
	var total = 0u;
	loop {
		total += 1u;
		if (total % 2u == 0u) { continue; }
		continuing {
			break if total >= 7u;
		}
	}
	var k = 10;
	for (; k > 0; k -= 3) {
		if (k == 4) { continue; }
	}
	var acc = vec3f();
	for (var j = 0u; j < 3u; j += 1u) {
		switch j {
			case 1u: { acc += pick(i32(j)); break; }
			default: {}
		}
	}
	let m = mat2x2f(cos(uv.x), sin(uv.x), -sin(uv.x), cos(uv.x));
	let r = m * uv;
	let sel = select(vec3f(0.0), acc, vec3f(uv, 0.2) > vec2f(0.5).xxy);
	let h = pick(n) + f32(total) * 0.01 + f32(k) * 0.001 + sel + vec3f(r, 0.0);
	return vec4f(saturate(h), select(0.5, 1.0, all(uv < vec2f(1.0)) && any(uv > vec2f(0.0))));
}
