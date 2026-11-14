//go:build js && wasm

package widget

import (
	"encoding/json"
	"strings"
	"syscall/js"
	"time"
)

// publish hands the timeline to the HarmonyOS ArkTS host. A home-screen
// card runs in its own process with no Web view, so the widget package
// reaches the host's bridge directly, as it reaches WidgetKit on Apple
// platforms: the host writes the data for the card's provider and asks
// the system to refresh its forms. The card's page shows what the app
// wrote last, until it writes again.
func publish(kind string, entries []Entry, now time.Time) error {
	bridge := js.Global().Get("nsgoHarmony")
	if !bridge.Truthy() {
		return ErrUnsupported
	}
	text := ""
	for _, e := range entries {
		if e.View == nil {
			continue
		}
		if t := textOf(e.View.node()); t != "" {
			text = t
			break
		}
	}
	data, err := json.Marshal(map[string]string{"kind": kind, "title": text})
	if err != nil {
		return err
	}
	bridge.Call("updatewidget", string(data))
	return nil
}

func available() bool {
	return js.Global().Get("nsgoHarmony").Truthy()
}

// textOf collects the text a widget's view shows, which the 2x2 card
// draws as its title.
func textOf(n *node) string {
	if n == nil {
		return ""
	}
	var parts []string
	if n.Text != "" {
		parts = append(parts, n.Text)
	}
	for _, c := range n.Children {
		parts = append(parts, textOf(c))
	}
	return strings.Join(parts, " ")
}
