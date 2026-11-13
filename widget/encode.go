package widget

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// The widget extension (cmd/nsgo/widgets/Widgets.swift) reads a kind's
// timeline from <dir>/timeline.json and its images from the files beside
// it, named by their contents.
const (
	timelineFile = "timeline.json"
	imagePrefix  = "img-"
	// timelineVersion changes with what the extension cannot read of
	// earlier timelines.
	timelineVersion = 1
)

type timeline struct {
	V       int     `json:"v"`
	Entries []entry `json:"entries"`
}

type entry struct {
	Date     float64          `json:"date"`
	View     *node            `json:"view,omitempty"`
	Families map[Family]*node `json:"families,omitempty"`
	BG       *node            `json:"bg,omitempty"`
	URL      string           `json:"url,omitempty"`
}

// node is a view as the extension reads it. Fonts are resolved here, each
// text taking its containers', so that the extension sets whole fonts.
type node struct {
	T         string      `json:"t"`
	Text      string      `json:"text,omitempty"`
	Symbol    string      `json:"symbol,omitempty"`
	Image     string      `json:"image,omitempty"`
	Cover     bool        `json:"cover,omitempty"`
	Date      *float64    `json:"date,omitempty"`
	DateStyle string      `json:"dateStyle,omitempty"`
	Value     *float64    `json:"value,omitempty"`
	Children  []*node     `json:"children,omitempty"`
	Gap       *float64    `json:"gap,omitempty"`
	Items     string      `json:"items,omitempty"`
	Justify   string      `json:"justify,omitempty"`
	Font      *font       `json:"font,omitempty"`
	Color     *color      `json:"color,omitempty"`
	BG        *color      `json:"bg,omitempty"`
	Padding   *[4]float64 `json:"padding,omitempty"` // top, end, bottom, start
	Width     *float64    `json:"width,omitempty"`
	Height    *float64    `json:"height,omitempty"`
	FillW     bool        `json:"fillW,omitempty"`
	FillH     bool        `json:"fillH,omitempty"`
	Radius    *float64    `json:"radius,omitempty"`
	Link      string      `json:"link,omitempty"`
	MaxLines  *int        `json:"maxLines,omitempty"`
	MinScale  *float64    `json:"minScale,omitempty"`
	TextAlign string      `json:"textAlign,omitempty"`
	Accent    bool        `json:"accent,omitempty"`
	Action    string      `json:"action,omitempty"`
}

// font is a text's font: a size or a text style, with a weight, a design
// and variants.
type font struct {
	Size      *float64 `json:"size,omitempty"`
	Style     string   `json:"style,omitempty"`
	Weight    int      `json:"weight,omitempty"`
	Design    string   `json:"design,omitempty"`
	Italic    bool     `json:"italic,omitempty"`
	Digits    bool     `json:"digits,omitempty"`
	SmallCaps bool     `json:"smallCaps,omitempty"`
}

// color is a Color as the extension reads it: sRGB components from 0 to
// 1, or a system color's name, with another for dark mode.
type color struct {
	R    float64 `json:"r"`
	G    float64 `json:"g"`
	B    float64 `json:"b"`
	A    float64 `json:"a"`
	Name string  `json:"name,omitempty"`
	Dark *color  `json:"dark,omitempty"`
}

func encodeColor(c Color) *color {
	if c.light == 0 && c.system == "" {
		return nil
	}
	rgba := func(v core.Color) *color {
		r, g, b, a := v.Components()
		unit := func(f float64) float64 { return math.Round(f*1e4) / 1e4 }
		return &color{R: unit(r), G: unit(g), B: unit(b), A: unit(a)}
	}
	out := &color{Name: c.system, A: 1}
	if c.system == "" {
		out = rgba(c.light)
	}
	if c.dark != 0 {
		out.Dark = rgba(c.dark)
	}
	return out
}

var (
	textStyles  = [...]string{"", "largeTitle", "title", "title2", "title3", "headline", "subheadline", "body", "callout", "footnote", "caption", "caption2"}
	fontDesigns = [...]string{"", "rounded", "serif", "monospaced"}
	dateStyles  = [...]string{"time", "date", "relative", "offset", "timer"}
	aligns      = [...]string{"", "start", "end"}
)

