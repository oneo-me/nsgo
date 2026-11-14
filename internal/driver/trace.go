package driver

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// WriteTrace prints batch b, which a commit that took took made, as
// traces show it: a line with label, the items and the commit's time,
// then a line per item. nsgo test -trace labels batches by their step,
// nsgo dev -trace by their commit.
func WriteTrace(w io.Writer, label string, b *MountBatch, took time.Duration) {
	var s strings.Builder
	fmt.Fprintf(&s, "── %s: %d items, commit %.2f ms\n", label, len(b.Items), float64(took.Microseconds())/1000)
	if c := b.Causes; c != nil {
		c.write(&s)
	}
	for i := range b.Items {
		fmt.Fprintf(&s, "   %s\n", b.Items[i].TraceLine())
	}
	io.WriteString(w, s.String())
}

// TraceCauses has commits record what they followed from in MountBatch.Causes: nsgo dev -trace and nsgo test -trace set it. While it is unset, a signal's write and a computation's run cost one check of it.
var TraceCauses bool

// Causes are what a commit followed from: the signal writes since the graph's last commit, grouped by the line that made them, and the computations that ran.
type Causes struct {
	Writes                  []Write
	Memos, Renders, Effects int
	// App is how long the app's code ran before the commit, its handlers, timers and effects, and Layout how long the commit laid the tree out; a trace's took is the commit's whole.
	App, Layout time.Duration
	// Ran are the computations that ran, by kind and the line that made them, the longest first; they are named only while DevTools record sites (devtools.Enabled).
	Ran []Ran
}

// Ran is the runs of the computations of one kind made at one line, and their time.
type Ran struct {
	Kind, Site string
	Runs       int
	Took       time.Duration
}

// traceRan is how many of a commit's computations a trace names.
const traceRan = 5

// Write is the signal writes one line made: the app's line, as dir/file.go:line, or for writes NSGo made of its own, as an animation's, NSGo's function that made them.
type Write struct {
	Site  string
	Count int
}

// traceSites is how many of a commit's write sites a trace names, the most writing first.
const traceSites = 8

