// Package updates finds a newer version of the app for the backends whose
// platforms have no updater of their own (updater.Check): in the app's
// appcast, the RSS feed in Sparkle's format that nsgo build writes, on
// Linux and Android, and in the App Store on iOS. macOS and Windows hand
// the same appcast to Sparkle and WinSparkle instead.
package updates

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

// Config is how the app updates, as the nsgo tool sets it from
// nsgo.yaml's updates; nsgo.Run gives it to the backend.
type Config struct {
	// App and ID are the app's name and identifier.
	App, ID string
	// Version is the running version as users see it, and Build the one
	// an appcast item's sparkle:version is compared with: the build
	// number, else Version.
	Version, Build string
	// Feed is this target's appcast, and Key the public EdDSA key its
	// downloads are signed with, in base64.
	Feed, Key string
	// Automatic has the app check on its own: WinSparkle on its schedule,
	// and nsgo.Run once a day as the app starts elsewhere.
	Automatic bool
	// Store is what installed the app and updates it in nsgo.yaml's
	// place, as the backend finds as the app starts: Google Play or
	// another store on Android, Flatpak or Snap on Linux, TestFlight on
	// iOS. The app then checks nothing.
	Store string
}

// Checks reports whether the app has anything to check, appStore for iOS,
// which asks the App Store, and elsewhere its appcast.
func (c Config) Checks(appStore bool) bool {
	return c.Store == "" && (appStore || c.Feed != "")
}

// AndroidStore names the store that installed an Android app, which
// updates it, from installer, the installing package: com.android.vending
// for Google Play. It is "" for an APK the user installed through the
// system's package installer, or adb did, and for none known. An
// installer it does not know is a store, so an app never sends a store's
// users to its own downloads, which stores forbid.
func AndroidStore(installer string) string {
	if installer == "" || installer == "com.android.shell" || strings.HasSuffix(installer, ".packageinstaller") {
		return ""
	}
	return installer
}

// managed is the answer to a check of an app that Store updates.
func (c Config) managed() driver.Update {
	return driver.Update{App: c.App, Current: c.Version, Err: "updates: " + c.Store + " installed the app and updates it"}
}

// build is the version an appcast's items are compared with.
func (c Config) build() string {
	if c.Build != "" {
		return c.Build
	}
	return c.Version
}

// timeout bounds one check.
const timeout = 30 * time.Second

// Answer runs find off the main thread and dispatches what it found as
// the EventUpdate answering request id.
func Answer(id uint32, find func(context.Context) driver.Update, dispatch func(driver.Event)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		u := find(ctx)
		dispatch(driver.Event{Kind: driver.EventUpdate, Count: id, Data: &u})
	}()
}

// Find reads c's appcast and answers with its newest item when that is
// newer than the running build.
func Find(ctx context.Context, c Config) driver.Update {
	if c.Store != "" {
		return c.managed()
	}
	u := driver.Update{App: c.App, Current: c.Version}
	if c.Feed == "" {
		u.Err = "updates: nsgo.yaml configures no updates for this build"
		return u
	}
	body, err := get(ctx, c.Feed)
	if err == nil {
		var item Item
		if item, err = ParseAppcast(body); err == nil && Newer(item.Build, c.build()) {
			u.Version, u.URL = item.Version, item.Link
			u.Download, u.Signature, u.SHA256, u.Key, u.Size = item.Download, item.Signature, item.SHA256, c.Key, item.Size
		}
	}
	if err != nil {
		u.Err = err.Error()
	}
	return u
}

// FindInAppStore asks the App Store of country, an ISO 3166 code or ""
// for the United States', for the app by its bundle ID, and answers with
// its version there when that is newer than the running one. An app the
// store does not list there has none newer.
func FindInAppStore(ctx context.Context, c Config, country string) driver.Update {
	if c.Store != "" {
		return c.managed()
	}
	u := driver.Update{App: c.App, Current: c.Version}
	if c.ID == "" {
		u.Err = "updates: the app has no bundle ID"
		return u
	}
	q := url.Values{"bundleId": {c.ID}}
	if country != "" {
		q.Set("country", strings.ToLower(country))
	}
	body, err := get(ctx, "https://itunes.apple.com/lookup?"+q.Encode())
	if err != nil {
		u.Err = err.Error()
		return u
	}
	version, link, err := parseLookup(body)
	if err != nil {
		u.Err = err.Error()
	} else if version != "" && Newer(version, c.Version) {
		u.Version, u.URL = version, link
	}
	return u
}

