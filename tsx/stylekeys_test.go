package tsx

import (
	"log"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

func TestTransitionPropertiesNameLayout(t *testing.T) {
	for s, layout := range map[string]bool{"width": true, "max-height, opacity": true, "inset": true, "all": true,
		"colors": false, "transform": false} {
		if got := transitionProperties(s)&core.TransitionLayout != 0; got != layout {
			t.Errorf("%q: layout %v, want %v", s, got, layout)
		}
	}
}

// decoding is a runtime with no app, for what decodes values alone.
func decoding() *runtime {
	return &runtime{vm: goja.New(), warned: map[string]bool{}}
}

// jsValue evaluates a JS expression in r.
func jsValue(t *testing.T, r *runtime, expr string) goja.Value {
	t.Helper()
	v, err := r.vm.RunString("(" + expr + ")")
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	return v
}

// TestStyleKeysSetTheirFields decodes each style and checks the field it
// sets, with no warning.
func TestStyleKeysSetTheirFields(t *testing.T) {
	transform := func(s core.Style) core.Transform { return s.Transform }
	cases := map[string]func(core.Style) bool{
		`{display: "flow-root"}`:                         func(s core.Style) bool { return s.Display == core.DisplayBlock },
		`{display: "flex"}`:                              func(s core.Style) bool { return s.Display == core.DisplayFlex && s.RowByDefault },
		`{display: "grid"}`:                              func(s core.Style) bool { return s.Display == core.DisplayGrid && !s.RowByDefault },
		`{boxSizing: "content-box"}`:                     func(s core.Style) bool { return s.BoxSizing == core.ContentBox },
		`{position: "sticky"}`:                           func(s core.Style) bool { return s.Position == core.Sticky },
		`{flexDirection: "row-reverse"}`:                 func(s core.Style) bool { return s.Direction == core.RowReverse },
		`{flexWrap: "wrap-reverse"}`:                     func(s core.Style) bool { return s.Wrap == core.WrapReverse },
		`{flex: 2}`:                                      func(s core.Style) bool { return s.FlexGrow == 2 && s.FlexShrink == 1 && s.FlexBasis == core.Pt(0) },
		`{flex: "none"}`:                                 func(s core.Style) bool { return s.FlexGrow == 0 && s.FlexShrink == 0 && s.FlexBasis == core.Length{} },
		`{flex: "auto"}`:                                 func(s core.Style) bool { return s.FlexGrow == 1 && s.FlexShrink == 1 },
		`{flex: "initial"}`:                              func(s core.Style) bool { return s.FlexGrow == 0 && s.FlexShrink == 1 },
		`{flex: "2 3 10px"}`:                             func(s core.Style) bool { return s.FlexGrow == 2 && s.FlexShrink == 3 && s.FlexBasis == core.Pt(10) },
		`{flexGrow: 2, flexShrink: 3, flexBasis: "50%"}`: func(s core.Style) bool { return s.FlexGrow == 2 && s.FlexShrink == 3 && s.FlexBasis == core.Pct(50) },
		`{alignSelf: "center", alignContent: "center", justifyItems: "center", justifySelf: "center"}`: func(s core.Style) bool {
			return s.AlignSelf != 0 && s.AlignSelf == s.AlignContent && s.JustifyItems == s.JustifySelf && s.AlignSelf == s.JustifySelf
		},
		`{minWidth: 1, minHeight: 2, maxWidth: 3, maxHeight: "50%"}`: func(s core.Style) bool {
			return s.MinWidth == core.Pt(1) && s.MinHeight == core.Pt(2) && s.MaxWidth == core.Pt(3) && s.MaxHeight == core.Pct(50)
		},
		`{aspectRatio: 2}`:        func(s core.Style) bool { return s.AspectRatio == 2 },
		`{aspectRatio: "16 / 8"}`: func(s core.Style) bool { return s.AspectRatio == 2 },
		`{top: 1, right: 2, bottom: 3, left: 4}`: func(s core.Style) bool {
			return s.Top == core.Pt(1) && s.Right == core.Pt(2) && s.Bottom == core.Pt(3) && s.Left == core.Pt(4) &&
				s.PhysicalEdges.Inset == core.SideLeft|core.SideRight
		},
		`{start: 1, end: 2}`: func(s core.Style) bool {
			return s.Left == core.Pt(1) && s.Right == core.Pt(2) && s.PhysicalEdges.Inset == 0
		},
		`{right: 9, end: 2}`: func(s core.Style) bool { return s.Right == core.Pt(2) && s.PhysicalEdges.Inset == 0 },
		`{zIndex: 3}`:        func(s core.Style) bool { return s.ZIndex == 3 },
		`{direction: "rtl"}`: func(s core.Style) bool { return s.LayoutDirection == core.DirectionRTL },
		`{marginHorizontal: 1, marginVertical: 2}`: func(s core.Style) bool {
			return s.Margin.Left == 1 && s.Margin.Right == 1 && s.Margin.Top == 2 && s.Margin.Bottom == 2
		},
		`{margin: 1, marginLeft: 2, marginStart: 3}`: func(s core.Style) bool {
			return s.Margin.Top == 1 && s.Margin.Left == 3 && s.PhysicalEdges.Margin == 0
		},
		`{paddingHorizontal: 1, paddingVertical: 2}`: func(s core.Style) bool {
			return s.Padding.Left == 1 && s.Padding.Bottom == 2
		},
		`{paddingRight: 4, paddingLeft: 5}`: func(s core.Style) bool {
			return s.Padding.Right == 4 && s.Padding.Left == 5 && s.PhysicalEdges.Padding == core.SideLeft|core.SideRight
		},
		`{gap: 1, rowGap: 2, columnGap: 3}`: func(s core.Style) bool { return s.Gap == 1 && s.RowGap == 2 && s.ColumnGap == 3 },
		`{gridTemplateColumns: "1fr 2fr", gridTemplateRows: "10px"}`: func(s core.Style) bool {
			return s.GridTemplateColumns == core.GridTracks("1fr 2fr") && s.GridTemplateRows == core.GridTracks("10px")
		},
		`{gridAutoColumns: "1fr", gridAutoRows: "10px"}`: func(s core.Style) bool {
			return s.GridAutoColumns == core.GridTracks("1fr") && s.GridAutoRows == core.GridTracks("10px")
		},
		`{gridAutoFlow: "column dense"}`: func(s core.Style) bool { return s.GridAutoFlow == core.FlowColumnDense },
		`{gridRow: "1 / 3"}`:             func(s core.Style) bool { return s.GridRow != core.GridLines{} },
		`{gridColumnStart: "2", gridColumnEnd: "span 2", gridRowStart: "1", gridRowEnd: "3"}`: func(s core.Style) bool {
			return s.GridColumn.Start != s.GridColumn.End && s.GridRow.Start != s.GridRow.End
		},
		`{numberOfLines: 2}`:                              func(s core.Style) bool { return s.NumberOfLines == 2 },
		`{numberOfLines: 0}`:                              func(s core.Style) bool { return s.NumberOfLines == core.NoLineLimit },
		`{textOverflow: "ellipsis-middle"}`:               func(s core.Style) bool { return s.TextOverflow == core.EllipsizeMiddle },
		`{userSelect: "text"}`:                            func(s core.Style) bool { return s.UserSelect == core.UserSelectText },
		`{pointerEvents: "box-only"}`:                     func(s core.Style) bool { return s.PointerEvents == core.PointerEventsBoxOnly },
		`{objectFit: "scale-down"}`:                       func(s core.Style) bool { return s.ObjectFit == core.ScaleDown },
		`{backgroundColor: "#ff0000"}`:                    func(s core.Style) bool { return s.Background == core.RGB(0xff0000) },
		`{backgroundImage: "linear-gradient(red, blue)"}`: func(s core.Style) bool { return len(s.Gradients) == 1 },
		`{borderRadius: 4}`:                               func(s core.Style) bool { return s.CornerRadius == 4 && s.CornerRadii == core.AllCorners(4) },
		`{borderTopStartRadius: 1, borderTopEndRadius: 2, borderBottomEndRadius: 3, borderBottomStartRadius: 4}`: func(s core.Style) bool {
			return s.CornerRadii == core.Corners{TopLeft: 1, TopRight: 2, BottomRight: 3, BottomLeft: 4} && s.PhysicalBorders.Corners == 0
		},
		`{borderTopLeftRadius: 1, borderTopRightRadius: 2, borderBottomRightRadius: 3, borderBottomLeftRadius: 4}`: func(s core.Style) bool {
			return s.CornerRadii == core.Corners{TopLeft: 1, TopRight: 2, BottomRight: 3, BottomLeft: 4} &&
				s.PhysicalBorders.Corners == core.SideLeft|core.SideRight
		},
		`{borderCurve: "continuous"}`: func(s core.Style) bool { return s.BorderCurve == core.BorderCurveContinuous },
		`{borderWidth: 2}`:            func(s core.Style) bool { return s.BorderWidth == 2 && s.Borders == core.All(2) },
		`{borderTopWidth: 1, borderBottomWidth: 2, borderStartWidth: 3, borderEndWidth: 4}`: func(s core.Style) bool {
			return s.Borders == core.Edges{Top: 1, Bottom: 2, Left: 3, Right: 4} && s.PhysicalBorders.Widths == 0
		},
		`{borderLeftWidth: 3, borderRightWidth: 4}`: func(s core.Style) bool {
			return s.PhysicalBorders.Widths == core.SideLeft|core.SideRight
		},
		`{borderColor: "#010203"}`: func(s core.Style) bool {
			return s.BorderColor == core.RGB(0x010203) && s.BorderColors == core.AllColors(core.RGB(0x010203))
		},
		`{borderTopColor: "#000001", borderBottomColor: "#000002", borderStartColor: "#000003", borderEndColor: "#000004"}`: func(s core.Style) bool {
			return s.BorderColors.Top == core.RGB(1) && s.BorderColors.Bottom == core.RGB(2) &&
				s.BorderColors.Left == core.RGB(3) && s.BorderColors.Right == core.RGB(4) && s.PhysicalBorders.Colors == 0
		},
		`{borderLeftColor: "#000003", borderRightColor: "#000004"}`: func(s core.Style) bool {
			return s.PhysicalBorders.Colors == core.SideLeft|core.SideRight
		},
		`{borderStyle: "dashed", outlineStyle: "dashed"}`: func(s core.Style) bool {
			return s.BorderStyle != 0 && s.BorderStyle == s.OutlineStyle
		},
		`{outlineWidth: 2, outlineColor: "#000001", outlineOffset: 3}`: func(s core.Style) bool {
			return s.OutlineWidth == 2 && s.OutlineColor == core.RGB(1) && s.OutlineOffset == 3
		},
		`{opacity: 0.5}`:                               func(s core.Style) bool { return s.Opacity == core.Alpha(0.5) },
		`{visibility: "collapse"}`:                     func(s core.Style) bool { return s.Visibility == core.VisibilityHidden },
		`{overflow: "scroll"}`:                         func(s core.Style) bool { return s.Overflow == core.OverflowScroll },
		`{boxShadow: "0 1px 2px red, 0 3px 4px blue"}`: func(s core.Style) bool { return len(s.Shadows) == 2 },
		`{shadowOpacity: 0.5, shadowColor: "#ff0000", shadowOffset: {width: 1, height: 2}, shadowRadius: 3}`: func(s core.Style) bool {
			return s.ShadowColor == core.RGB(0xff0000).Faded(0.5) && s.ShadowOffset == core.Point{X: 1, Y: 2} && s.ShadowRadius == 3
		},
		`{shadowOpacity: 0.5}`: func(s core.Style) bool { return s.ShadowColor == core.RGB(0).Faded(0.5) },
		`{filter: "blur(2px)", backdropFilter: "blur(3px) saturate(2)"}`: func(s core.Style) bool {
			return len(s.Filters) == 1 && len(s.BackdropFilters) == 2
		},
		`{maskImage: "linear-gradient(red, transparent)"}`: func(s core.Style) bool { return s.Mask != nil },
		`{glassEffect: "clear"}`:                           func(s core.Style) bool { return s.GlassEffect == core.GlassClear },
		`{mixBlendMode: "multiply"}`:                       func(s core.Style) bool { return s.BlendMode != 0 },
		`{cursor: "text"}`:                                 func(s core.Style) bool { return s.Cursor != core.CursorAuto },
		`{cursor: "no-such-cursor"}`:                       func(s core.Style) bool { return s.Cursor == core.CursorAuto },
		`{backfaceVisibility: "hidden"}`:                   func(s core.Style) bool { return s.BackfaceHidden },
		`{transform: "rotate(45deg) scale(2)"}`:            func(s core.Style) bool { return len(s.Transforms) == 3 },
		`{translate: "10px 50%"}`: func(s core.Style) bool {
			return transform(s).TranslateX == 10 && transform(s).TranslateYPercent == 50
		},
		`{scale: "2 3"}`:                    func(s core.Style) bool { return transform(s).ScaleX == 2 && transform(s).ScaleY == 3 },
		`{scale: 2}`:                        func(s core.Style) bool { return transform(s).ScaleX == 2 && transform(s).ScaleY == 2 },
		`{rotate: "45deg"}`:                 func(s core.Style) bool { return transform(s).Rotate == 45 },
		`{transformOrigin: "left top 4px"}`: func(s core.Style) bool { return s.TransformOriginZ == 4 },
		`{transformOrigin: [1, 2, 3]}`:      func(s core.Style) bool { return s.TransformOriginZ == 3 },
		`{transitionProperty: "opacity", transitionDuration: 200, transitionDelay: "1s", transitionTimingFunction: "ease-in"}`: func(s core.Style) bool {
			return s.Transition.Properties != 0 && s.Transition.Duration == 200*time.Millisecond && s.Transition.Delay == time.Second
		},
		`{animationName: "spin", animationDuration: 200, animationDelay: 100, animationTimingFunction: "linear", animationIterationCount: 3}`: func(s core.Style) bool {
			return s.Animation.Keyframes == core.KeyframesSpin && s.Animation.Duration == 200*time.Millisecond &&
				s.Animation.Delay == 100*time.Millisecond && s.Animation.Iterations == 3
		},
		`{animationIterationCount: "infinite"}`:       func(s core.Style) bool { return s.Animation.Iterations == 0 },
		`{strokeWidth: 2, strokeDasharray: "1, 2 3"}`: func(s core.Style) bool { return s.StrokeWidth == 2 && len(s.StrokeDash) == 3 },
		`{verticalAlign: "middle"}`:                   func(s core.Style) bool { return s.VerticalAlign == core.VerticalAlignMiddle },
		`{tintColor: "#000001"}`:                      func(s core.Style) bool { return s.TintColor == core.RGB(1) },
		`{color: "#000001", fontSize: 12, fontFamily: "Mono", fontStyle: "italic"}`: func(s core.Style) bool {
			return s.Color == core.RGB(1) && s.FontSize == 12 && s.FontFamily == "Mono" && s.FontStyle == core.FontStyleItalic
		},
		`{lineHeight: 20, letterSpacing: 2}`:            func(s core.Style) bool { return s.LineHeight == 20 && s.LetterSpacing == 2 },
		`{lineHeight: "1.5em", letterSpacing: "0.1em"}`: func(s core.Style) bool { return s.LineHeightEm == 1.5 && s.LetterSpacingEm == 0.1 },
		`{textAlign: "justify", textAlignVertical: "center"}`: func(s core.Style) bool {
			return s.TextAlign == core.TextAlignJustify && s.TextAlignVertical == core.TextAlignVerticalCenter
		},
		`{textDecorationLine: "none"}`: func(s core.Style) bool { return s.TextDecoration == core.TextDecorationNone },
		`{textDecorationStyle: "wavy", textDecorationColor: "#000001", textDecorationThickness: 2, textUnderlineOffset: 3}`: func(s core.Style) bool {
			return s.TextDecorationStyle == core.DecorationWavy && s.TextDecorationColor == core.RGB(1) &&
				s.TextDecorationThickness == core.Pt(2) && s.TextUnderlineOffset == core.Pt(3)
		},
		`{textDecorationThickness: "auto"}`:         func(s core.Style) bool { return s.TextDecorationThickness == core.Length{} },
		`{textShadow: "0 0 1px red, 0 0 5px blue"}`: func(s core.Style) bool { return s.TextShadowRadius == 5 },
		`{textShadowColor: "#000001", textShadowOffset: {width: 1, height: 2}, textShadowRadius: 3}`: func(s core.Style) bool {
			return s.TextShadowColor == core.RGB(1) && s.TextShadowOffset == core.Point{X: 1, Y: 2} && s.TextShadowRadius == 3
		},
		`{writingDirection: "rtl"}`: func(s core.Style) bool { return s.WritingDirection == core.WritingRTL },
		`{placeholderColor: "#000001", selectionColor: "#000002", caretColor: "#000003"}`: func(s core.Style) bool {
			return s.PlaceholderColor == core.RGB(1) && s.SelectionColor == core.RGB(2) && s.CaretColor == core.RGB(3)
		},
		`{fontWeight: "bold"}`:                          func(s core.Style) bool { return s.FontWeight == core.FontWeight(700) },
		`{fontVariant: ["small-caps", "tabular-nums"]}`: func(s core.Style) bool { return s.FontVariant != core.FontVariantNormal },
	}
	for js, ok := range cases {
		t.Run(js, func(t *testing.T) {
			r := decoding()
			s := r.style(jsValue(t, r, js))
			if !ok(s) {
				t.Errorf("decoded %+v", s)
			}
			if len(r.warned) > 0 {
				t.Errorf("warned %v", r.warned)
			}
		})
	}
}

func TestStyleArraysMergeLaterOverEarlier(t *testing.T) {
	r := decoding()
	s := r.style(jsValue(t, r, `[{width: 1, height: 2}, null, false, {width: 3}]`))
	if s.Width != core.Pt(3) || s.Height != core.Pt(2) {
		t.Errorf("merged %+v", s)
	}
}

func TestStyleLonghandsGoOverShorthandsInAnyOrder(t *testing.T) {
	r := decoding()
	s := r.style(jsValue(t, r, `{paddingTop: 1, padding: 2, marginLeft: 3, marginStart: 4, margin: 5}`))
	if s.Padding.Top != 1 || s.Padding.Left != 2 || s.Margin.Left != 4 || s.Margin.Top != 5 {
		t.Errorf("padding %+v, margin %+v", s.Padding, s.Margin)
	}
}

// TestUnknownStyleValuesWarnOnce checks what an app misspells is reported
// and leaves the style as it was.
func TestUnknownStyleValuesWarnOnce(t *testing.T) {
	var logged strings.Builder
	prev := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(prev) })

	r := decoding()
	s := r.style(jsValue(t, r, `[{nope: 1}, {nope: 2}, {display: "wobbly"}, {flex: "a b c d"}, {textDecorationLine: "sideways"}]`))
	if s.Display != 0 {
		t.Errorf("display %v", s.Display)
	}
	got := make([]string, 0, len(r.warned))
	for w := range r.warned {
		got = append(got, w)
	}
	slices.Sort(got)
	if len(got) != 4 || !slices.Contains(got, `style key "nope" is not supported`) {
		t.Errorf("warned %q", got)
	}
	if n := strings.Count(logged.String(), "nope"); n != 1 {
		t.Errorf("%q warned %d times", "nope", n)
	}
}