func (c *Causes) write(s *strings.Builder) {
	if len(c.Writes) > 0 {
		s.WriteString("   after")
		for i, w := range c.Writes[:min(len(c.Writes), traceSites)] {
			if i > 0 {
				s.WriteString(",")
			}
			fmt.Fprintf(s, " %s", w.Site)
			if w.Count > 1 {
				fmt.Fprintf(s, " ×%d", w.Count)
			}
		}
		if more := len(c.Writes) - traceSites; more > 0 {
			fmt.Fprintf(s, " and %d more", more)
		}
		s.WriteString("\n")
	}
	fmt.Fprintf(s, "   ran %d bindings, %d effects, %d memos\n", c.Renders, c.Effects, c.Memos)
	if c.App > 0 || c.Layout > 0 {
		fmt.Fprintf(s, "   app %.2f ms before, layout %.2f ms\n", ms(c.App), ms(c.Layout))
	}
	if len(c.Ran) > 0 {
		s.WriteString("   longest")
		for i, r := range c.Ran[:min(len(c.Ran), traceRan)] {
			if i > 0 {
				s.WriteString(",")
			}
			fmt.Fprintf(s, " %s %s ×%d %.2f ms", r.Kind, r.Site, r.Runs, ms(r.Took))
		}
		s.WriteString("\n")
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

var itemNames = map[ItemKind]string{
	ItemCreate: "create", ItemInsert: "insert", ItemRemove: "remove", ItemDelete: "delete",
	ItemFrame: "frame", ItemViewStyle: "style", ItemText: "text", ItemInput: "input",
	ItemInputValue: "value", ItemEvents: "events", ItemViewProps: "props",
	ItemScrollConfig: "scroll-config", ItemScrollContent: "scroll-content", ItemFocus: "focus",
	ItemBlur: "blur", ItemImage: "image", ItemModal: "modal", ItemAlert: "alert",
	ItemDismissKeyboard: "dismiss-keyboard", ItemScrollTo: "scroll-to", ItemCanvas: "canvas",
	ItemSetClipboard: "set-clipboard", ItemReadClipboard: "read-clipboard", ItemAnnounce: "announce", ItemInputSource: "input-source",
	ItemKeymap: "keymap", ItemStatusBar: "status-bar", ItemTitle: "title",
	ItemBackUnhandled: "back-unhandled", ItemWindow: "window", ItemMenu: "menu", ItemWeb: "web",
	ItemWebCommand: "web-command", ItemVideo: "video", ItemVideoSeek: "video-seek",
	ItemNative: "native", ItemNativeCommand: "native-command", ItemAnimation: "animation",
	ItemZoomTo: "zoom-to", ItemImageInfo: "image-info", ItemSelection: "selection",
	ItemScrollBy: "scroll-by", ItemStickyCells: "sticky-cells", ItemAccessibilityLabel: "label",
	ItemAccessibilityFocus: "accessibility-focus", ItemScreen: "screen", ItemOverflow: "overflow",
	ItemBlockNative: "block-native", ItemShader: "shader", ItemSetClipboardImage: "set-clipboard-image",
	ItemOpenURL: "open-url", ItemPickImages: "pick-images",
	ItemAppearance: "appearance", ItemPopupMenu: "popup-menu", ItemQuit: "quit", ItemBackPreview: "back-preview", ItemHistory: "history",
	ItemPickFiles: "pick-files", ItemSaveFile: "save-file", ItemDirection: "direction", ItemPickDirectories: "pick-directories", ItemPickScreenColor: "pick-screen-color",
	ItemOpenFile: "open-file", ItemShare: "share", ItemInstallPackage: "install-package",
	ItemCanInstallPackages: "can-install-packages", ItemInstallSettings: "install-settings",
	ItemCanDrawOverlays: "can-draw-overlays", ItemOverlaySettings: "overlay-settings",
	ItemOpenSurface: "open-surface", ItemTray: "tray", ItemTrayMenu: "tray-menu", ItemHotKeys: "hot-keys",
	ItemDock: "dock", ItemSetLaunchAtLogin: "set-launch-at-login", ItemLaunchAtLogin: "launch-at-login",
	ItemShaderTransition: "shader-transition", ItemShaderInput: "shader-input", ItemSpeech: "speech",
	ItemTakePhoto: "take-photo", ItemMatch: "match", ItemCheckForUpdates: "check-for-updates",
	ItemBiometricAvailability: "biometric-availability", ItemBiometricAuthenticate: "biometric-authenticate",
}

// TraceLine is the item as traces print it: its kind, its view and what
// it carries that a reader looks for.
func (it *MountItem) TraceLine() string {
	name := itemNames[it.Kind]
	if name == "" {
		name = fmt.Sprintf("item-%d", it.Kind)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-14s view:%d", name, it.ID)
	switch it.Kind {
	case ItemCreate:
		fmt.Fprintf(&b, " %s", it.View)
	case ItemInsert:
		fmt.Fprintf(&b, " into view:%d", it.Parent)
		if it.Before != 0 {
			fmt.Fprintf(&b, " before view:%d", it.Before)
		}
	case ItemFrame:
		f := it.Frame
		fmt.Fprintf(&b, " %g,%g %gx%g", f.Origin.X, f.Origin.Y, f.Size.Width, f.Size.Height)
	case ItemText:
		fmt.Fprintf(&b, " %q", it.Text.Text)
	case ItemInputValue, ItemTitle, ItemAnnounce, ItemAccessibilityLabel,
		ItemSetClipboard, ItemOpenURL, ItemInputSource:
		fmt.Fprintf(&b, " %q", it.Value)
	case ItemViewProps:
		if p := it.ViewProps; p.TestID != "" || p.Label != "" {
			fmt.Fprintf(&b, " id=%q label=%q", p.TestID, p.Label)
		}
	case ItemScrollTo, ItemScrollBy:
		fmt.Fprintf(&b, " %g,%g", it.Point.X, it.Point.Y)
	case ItemScrollContent:
		fmt.Fprintf(&b, " %gx%g", it.Size.Width, it.Size.Height)
	case ItemAnimation:
		fmt.Fprintf(&b, " %d ops", len(it.Animation))
	}
	return b.String()
}

// String names the kind as traces print it.
func (k ViewKind) String() string {
	switch k {
	case KindView:
		return "view"
	case KindText:
		return "text"
	case KindInput:
		return "input"
	case KindScroll:
		return "scroll"
	case KindImage:
		return "image"
	case KindModal:
		return "modal"
	case KindCanvas:
		return "canvas"
	case KindWeb:
		return "web"
	case KindVideo:
		return "video"
	case KindShader:
		return "shader"
	}
	return "?"
}

// Applied, which nsgo dev -devtools sets, hears how long a platform took to apply each batch on its main thread; nil measures nothing.
var Applied func(b *MountBatch, took time.Duration)

// Apply applies b with apply, timing it for Applied while one is set: what each backend calls where it applies a batch.
func Apply(b *MountBatch, apply func(*MountBatch)) {
	if Applied == nil {
		apply(b)
		return
	}
	began := time.Now()
	apply(b)
	Applied(b, time.Since(began))
}