func pick[T ~uint8](names []string, v T) string {
	if int(v) < len(names) {
		return names[v]
	}
	return ""
}

func ptr[T any](v T) *T { return &v }

// encoder turns the app's views into the nodes the extension reads,
// collecting their images.
type encoder struct {
	images map[string][]byte // by file name
}

// node returns the node of v inside a view of the font parent.
func (e *encoder) node(v View, parent *font) (*node, error) {
	if v == nil {
		return nil, nil
	}
	n := v.node()
	if n == nil {
		return nil, nil
	}
	var s Style
	var children []View
	switch v := v.(type) {
	case Text:
		s = v.Style
	case Symbol:
		s = v.Style
	case Image:
		s = v.Style
		data := v.Data
		if v.File != "" {
			var err error
			if data, err = os.ReadFile(v.File); err != nil {
				return nil, err
			}
		}
		sum := sha256.Sum256(data)
		n.Image = imagePrefix + hex.EncodeToString(sum[:16])
		e.images[n.Image] = data
	case Date:
		s = v.Style
	case Column:
		s, children = v.Style, v.Children
	case Row:
		s, children = v.Style, v.Children
	case Stack:
		s, children = v.Style, v.Children
	case Divider:
		s = v.Style
	case Rectangle:
		s = v.Style
	case Progress:
		s = v.Style
	case Gauge:
		s = v.Style
	case Button:
		s, children = v.Style, v.Children
	}
	f := mergeFont(parent, s)
	if n.T == "text" || n.T == "symbol" || n.T == "date" {
		n.Font = f
	}
	applyStyle(n, s)
	for _, c := range children {
		cn, err := e.node(c, f)
		if err != nil {
			return nil, err
		}
		if cn != nil {
			n.Children = append(n.Children, cn)
		}
	}
	return n, nil
}

func (v Text) node() *node   { return &node{T: "text", Text: v.Content} }
func (v Symbol) node() *node { return &node{T: "symbol", Symbol: v.Name} }
func (v Image) node() *node  { return &node{T: "image", Cover: v.Cover} }
func (v Date) node() *node {
	return &node{T: "date", Date: ptr(float64(v.Time.UnixNano()) / 1e9), DateStyle: pick(dateStyles[:], v.Format)}
}
func (Column) node() *node  { return &node{T: "column"} }
func (Row) node() *node     { return &node{T: "row"} }
func (Stack) node() *node   { return &node{T: "stack"} }
func (Spacer) node() *node  { return &node{T: "spacer"} }
func (Divider) node() *node { return &node{T: "divider"} }
func (v Rectangle) node() *node {
	return &node{T: "rectangle", Color: encodeColor(v.Fill)}
}
func (v Progress) node() *node { return &node{T: "progress", Value: ptr(min(max(v.Value, 0), 1))} }
func (v Gauge) node() *node    { return &node{T: "gauge", Value: ptr(min(max(v.Value, 0), 1))} }
func (v Button) node() *node   { return &node{T: "button", Action: v.Action} }

// applyStyle sets a node's layout and look from s; its font is mergeFont's.
func applyStyle(n *node, s Style) {
	if s.Gap != 0 {
		n.Gap = ptr(float64(s.Gap))
	}
	n.Items, n.Justify = pick(aligns[:], s.AlignItems), pick(aligns[:], s.Justify)
	if p := s.Padding; p != (core.Edges{}) {
		n.Padding = &[4]float64{float64(p.Top), float64(p.Right), float64(p.Bottom), float64(p.Left)}
	}
	if s.Width > 0 {
		n.Width = ptr(float64(s.Width))
	}
	if s.Height > 0 {
		n.Height = ptr(float64(s.Height))
	}
	n.FillW, n.FillH = s.FillWidth, s.FillHeight
	if c := encodeColor(s.Background); c != nil {
		n.BG = c
	}
	if c := encodeColor(s.Color); c != nil && n.T != "rectangle" {
		n.Color = c
	}
	if s.CornerRadius > 0 {
		n.Radius = ptr(float64(s.CornerRadius))
	}
	if s.MaxLines > 0 {
		n.MaxLines = ptr(s.MaxLines)
	}
	if s.MinScale > 0 && s.MinScale < 1 {
		n.MinScale = ptr(float64(s.MinScale))
	}
	switch s.TextAlign {
	case core.TextAlignCenter:
		n.TextAlign = "center"
	case core.TextAlignEnd, core.TextAlignRight:
		n.TextAlign = "end"
	}
	n.Link, n.Accent = s.Link, s.Accented
}

