package harmony_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// This file is the HarmonyOS preview's coverage table: what the driver can
// send and what the renderer answers. A kind with no case in mount.go or no
// KIND_ constant in Index.ets used to disappear with no error anywhere; it
// must appear in one of the groups below instead. Adding a case without
// removing its group, or adding a driver constant without a group, fails the
// suite, so the table only ever shrinks as the target grows.
var (
	itemCase = regexp.MustCompile(`(?m)^\s*case\s+((?:driver\.Item\w+)(?:\s*,\s*driver\.Item\w+)*)`)
	usedEv   = regexp.MustCompile(`driver\.(Event[A-Z]\w*)`)
)

// unsupportedItems are the mount items the HarmonyOS mounter drops, grouped
// by the work that would answer them. "platform limits" holds what no public
// SDK interface here answers; each has its reason in harmonyos.md's
// constraints. Android's runtime answers 8 of these, the core already
// mirrors right-to-left frames, Direction and Overflow need no answer here,
// and the rest are desktop-only or unimplemented on every mobile backend.
var unsupportedItems = map[string]string{
	"overlays and navigation": `ItemBackPreview ItemHistory`,
	"platform limits": `ItemPickScreenColor
		ItemSpeech ItemCanInstallPackages ItemInstallPackage ItemInstallSettings
		ItemInputSource ItemNative
		ItemNativeCommand ItemBlockNative ItemShader ItemShaderInput
		ItemShaderTransition`,
	"accessibility": `ItemAccessibilityFocus`,
	"notifications and badges": `ItemBadge ItemCloseNotification ItemHaptic`,
	"desktop only": `ItemDock ItemMenu ItemPopupMenu ItemLaunchAtLogin
		ItemSetLaunchAtLogin ItemOpenSurface ItemQuit ItemTray ItemTrayMenu
		ItemWindow ItemHotKeys`,
}

// unsupportedEvents are the events no HarmonyOS code reads or dispatches.
var unsupportedEvents = map[string]string{
	"pointers, gestures and drops": `EventMouseEnter EventMouseLeave EventMouseMove
		EventPan EventPinch EventRotation EventWheel EventPressOutside
		EventContextMenu EventFileDragEnter EventFileDragLeave EventFileDrop`,
	"keyboard, text and selection": `EventKeyPress EventCompositionChange
		EventCaretMove EventPaste EventAccessibilityAction`,
	"loading, media and animation": `EventLoadEnd EventDownload EventNative
		EventNativeSize EventAnimation EventBuffer`,
	"overlays, navigation and windows": `EventRequestClose EventModalSize
		EventModalShown EventModalDismissed EventScreenHeader EventScreenInsets
		EventNavigationStateChange EventMenu EventMenuClosed EventMessage
		EventBackStarted EventBackCancelled EventWindowClose
		EventWindowHidden EventReopen EventOpenFiles`,
	"notifications and input modality": `EventNotificationPress EventInputModality`,
	"system and desktop": `EventTray EventHotKey EventLaunchAtLogin EventSpeech`,
}

// unsupportedKinds are the view kinds Index.ets has no constant for.
// KindView is the default branch of NodeView, so it needs no constant of
// its own but still has no dedicated renderer.
var unsupportedKinds = map[string]string{
	"no view drawn yet": `KindNative KindShader`,
	"default branch":    `KindView`,
}

func TestItemCoverage(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	all := driverNames(t, filepath.Join(root, "internal", "driver", "mount.go"), "ItemCreate", itemNameOf("Item"))
	handled := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(root, "internal", "platform", "harmonyos", "mount.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range itemCase.FindAllStringSubmatch(string(data), -1) {
		for _, n := range regexp.MustCompile(`Item\w+`).FindAllString(c[1], -1) {
			handled[n] = true
		}
	}
	checkCoverage(t, "item", all, handled, unsupportedItems)
}

func TestEventCoverage(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	all := driverNames(t, filepath.Join(root, "internal", "driver", "events.go"), "EventPress", itemNameOf("Event"))
	used := map[string]bool{}
	dir := filepath.Join(root, "internal", "platform", "harmonyos")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range usedEv.FindAllStringSubmatch(string(data), -1) {
			used[m[1]] = true
		}
	}
	checkCoverage(t, "event", all, used, unsupportedEvents)
}

func TestViewKindCoverage(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	all := driverNames(t, filepath.Join(root, "internal", "driver", "mount.go"), "KindView", kindName)
	data, err := os.ReadFile(filepath.Join(root, "cmd", "nsgo", "harmony", "templates", "entry", "src", "main", "ets", "pages", "Index.ets"))
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, m := range hostKind.FindAllStringSubmatch(string(data), -1) {
		declared[driverKindName(m[1])] = true
	}
	checkCoverage(t, "view kind", all, declared, unsupportedKinds)
}

// checkCoverage fails when a driver name has no renderer and no group, or a
// group still names something the renderer now handles or the driver lost.
func checkCoverage(t *testing.T, label string, all, handled map[string]bool, groups map[string]string) {
	t.Helper()
	if len(all) == 0 || len(handled) == 0 {
		t.Fatalf("%s: parsed %d driver names and %d handled ones: the patterns no longer match", label, len(all), len(handled))
	}
	registered := map[string]string{}
	for group, list := range groups {
		for _, n := range strings.Fields(list) {
			if !all[n] {
				t.Errorf("%s: %s in group %q is not a driver name", label, n, group)
			}
			if handled[n] {
				t.Errorf("%s: %s has a renderer now: remove it from group %q", label, n, group)
			}
			if prior, dup := registered[n]; dup {
				t.Errorf("%s: %s is in groups %q and %q", label, n, prior, group)
			}
			registered[n] = group
		}
	}
	var unregistered []string
	for n := range all {
		if !handled[n] && registered[n] == "" {
			unregistered = append(unregistered, n)
		}
	}
	sort.Strings(unregistered)
	if len(unregistered) > 0 {
		t.Errorf("%s: %d with no renderer and no group: %s", label, len(unregistered), strings.Join(unregistered, ", "))
	}
}

// itemNameOf matches the constants of one const block in a driver file.
func itemNameOf(prefix string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*(` + prefix + `[A-Z]\w*)\b`)
}

// driverNames returns the constants of the block that starts with first.
func driverNames(t *testing.T, path, first string, re *regexp.Regexp) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, b := range constBlock.FindAllStringSubmatch(string(data), -1) {
		found := re.FindAllStringSubmatch(b[1], -1)
		if len(found) == 0 || found[0][1] != first {
			continue
		}
		for _, n := range found {
			out[n[1]] = true
		}
	}
	if len(out) == 0 {
		t.Fatalf("no %s constants in %s: the pattern no longer matches", first, path)
	}
	return out
}
