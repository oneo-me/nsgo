(function (props, keys) {
	const picked = {}, rest = {};
	const all = Object.getOwnPropertyDescriptors(props);
	for (const k of Object.keys(all)) Object.defineProperty(keys.includes(k) ? picked : rest, k, all[k]);
	return [picked, rest];
})
