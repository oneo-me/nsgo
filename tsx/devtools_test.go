package tsx_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/devtools"
)

// DevTools name a TSX app's state and computations by their TSX lines, through the bundle's source map, not the Go that runs them.
func TestDevToolsNameTSXLines(t *testing.T) {
	devtools.Enabled = true
	defer func() { devtools.Enabled = false }()
	app := run(t, `import { mount, signal, computed, Text } from "nsgo";
function App() {
	const count = signal(41);
	const next = computed(() => count() + 1);
	return <Text testID="t">{next()}</Text>;
}
mount(App);`)
	surfaces := devtools.Surfaces()
	s := surfaces[len(surfaces)-1]
	defer devtools.Unregister(s)
	var owners devtools.Owner
	app.Post(func() { owners = s.Owners() })
	app.Settle()
	sites := map[string]string{}
	var walk func(o devtools.Owner)
	walk = func(o devtools.Owner) {
		for _, v := range o.Values {
			sites[v.Text] = v.Site
		}
		for _, c := range o.Computations {
			if c.Kind == "memo" {
				sites["memo "+c.Value] = c.Site
			}
		}
		for _, c := range o.Children {
			walk(c)
		}
	}
	walk(owners)
	if sites["41"] != "App.tsx:3" || sites["memo 42"] != "App.tsx:4" {
		t.Fatalf("sites = %v, want the signal at App.tsx:3 and the memo at App.tsx:4", sites)
	}
}
