package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// The gallery counts its use in the NSGo site's Umami, as pages of the
// site under /gallery/: a pageview for each demo shown, tagged
// "gallery-app" on the native targets and "gallery-web" in the browser, as
// the site's own pages are "website". The browser's loader counts the
// arrival as the event load-gallery before the app starts. A native
// install carries a random ID kept in the storage, so that one install is
// one visitor; the web is counted as the site's pages are, and only on the
// site's hosts (statsPage). Builds of nsgo dev (statsBuild) and test
// binaries report nothing, and the settings turn it off.
const (
	statsURL     = "https://umami.oneo.me/api/send"
	statsWebsite = "e8174058-eb31-40f8-bf9c-4ddc9ca5f4ac"
	statsPath    = "/gallery/"
)

// Storage keys of the statistics: the install's ID, and statsKey "off"
// when the settings turned them off.
const (
	installKey = "installID"
	statsKey   = "stats"
)

// Umami measures a visit from its first event to its last, so while a demo
// is read the gallery sends a heartbeat each statsBeat, for statsIdle after
// the last demo shown: a window left open does not count as use.
const (
	statsBeat = time.Minute
	statsIdle = 5 * time.Minute
)

// statsOn reports whether the settings leave the statistics on.
func statsOn(cx *core.Scope) bool {
	v, _ := cx.Storage().GetItem(statsKey)
	return v != "off"
}

// setStatsOn turns the statistics on or off.
func setStatsOn(cx *core.Scope, on bool) {
	if on {
		cx.Storage().RemoveItem(statsKey)
	} else {
		cx.Storage().SetItem(statsKey, "off")
	}
}

// statsPayload is an event of Umami's /api/send: a pageview without a
// name.
type statsPayload struct {
	Website  string `json:"website"`
	Hostname string `json:"hostname"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Language string `json:"language,omitempty"`
	Screen   string `json:"screen,omitempty"`
	Referrer string `json:"referrer,omitempty"`
	Tag      string `json:"tag"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
}

// reportUsage sends a pageview whenever the demo shown changes, and the
// heartbeats while it is read. Events are made on the graph's goroutine
// and sent from another, oldest first; one that fails is dropped.
func reportUsage(cx *core.Scope, g *gallery) {
	if !statsBuild || testing.Testing() {
		return
	}
	host, referrer, ok := statsPage()
	if !ok {
		return
	}
	env := cx.Env()
	tag, id := "gallery-web", ""
	if env.OS() != core.OSWeb {
		tag, id = "gallery-app", installID(cx)
	}
	agent := statsAgent(env.OS())
	queue := make(chan statsPayload, 32)
	go sendStats(queue, agent)

	var shown string
	var since time.Time
	send := func(name string) {
		if !statsOn(cx) {
			return
		}
		url, title := statsPath, "Gallery"
		if shown != "" {
			url, title = statsPath+neturl.PathEscape(shown), shown
		}
		w := env.Window()
		p := statsPayload{Website: statsWebsite, Hostname: host, URL: url, Title: title,
			Language: env.Locale(), Screen: fmt.Sprintf("%dx%d", int(w.Width), int(w.Height)),
			Referrer: referrer, Tag: tag, ID: id, Name: name}
		// The visit came from the referrer once, with its first page.
		referrer = ""
		select {
		case queue <- p:
		default:
		}
	}
	cx.Effect(func() {
		open := g.shownDemo()
		core.Untracked(cx, func() bool {
			if open != shown || since.IsZero() {
				shown, since = open, time.Now()
				send("")
			}
			return true
		})
	})
	var beat func()
	beat = func() {
		if time.Since(since) < statsIdle {
			send("heartbeat")
		}
		cx.AfterFunc(statsBeat, beat)
	}
	cx.AfterFunc(statsBeat, beat)
}

// installID returns the install's random ID, made on first use.
func installID(cx *core.Scope) string {
	store := cx.Storage()
	id, ok := store.GetItem(installKey)
	if !ok {
		b := make([]byte, 16)
		rand.Read(b)
		id = hex.EncodeToString(b)
		store.SetItem(installKey, id)
	}
	return id
}

// sendStats posts the queue's events to Umami until it closes.
func sendStats(queue <-chan statsPayload, agent string) {
	client := &http.Client{Timeout: 10 * time.Second}
	for p := range queue {
		body, _ := json.Marshal(map[string]any{"type": "event", "payload": p})
		req, err := http.NewRequest(http.MethodPost, statsURL, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if agent != "" {
			req.Header.Set("User-Agent", agent)
		}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
		}
	}
}

// statsAgent is the User-Agent a native gallery sends, as a browser on
// the platform would say it, for Umami to tell the system and not take it
// for a bot as it does Go's own. The browser sends its own.
func statsAgent(os core.OS) string {
	platforms := map[core.OS]string{
		core.OSMacOS:   "Macintosh; Intel Mac OS X 10_15_7",
		core.OSWindows: "Windows NT 10.0; Win64; x64",
		core.OSLinux:   "X11; Linux x86_64",
		core.OSIOS:     "iPhone; CPU iPhone OS 18_0 like Mac OS X",
		core.OSAndroid: "Linux; Android 15",
	}
	p, ok := platforms[os]
	if !ok {
		return ""
	}
	return "Mozilla/5.0 (" + p + ") NSGoGallery/1.0"
}
