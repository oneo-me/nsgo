package tsx_test

import (
	"testing"
	"testing/fstest"

	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/tsx"
	"github.com/oneo-me/nsgo/tsx/bundler"
)

// TestHotUpdatesKeepState edits a component: it renders again with the
// new code, while a counter in another module keeps its count and the
// app is not mounted again.
func TestHotUpdatesKeepState(t *testing.T) {
	files := fstest.MapFS{
		"App.tsx": {Data: []byte(`import { mount, View } from "nsgo";
import { Counter } from "./counter";
import { Greeting } from "./greeting";
mount(() => <View><Counter /><Greeting /></View>);`)},
		"counter.tsx": {Data: []byte(`import { signal, Text } from "nsgo";
const count = signal(0);
export function Counter() { return <Text testID="count" onPress={() => count.set(count() + 1)}>{() => "count " + count()}</Text>; }`)},
		"greeting.tsx": {Data: []byte(`import { Text } from "nsgo";
export function Greeting() { return <Text>Hello</Text>; }`)},
	}
	first, err := bundler.CompileHot(files, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	app, update := tsx.Hot("App.tsx", first)
	a := nsgotest.Start(t, app)
	a.Press("id:count")
	a.Press("id:count")
	if !a.Exists("text:count 2") || !a.Exists("text:Hello") {
		t.Fatalf("texts %q", a.Texts())
	}
	files["greeting.tsx"] = &fstest.MapFile{Data: []byte(`import { Text } from "nsgo";
export function Greeting() { return <Text>Hello again</Text>; }`)}
	next, err := bundler.CompileHot(files, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	update(next)
	a.Settle()
	if !a.Exists("text:Hello again") || !a.Exists("text:count 2") || len(a.Query("text:count 2")) != 1 {
		t.Errorf("after the edit %q", a.Texts())
	}
	// An edited entry mounts the app again, once; the counter's module,
	// unchanged, keeps its count.
	files["App.tsx"] = &fstest.MapFile{Data: []byte(`import { mount, Text, View } from "nsgo";
import { Counter } from "./counter";
import { Greeting } from "./greeting";
mount(() => <View><Text>Title</Text><Counter /><Greeting /></View>);`)}
	next, err = bundler.CompileHot(files, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	update(next)
	a.Settle()
	if got := a.Texts(); len(got) != 3 || got[0] != "Title" || got[1] != "count 2" {
		t.Errorf("after editing the entry %q", got)
	}
}