// mergeFont returns the font of a view of style s inside a view of the
// font parent; nil for the system's.
func mergeFont(parent *font, s Style) *font {
	if s.FontSize == 0 && s.TextStyle == NoTextStyle && s.FontWeight == 0 && s.FontDesign == DefaultDesign && !s.Italic && s.FontVariant == 0 {
		return parent
	}
	var f font
	if parent != nil {
		f = *parent
	}
	switch {
	case s.FontSize > 0:
		f.Size, f.Style = ptr(float64(s.FontSize)), ""
	case s.TextStyle != NoTextStyle:
		f.Size, f.Style = nil, pick(textStyles[:], s.TextStyle)
	}
	if s.FontWeight != 0 {
		f.Weight = min(max(int(s.FontWeight), 100), 900)
	}
	if s.FontDesign != DefaultDesign {
		f.Design = pick(fontDesigns[:], s.FontDesign)
	}
	f.Italic = f.Italic || s.Italic
	if s.FontVariant&core.FontVariantNormal != 0 {
		f.Digits, f.SmallCaps = false, false
	}
	f.Digits = f.Digits || s.FontVariant&core.FontTabularNums != 0
	f.SmallCaps = f.SmallCaps || s.FontVariant&core.FontSmallCaps != 0
	return &f
}

// writeTimeline writes the timeline of entries into dir, with its images,
// and removes the images of the timeline it replaces. Zero dates are now.
func writeTimeline(dir string, entries []Entry, now time.Time) error {
	e := &encoder{images: map[string][]byte{}}
	tl := timeline{V: timelineVersion, Entries: []entry{}}
	for _, en := range entries {
		date := en.Date
		if date.IsZero() {
			date = now
		}
		out := entry{Date: float64(date.UnixNano()) / 1e9, URL: en.URL}
		var err error
		if out.View, err = e.node(en.View, nil); err != nil {
			return err
		}
		if out.BG, err = e.node(en.Background, nil); err != nil {
			return err
		}
		for fam, v := range en.Families {
			n, err := e.node(v, nil)
			if err != nil {
				return err
			}
			if n != nil {
				if out.Families == nil {
					out.Families = map[Family]*node{}
				}
				out.Families[fam] = n
			}
		}
		tl.Entries = append(tl.Entries, out)
	}
	slices.SortStableFunc(tl.Entries, func(a, b entry) int {
		switch {
		case a.Date < b.Date:
			return -1
		case a.Date > b.Date:
			return 1
		}
		return 0
	})
	data, err := json.Marshal(tl)
	if err != nil {
		return err
	}
	if err := writeImages(dir, e.images); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, timelineFile), data); err != nil {
		return err
	}
	return pruneImages(dir, e.images)
}

// writeImages writes images into dir, which it makes, by their names.
func writeImages(dir string, images map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, img := range images {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue // named by its contents
		}
		if err := writeAtomic(path, img); err != nil {
			return err
		}
	}
	return nil
}

// pruneImages removes the images of dir that are not among keep.
func pruneImages(dir string, keep map[string][]byte) error {
	old, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, f := range old {
		if name := f.Name(); strings.HasPrefix(name, imagePrefix) && keep[name] == nil {
			os.Remove(filepath.Join(dir, name))
		}
	}
	return nil
}

// writeAtomic writes a file through a temporary one renamed over it, so
// that the extension never reads half of it.
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(f.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("writing %s: %w", filepath.Base(path), err)
	}
	return nil
}
