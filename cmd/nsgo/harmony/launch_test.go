package harmony_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLaunchFlagsReachThePage checks the chain a Want's nsgo- parameters
// take to go.argv: the ability reads them, the page's proxy answers launch(),
// boot.html puts them in argv, and a launch without them still has the
// build's perf.json to fall back on. A link in that chain that drifts loses
// the flags without any error on the device.
func TestLaunchFlagsReachThePage(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	ability := read(t, filepath.Join(root, "cmd", "nsgo", "harmony", "templates", "entry", "src", "main", "ets", "entryability", "EntryAbility.ets"))
	page := read(t, filepath.Join(root, "cmd", "nsgo", "harmony", "templates", "entry", "src", "main", "ets", "pages", "Index.ets"))
	boot := read(t, filepath.Join(root, "cmd", "nsgo", "harmony", "boot.html"))
	module := read(t, filepath.Join(root, "cmd", "nsgo", "harmony", "templates", "entry", "src", "main", "module.json5"))

	for _, want := range []string{"want.parameters", "nsgo-", "onNewWant", "AppStorage.setOrCreate('nsgoLaunch'"} {
		if !strings.Contains(ability, want) {
			t.Errorf("EntryAbility.ets no longer %q: a Want's flags never reach the page", want)
		}
	}
	if !strings.Contains(page, "launch(): string") || !strings.Contains(page, "'launch'") {
		t.Error("Index.ets must answer launch() and list it in javaScriptProxy's methodList")
	}
	for _, want := range []string{"nsgoHarmony.launch()", "launch.args", "go.argv", "perf.json"} {
		if !strings.Contains(boot, want) {
			t.Errorf("boot.html no longer %q: the flags or their fallback are gone", want)
		}
	}
	// The environment is the system's, not a hard-coded guess.
	for _, want := range []string{"getSystemLocale", "deviceInfo.majorVersion", "fontSizeScale", "isRTL", "isScreenReaderOpenSync", "OpenedURL"} {
		if !strings.Contains(page, want) {
			t.Errorf("Index.ets no longer reads %q from the system", want)
		}
	}
	if !strings.Contains(module, "ohos.want.action.viewData") || !strings.Contains(module, `"scheme": "nsgo"`) {
		t.Error("module.json5 no longer routes nsgo: links to the ability")
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
