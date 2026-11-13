(function (...sources) {
	const out = {};
	for (const s of sources) {
		if (s != null) Object.defineProperties(out, Object.getOwnPropertyDescriptors(s));
	}
	return out;
})
