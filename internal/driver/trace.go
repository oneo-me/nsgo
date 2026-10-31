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
	for i := range b.Items {
		fmt.Fprintf(&s, "   %s\n", b.Items[i].TraceLine())
	}
	io.WriteString(w, s.String())
}

var itemNames = map[ItemKind]string{
	ItemCreate: "create", ItemInsert: "insert", ItemRemove: "remove", ItemDelete: "delete",
	ItemFrame: "frame", ItemViewStyle: "style", ItemText: "text", ItemInput: "input",
	ItemInputValue: "value", ItemEvents: "events", ItemViewProps: "props",
	ItemScrollConfig: "scroll-config", ItemScrollContent: "scroll-content", ItemFocus: "focus",
	ItemBlur: "blur", ItemImage: "image", ItemModal: "modal", ItemAlert: "alert",
	ItemDismissKeyboard: "dismiss-keyboard", ItemScrollTo: "scroll-to", ItemCanvas: "canvas",
	ItemSetClipboard: "set-clipboard", ItemReadClipboard: "read-clipboard", ItemAnnounce: "announce",
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
		ItemSetClipboard, ItemOpenURL:
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
