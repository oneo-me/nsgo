(function (g) {
	"use strict";
	const refuse = function () { throw new EvalError("nsgo: code is not made from strings here"); };
	for (const make of [function () {}, async function () {}, function* () {}]) {
		const F = make.constructor;
		Object.defineProperty(F.prototype, "constructor", { value: refuse });
	}
	delete g.eval;
	g.Function = refuse;
	for (const C of [Object, Array, Function, String, Number, Boolean, Symbol, Promise, Map, Set, WeakMap,
		WeakSet, Date, RegExp, Error, TypeError, RangeError, SyntaxError, ArrayBuffer, Uint8Array, JSON, Math, Reflect]) {
		Object.freeze(C);
		if (C.prototype) Object.freeze(C.prototype);
	}
})