// parseLookup reads the App Store's lookup answer: the first result's
// version and page, none when it lists no app.
func parseLookup(body []byte) (version, link string, err error) {
	var r struct {
		Results []struct {
			Version string `json:"version"`
			URL     string `json:"trackViewUrl"`
			ID      int64  `json:"trackId"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", "", fmt.Errorf("updates: the App Store's answer: %w", err)
	}
	if len(r.Results) == 0 {
		return "", "", nil
	}
	app := r.Results[0]
	// The page the answer names, else the app's by its ID in the store.
	link = webURL(app.URL)
	if link == "" && app.ID > 0 {
		link = fmt.Sprintf("https://apps.apple.com/app/id%d", app.ID)
	}
	return strings.TrimSpace(app.Version), link, nil
}

// get downloads u with the backend's HTTP client.
func get(ctx context.Context, u string) ([]byte, error) {
	if imageload.HTTP == nil {
		return nil, errors.New("updates: no HTTP client")
	}
	resp, err := imageload.HTTP(ctx, u, imageload.Header{"Cache-Control": {"no-cache"}}, func(int64, int64) {})
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("updates: %s answered %d", u, resp.Status)
	}
	return resp.Body, nil
}

// Item is a release an appcast lists.
type Item struct {
	// Version is what users see (sparkle:shortVersionString, else
	// sparkle:version) and Build what is compared (sparkle:version).
	Version, Build string
	// Link is the release's page, else its download.
	Link string
	// Download and its integrity metadata name the package itself.
	Download, Signature, SHA256 string
	Size                        int64
}

// appcast is the part of Sparkle's feed the backends read. An item's
// versions are elements of their own, or the enclosure's attributes in
// Sparkle's older form.
type appcast struct {
	Items []struct {
		Link         string `xml:"link"`
		Version      string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle version"`
		ShortVersion string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle shortVersionString"`
		Enclosure    struct {
			URL          string `xml:"url,attr"`
			Length       int64  `xml:"length,attr"`
			Signature    string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle edSignature,attr"`
			SHA256       string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle sha256,attr"`
			Version      string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle version,attr"`
			ShortVersion string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle shortVersionString,attr"`
		} `xml:"enclosure"`
	} `xml:"channel>item"`
}

// ParseAppcast reads an appcast's newest release: its item of the
// highest build that names one and a page or download.
func ParseAppcast(data []byte) (Item, error) {
	var feed appcast
	if err := xml.Unmarshal(data, &feed); err != nil {
		return Item{}, fmt.Errorf("updates: the appcast: %w", err)
	}
	var newest Item
	for _, it := range feed.Items {
		item := Item{
			Build:    first(it.Version, it.Enclosure.Version),
			Download: webURL(it.Enclosure.URL), Signature: strings.TrimSpace(it.Enclosure.Signature),
			SHA256: strings.TrimSpace(it.Enclosure.SHA256), Size: it.Enclosure.Length,
		}
		item.Version = first(it.ShortVersion, it.Enclosure.ShortVersion, item.Build)
		item.Link = first(webURL(it.Link), webURL(it.Enclosure.URL))
		if item.Build != "" && item.Link != "" && (newest.Build == "" || Newer(item.Build, newest.Build)) {
			newest = item
		}
	}
	if newest.Build == "" {
		return Item{}, errors.New("updates: the appcast lists no release")
	}
	return newest, nil
}

// webURL returns link as a browser or a store opens it when it is an http
// or https URL with a host, else "".
func webURL(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.String()
}

// first returns the first of values that is not blank, trimmed.
func first(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// Newer reports whether version a is newer than b, as Sparkle compares
// them: run by run, of digits, compared as numbers, of periods and of
// anything else, compared as text. A number is newer than text and a
// period. Where one version has more runs, the next one makes it older
// when it is text (1.0b1 before 1.0) and newer otherwise (1.0.1 after
// 1.0).
func Newer(a, b string) bool { return compare(a, b) > 0 }

// compare orders versions a and b: -1, 0 or 1.
func compare(a, b string) int {
	ra, rb := runs(a), runs(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		x, y := ra[i], rb[i]
		if x.kind == y.kind {
			c := strings.Compare(x.text, y.text)
			if x.kind == runNumber {
				c = compareNumbers(x.text, y.text)
			}
			if c != 0 {
				return c
			}
			continue
		}
		switch {
		case y.kind == runText:
			return 1
		case x.kind == runText:
			return -1
		case x.kind == runNumber:
			return 1
		default:
			return -1
		}
	}
	switch {
	case len(ra) > len(rb):
		if ra[len(rb)].kind == runText {
			return -1
		}
		return 1
	case len(rb) > len(ra):
		if rb[len(ra)].kind == runText {
			return 1
		}
		return -1
	}
	return 0
}

// compareNumbers orders runs of digits by their values, however long.
func compareNumbers(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		if len(a) > len(b) {
			return 1
		}
		return -1
	}
	return strings.Compare(a, b)
}

type runKind uint8

const (
	runNumber runKind = iota
	runPeriod
	runText
)

type run struct {
	kind runKind
	text string
}

// runs splits a version into its runs; spaces separate them and are
// dropped.
func runs(v string) []run {
	var out []run
	kindOf := func(r rune) (runKind, bool) {
		switch {
		case r >= '0' && r <= '9':
			return runNumber, true
		case r == '.':
			return runPeriod, true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			return 0, false
		}
		return runText, true
	}
	start, last, open := 0, runKind(0), false
	for i, r := range v {
		k, ok := kindOf(r)
		// Each period is a run of its own, as Sparkle's.
		if open && (!ok || k != last || k == runPeriod) {
			out = append(out, run{last, v[start:i]})
			open = false
		}
		if ok && !open {
			start, last, open = i, k, true
		}
	}
	if open {
		out = append(out, run{last, v[start:]})
	}
	return out
}
