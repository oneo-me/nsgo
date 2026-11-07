package base

import "github.com/oneo-me/nsgo/core"

// controlKey is a key a focused control binds: whether it repeats, and
// what it does.
type controlKey struct {
	key    string
	repeat bool
	run    func()
}

// bindWhile binds keys while active holds, a control's focus, so they
// never take a key from the rest of the app; with directed Left and Right
// trade places in a right-to-left layout, as WAI-ARIA's patterns say.
func bindWhile(active func() bool, when core.KeyWhen, directed bool, keys ...controlKey) core.Node {
	return core.Show{When: active, Then: func(cx *core.Scope) core.Node {
		env := cx.Env()
		for _, k := range keys {
			key, swapped := k.key, k.key
			switch k.key {
			case "left":
				swapped = "right"
			case "right":
				swapped = "left"
			}
			run := k.run
			if directed && key != swapped {
				other := findKey(keys, swapped)
				if other != nil {
					mine := run
					run = func() {
						if core.Untracked(cx, env.RTL) {
							other()
						} else {
							mine()
						}
					}
				}
			}
			cx.OnKeyWith(key, core.KeyOptions{When: when, Repeat: k.repeat}, run)
		}
		return nil
	}}
}

func findKey(keys []controlKey, key string) func() {
	for _, k := range keys {
		if k.key == key {
			return k.run
		}
	}
	return nil
}
