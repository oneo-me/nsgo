package widget

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// read returns the timeline written in dir, and the names of the images
// beside it.
func read(t *testing.T, dir string) (timeline, []string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, timelineFile))
	if err != nil {
		t.Fatal(err)
	}
	var tl timeline
	if err := json.Unmarshal(data, &tl); err != nil {
		t.Fatal(err)
	}
	var images []string
	files, _ := os.ReadDir(dir)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), imagePrefix) {
			images = append(images, f.Name())
		} else if f.Name() != timelineFile {
			t.Errorf("%s left in the kind's directory", f.Name())
		}
	}
	return tl, images
}

func TestTimeline(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "counter")
	now := time.Unix(1_800_000_000, 500_000_000)
	later := now.Add(time.Hour)
	err := writeTimeline(dir, []Entry{
		{Date: later, View: Text{Content: "later"}, URL: "https://example.com/later"},
		{
			View: Column{
				Style: Style{FontSize: 20, FontDesign: Rounded, Gap: 2, AlignItems: AlignStart, Padding: core.All(8), FillWidth: true, FillHeight: true},
				Children: []View{
					Text{Content: "Count", Style: Style{TextStyle: Caption}},
					Text{Content: "42", Style: Style{FontWeight: core.FontWeightBold, FontVariant: core.FontTabularNums}},
					Row{Style: Style{Gap: 4}, Children: []View{Symbol{Name: "star.fill"}, Text{Content: "x", Style: Style{FontSize: 12}}}},
					nil,
				},
			},
			Families:   map[Family]View{Small: Text{Content: "small"}, Large: nil},
			Background: Rectangle{Fill: RGB(0x336699).Dark(RGB(0x000000))},
		},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	tl, images := read(t, dir)
	if tl.V != timelineVersion || len(tl.Entries) != 2 || len(images) != 0 {
		t.Fatalf("timeline = %+v, images %v", tl, images)
	}
	first, second := tl.Entries[0], tl.Entries[1]
	if first.Date != 1_800_000_000.5 || second.Date != 1_800_003_600.5 || second.URL != "https://example.com/later" {
		t.Errorf("entries out of order, or zero date not now: %+v", tl.Entries)
	}
	if len(first.Families) != 1 || first.Families[Small].Text != "small" {
		t.Errorf("families = %+v, want small alone", first.Families)
	}
	col := first.View
	if col.T != "column" || col.Font != nil || *col.Gap != 2 || col.Items != "start" || !col.FillW || !col.FillH || *col.Padding != [4]float64{8, 8, 8, 8} {
		t.Errorf("column = %+v", col)
	}
	if len(col.Children) != 3 {
		t.Fatalf("children = %d, want the nil view left out", len(col.Children))
	}
	label, count, row := col.Children[0], col.Children[1], col.Children[2]
	// Texts take the font of the views holding them.
	if f := label.Font; f.Size != nil || f.Style != "caption" || f.Design != "rounded" {
		t.Errorf("label font = %+v, want caption, rounded", f)
	}
	if f := count.Font; *f.Size != 20 || f.Weight != 700 || !f.Digits || f.Design != "rounded" {
		t.Errorf("count font = %+v, want 20 bold rounded with tabular digits", f)
	}
	if f := row.Children[0].Font; *f.Size != 20 || f.Weight != 0 {
		t.Errorf("symbol font = %+v, want the column's", f)
	}
	if f := row.Children[1].Font; *f.Size != 12 {
		t.Errorf("small text font = %+v", f)
	}
	if bg := first.BG; bg.T != "rectangle" || bg.Color == nil || bg.Color.Dark == nil {
		t.Errorf("background = %+v", bg)
	}
}

func TestImages(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "photo")
	png := []byte("\x89PNG one")
	file := filepath.Join(t.TempDir(), "two.jpg")
	if err := os.WriteFile(file, []byte("\xff\xd8 two"), 0o644); err != nil {
		t.Fatal(err)
	}
	pic := Image{Data: png, Cover: true}
	if err := writeTimeline(dir, []Entry{{View: Stack{Children: []View{pic, Image{File: file}, pic}}}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	tl, images := read(t, dir)
	kids := tl.Entries[0].View.Children
	if len(images) != 2 || kids[0].Image != kids[2].Image || kids[0].Image == kids[1].Image || !slices.Contains(images, kids[1].Image) {
		t.Fatalf("images %v for %+v, want one file per picture", images, kids)
	}
	if !kids[0].Cover || kids[1].Cover {
		t.Errorf("covers = %v, %v", kids[0].Cover, kids[1].Cover)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, kids[1].Image)); string(data) != "\xff\xd8 two" {
		t.Errorf("image file holds %q", data)
	}

	// A new timeline takes away the images it no longer shows.
	if err := writeTimeline(dir, []Entry{{View: pic}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, images := read(t, dir); !slices.Equal(images, []string{kids[0].Image}) {
		t.Errorf("images after the update = %v, want %s alone", images, kids[0].Image)
	}
	if err := writeTimeline(dir, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if tl, images := read(t, dir); len(tl.Entries) != 0 || len(images) != 0 {
		t.Errorf("an empty timeline left %+v and %v", tl, images)
	}

	if err := writeTimeline(dir, []Entry{{View: Image{File: filepath.Join(dir, "missing")}}}, time.Now()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an image that cannot be read = %v", err)
	}
}

func TestValues(t *testing.T) {
	e := &encoder{images: map[string][]byte{}}
	for _, c := range []struct {
		v    View
		want float64
	}{{Progress{Value: 2}, 1}, {Gauge{Value: -1}, 0}, {Progress{Value: 0.25}, 0.25}} {
		n, _ := e.node(c.v, nil)
		if *n.Value != c.want {
			t.Errorf("%+v = %v, want %v", c.v, *n.Value, c.want)
		}
	}
	n, _ := e.node(Date{Time: time.Unix(10, 0), Format: Timer}, nil)
	if *n.Date != 10 || n.DateStyle != "timer" {
		t.Errorf("date = %+v", n)
	}
	n, _ = e.node(Text{Content: "a", Style: Style{TextAlign: core.TextAlignCenter, MaxLines: 2, MinScale: 0.5, Link: "https://example.com", Accented: true}}, nil)
	if n.TextAlign != "center" || *n.MaxLines != 2 || *n.MinScale != 0.5 || n.Link != "https://example.com" || !n.Accent {
		t.Errorf("text = %+v", n)
	}
}

func TestColors(t *testing.T) {
	for _, c := range []struct {
		c    Color
		want string
	}{
		{Color{}, `null`},
		{RGB(0xffffff), `{"r":1,"g":1,"b":1,"a":1}`},
		{RGB(0x336699), `{"r":0.2,"g":0.4,"b":0.6,"a":1}`},
		{RGBA(0x000000, 0.5), `{"r":0,"g":0,"b":0,"a":0.502}`},
		{RGBA(0xff0000, 0.5).Dark(RGB(0x00ff00)), `{"r":1,"g":0,"b":0,"a":0.502,"dark":{"r":0,"g":1,"b":0,"a":1}}`},
		{Secondary, `{"r":0,"g":0,"b":0,"a":1,"name":"secondary"}`},
		{Primary.Dark(RGB(0xffffff)), `{"r":0,"g":0,"b":0,"a":1,"name":"primary","dark":{"r":1,"g":1,"b":1,"a":1}}`},
	} {
		got, err := json.Marshal(encodeColor(c.c))
		if err != nil || string(got) != c.want {
			t.Errorf("%+v = %s, %v, want %s", c.c, got, err, c.want)
		}
	}
}

func TestUpdate(t *testing.T) {
	if err := Update("no/slash"); err == nil {
		t.Error("Update took a kind with a slash")
	}
	err := Update("counter", Entry{View: Text{Content: "1"}})
	switch runtime.GOOS {
	case "darwin", "ios":
		// A test is no app built with its widgets.
		if !errors.Is(err, ErrNoWidgets) || Available() {
			t.Errorf("Update outside an app = %v, want ErrNoWidgets", err)
		}
	default:
		if !errors.Is(err, errors.ErrUnsupported) || Available() {
			t.Errorf("Update on %s = %v, want ErrUnsupported", runtime.GOOS, err)
		}
	}
}
