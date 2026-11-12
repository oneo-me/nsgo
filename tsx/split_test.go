package tsx_test

import (
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/tsx"
	"github.com/nsgo-dev/nsgo/tsx/bundler"
)

// TestSplitBundlesLoadChunksOnDemand splits an app: the screen an
// import() names loads only when the import runs, as one chunk with its
// own module, and shares the entry's modules, their state included.
func TestSplitBundlesLoadChunksOnDemand(t *testing.T) {
	files := fstest.MapFS{
		"App.tsx": {Data: []byte(`import { mount, signal, Text, View } from "nsgo";
import { taps } from "./state";
const Screen = signal<null | (() => any)>(null);
function open() { import("./screen").then((m) => Screen.set(() => m.Screen)); }
mount(() => <View>
  <Text testID="open" onPress={() => { taps.set(taps() + 1); open(); }}>open</Text>
  {() => { const S = Screen(); return S ? <S /> : null; }}
</View>);`)},
		"state.ts": {Data: []byte(`import { signal } from "nsgo";
export const taps = signal(0);`)},
		"screen.tsx": {Data: []byte(`import { Text } from "nsgo";
import { taps } from "./state";
import { label } from "./label";
export function Screen() { return <Text testID="screen">{label} {taps()}</Text>; }`)},
		"label.ts": {Data: []byte(`export const label = "screen after";`)},
	}
	main, chunks, err := bundler.CompileSplit(files, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks["app:screen.tsx"] == "" {
		t.Fatalf("chunks %v", keys(chunks))
	}
	if strings.Contains(main, "screen after") || !strings.Contains(chunks["app:screen.tsx"], "screen after") {
		t.Error("the screen's own module is not in its chunk alone")
	}
	if strings.Contains(chunks["app:screen.tsx"], "signal(0)") {
		t.Error("the chunk holds the entry's state module again")
	}
	var mu sync.Mutex
	var loaded []string
	app := tsx.SplitApp("App.tsx", main, func(key string) (string, error) {
		mu.Lock()
		loaded = append(loaded, key)
		mu.Unlock()
		return chunks[key], nil
	})
	a := nsgotest.Start(t, app)
	if len(loaded) != 0 || a.Exists("id:screen") {
		t.Fatalf("loaded %v before the import", loaded)
	}
	a.Press("id:open")
	a.WaitFor("id:screen", 5*time.Second)
	if got := a.Find("id:screen").Text; got != "screen after 1" {
		t.Errorf("the screen shows %q", got)
	}
	a.Press("id:open")
	a.WaitFor("text:screen after 2", 5*time.Second)
	if len(loaded) != 1 {
		t.Errorf("loaded %v: each chunk once", loaded)
	}
	// A package that is not split holds the screen and imports it inline.
	whole, err := bundler.Compile(files, "App.tsx")
	if err != nil || !strings.Contains(whole, "screen after") {
		t.Errorf("the whole bundle: %v", err)
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
