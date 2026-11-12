// Package ui gives TSX nsgo/ui, package ui's components as Go components,
// for an app that imports this package for its effect; a theme of its
// own needs ui's roles, as core.DefaultTheme and ui.WarmDuskTheme have.
//
// Each takes its Go fields as props (tsx.Register): variants by name, and
// icons as a name Icons gives or an SVG document. Generic components take
// string values. What a Go app holds in Go values a JS app holds in props:
// a List's and a Tree's selection are lists of keys, a Tree's items are
// data, and toasts go through the ref of Toasts.
package ui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx"
	tsxdiff "github.com/nsgo-dev/nsgo/tsx/diff"
	tsxtextview "github.com/nsgo-dev/nsgo/tsx/textview"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/nsgo-dev/nsgo/ui/commitgraph"
	uidiff "github.com/nsgo-dev/nsgo/ui/diff"
	uiflow "github.com/nsgo-dev/nsgo/ui/flow"
)

const module = "nsgo/ui"

var (
	iconsMu sync.RWMutex
	icons   = map[string]func() *core.SVG{}
)

// Icons names the icons TSX's icon props take, as an app lists the Lucide
// icons it uses, which keeps the others out of its binary.
func Icons(named map[string]func() *core.SVG) {
	iconsMu.Lock()
	defer iconsMu.Unlock()
	for name, icon := range named {
		icons[name] = icon
	}
}

// readIcon reads an icon prop: an SVG document, or a name Icons gave.
func readIcon(v any) *core.SVG {
	s, _ := v.(string)
	if strings.HasPrefix(strings.TrimSpace(s), "<") {
		if svg, err := core.ParseSVG([]byte(s)); err == nil {
			return svg
		}
		return nil
	}
	iconsMu.RLock()
	defer iconsMu.RUnlock()
	if icon := icons[s]; icon != nil {
		return icon()
	}
	return nil
}

// roleTokens declares the colors the default theme adds, ui's roles
// among them, as names token() reads.
func roleTokens() string {
	names := slices.Sorted(maps.Keys(core.DefaultTheme().Light.Colors))
	var b strings.Builder
	b.WriteString("declare module \"nsgo\" {\n  interface Tokens {\n")
	for _, n := range names {
		fmt.Fprintf(&b, "    %q: string;\n", n)
	}
	b.WriteString("  }\n}")
	return b.String()
}

// enums name the variants and sizes of ui's components.
var enums = []tsx.Option{
	tsx.Enum(map[string]ui.AvatarStatus{"none": ui.AvatarNoStatus, "online": ui.AvatarOnline, "offline": ui.AvatarOffline}),
	tsx.Enum(map[string]ui.BadgeVariant{"neutral": ui.BadgeNeutral, "accent": ui.BadgeAccent, "info": ui.BadgeInfo, "warn": ui.BadgeWarn,
		"danger": ui.BadgeDanger, "up": ui.BadgeUp, "down": ui.BadgeDown, "outline": ui.BadgeOutline}),
	tsx.Enum(map[string]ui.ButtonVariant{"primary": ui.ButtonPrimary, "secondary": ui.ButtonSecondary, "outline": ui.ButtonOutline,
		"ghost": ui.ButtonGhost}),
	tsx.Enum(map[string]ui.CollapsibleVariant{"arrowRight": ui.CollapsibleArrowRight, "arrowLeft": ui.CollapsibleArrowLeft}),
	tsx.Enum(map[string]ui.HoldVariant{"accent": ui.HoldAccent, "danger": ui.HoldDanger, "neutral": ui.HoldNeutral}),
	tsx.Enum(map[string]ui.IconButtonVariant{"ghost": ui.IconButtonGhost, "primary": ui.IconButtonPrimary, "raised": ui.IconButtonRaised}),
	tsx.Enum(map[string]ui.MeterVariant{"neutral": ui.MeterNeutral, "accent": ui.MeterAccent, "info": ui.MeterInfo, "warn": ui.MeterWarn,
		"danger": ui.MeterDanger, "up": ui.MeterUp, "down": ui.MeterDown}),
	tsx.Enum(map[string]ui.MenuItemVariant{"default": ui.MenuItemDefault, "danger": ui.MenuItemDanger}),
	tsx.Enum(map[string]ui.Size{"default": ui.SizeDefault, "small": ui.SizeSmall, "large": ui.SizeLarge}),
	tsx.Enum(map[string]ui.SpinnerVariant{"ring": ui.SpinnerRing, "spokes": ui.SpinnerSpokes}),
	tsx.Enum(map[string]ui.StepStatus{"upcoming": ui.StepUpcoming, "current": ui.StepCurrent, "complete": ui.StepComplete, "failed": ui.StepFailed}),
	tsx.Enum(map[string]ui.SwitchVariant{"default": ui.SwitchDefault, "system": ui.SwitchSystem}),
	tsx.Enum(map[string]ui.TabsVariant{"segmented": ui.TabsSegmented, "underline": ui.TabsUnderline}),
	tsx.Enum(map[string]ui.ToastType{"plain": ui.ToastPlain, "info": ui.ToastInfo, "success": ui.ToastSuccess, "warning": ui.ToastWarning,
		"error": ui.ToastError}),
	tsx.Enum(map[string]ui.ToastPlacement{"top": ui.ToastTop, "topStart": ui.ToastTopStart, "topEnd": ui.ToastTopEnd, "bottom": ui.ToastBottom,
		"bottomStart": ui.ToastBottomStart, "bottomEnd": ui.ToastBottomEnd, "start": ui.ToastStart, "end": ui.ToastEnd}),
	tsx.Enum(map[string]ui.ToastSwipe{"edge": ui.ToastSwipeEdge, "left": ui.ToastSwipeLeft, "right": ui.ToastSwipeRight, "up": ui.ToastSwipeUp,
		"down": ui.ToastSwipeDown, "off": ui.ToastSwipeOff}),
	tsx.Enum(map[string]base.TimePrecision{"minute": base.TimeMinute, "second": base.TimeSecond}),
	tsx.Enum(map[string]base.HourCycle{"h23": base.HourCycle23, "h12": base.HourCycle12}),
	tsx.Enum(map[string]base.ColorFormat{"hex": base.ColorFormatHex, "rgb": base.ColorFormatRGB, "hsl": base.ColorFormatHSL,
		"hsb": base.ColorFormatHSB, "oklch": base.ColorFormatOKLCH}),
	tsx.Enum(map[string]core.ResizeMode{"cover": core.Cover, "contain": core.Contain, "stretch": core.Stretch, "center": core.Center}),
	tsx.Enum(map[string]commitgraph.RefKind{"branch": commitgraph.RefBranch, "remote": commitgraph.RefRemote, "tag": commitgraph.RefTag, "head": commitgraph.RefHead}),
	tsx.Enum(map[string]uiflow.Direction{"top-to-bottom": uiflow.TopToBottom, "left-to-right": uiflow.LeftToRight}),
	tsx.Enum(map[string]uiflow.CardVariant{"default": uiflow.CardDefault, "accent": uiflow.CardAccent}),
}

// options are enums with what one Register adds.
func options(extra ...tsx.Option) []tsx.Option {
	return append(append([]tsx.Option(nil), enums...), extra...)
}

func init() {
	tsx.Convert("Icon", "export type Icon = string;\n\n"+roleTokens(), readIcon, nil)
	tsx.Convert("DateText", `export type DateText = string | Date;`, func(v any) time.Time {
		switch d := v.(type) {
		case time.Time:
			return d
		case string:
			t, _ := time.Parse(time.RFC3339, d)
			if t.IsZero() {
				t, _ = time.Parse(time.DateOnly, d)
			}
			return t
		}
		return time.Time{}
	}, func(t time.Time) any { return t.Format(time.RFC3339) })

	// What data objects leave out is the default.
	tsx.Optional[ui.Button]("Flex", "Side", "IconSize")
	tsx.Optional[ui.AvatarGroupMember]("Image")
	tsx.Optional[ui.ToastContent]("Duration")
	tsx.Optional[ui.Command]("Inset")

	openReason := tsx.Event("OnOpenChange", "open", "reason")
	tsx.Register(module, map[string]core.Node{
		"Avatar": ui.Avatar{}, "AvatarGroup": ui.AvatarGroup{}, "Badge": ui.Badge{}, "Button": ui.Button{}, "HoldToConfirm": ui.HoldToConfirm{},
		"IconButton": ui.IconButton{}, "MenuItem": ui.MenuItem{}, "MenuLabel": ui.MenuLabel{}, "MenuSeparator": ui.MenuSeparator{},
		"Meter": ui.Meter{}, "NavHeader": ui.NavHeader{}, "PlainShell": ui.PlainShell{}, "Progress": ui.Progress{}, "ProgressRing": ui.ProgressRing{},
		"RadioGroup": ui.RadioGroup{}, "Sheet": ui.Sheet{}, "Skeleton": ui.Skeleton{}, "SkeletonBlock": ui.SkeletonBlock{},
		"SkeletonContent": ui.SkeletonContent{}, "Spacer": ui.Spacer{}, "Spinner": ui.Spinner{}, "Submenu": ui.Submenu{},
		"ToggleGroup": ui.ToggleGroup{}, "Toolbar": ui.Toolbar{}, "ToolbarGroup": ui.ToolbarGroup{}, "ToolbarSeparator": ui.ToolbarSeparator{},
	}, options(tsx.Omit("Scroll"))...)
	tsx.Register(module, map[string]core.Node{"Checkbox": ui.Checkbox{}, "Radio": ui.Radio{}, "Switch": ui.Switch{}},
		options(tsx.Event("OnChange", "checked"))...)
	tsx.Register(module, map[string]core.Node{"ToggleButton": ui.ToggleButton{}}, options(tsx.Event("OnChange", "on"))...)
	tsx.Register(module, map[string]core.Node{
		"NumberInput": ui.NumberInput{}, "OtpInput": ui.OtpInput{}, "Rating": ui.Rating{}, "Select": ui.Select[string]{}, "Slider": ui.Slider{},
		"Combobox": ui.Combobox[string]{}, "Tabs": ui.Tabs[string]{}, "TagInput": ui.TagInput{},
	}, options(tsx.Omit("Parse", "Ref"))...)
	tsx.Register(module, map[string]core.Node{"Calendar": ui.Calendar{}, "TimeField": ui.TimeField{}},
		options(tsx.Event("OnChange", "value"))...)
	tsx.Register(module, map[string]core.Node{"DatePicker": ui.DatePicker{}}, options(tsx.Event("OnChange", "value"), openReason)...)
	tsx.Register(module, map[string]core.Node{"ColorPicker": ui.ColorPicker{}},
		options(tsx.Event("OnChange", "value", "formatted"), tsx.Event("OnChangeEnd", "value", "formatted"))...)
	tsx.Register(module, map[string]core.Node{"ColorWell": ui.ColorWell{}}, options(tsx.Event("OnChange", "value"), openReason)...)
	tsx.Register(module, map[string]core.Node{"Accordion": ui.Accordion{}}, options(tsx.Event("OnOpenChange", "index", "open"))...)
	tsx.Register(module, map[string]core.Node{"Collapsible": ui.Collapsible{}}, options(tsx.Event("OnOpenChange", "open"))...)
	tsx.Register(module, map[string]core.Node{
		"AlertDialog": ui.AlertDialog{}, "ContextMenu": ui.ContextMenu{}, "Dialog": ui.Dialog{}, "HoverCard": ui.HoverCard{}, "Menu": ui.Menu{},
		"Popover": ui.Popover{}, "Tooltip": ui.Tooltip{},
	}, options(openReason)...)
	tsx.Register(module, map[string]core.Node{"Command": ui.Command{}, "CommandDialog": ui.CommandDialog{}},
		options(openReason, tsx.Omit("Filter", "Ref"), tsx.Event("OnSelect", "id"), tsx.Event("OnQueryChange", "query"),
			tsx.Event("OnHighlightChange", "highlighted"))...)
	tsx.Register(module, map[string]core.Node{"Breadcrumbs": ui.Breadcrumbs{}}, options(tsx.Event("OnSelect", "index"))...)
	tsx.Register(module, map[string]core.Node{"Stepper": ui.Stepper{}}, options(tsx.Event("OnSelect", "step"))...)
	tsx.Register(module, map[string]core.Node{"Pagination": ui.Pagination{}}, options(tsx.Event("OnChange", "page"))...)
	tsx.Register(module, map[string]core.Node{"Carousel": ui.Carousel{}}, options(tsx.Event("OnChange", "index"))...)
	tsx.Register(module, map[string]core.Node{"Sidebar": ui.Sidebar{}}, options(tsx.Event("OnSelect", "id"))...)
	tsx.Register(module, map[string]core.Node{"SidebarShell": ui.SidebarShell{}}, options(tsx.Event("OnOpenChange", "open"),
		tsx.Event("OnRightOpenChange", "open"), tsx.Event("OnWidthChange", "width"), tsx.Event("OnRightWidthChange", "width"))...)
	tsx.Register(module, map[string]core.Node{"Link": ui.Link{}}, options(tsx.Event("OnOpen", "href"))...)
	tsx.Register(module, map[string]core.Node{"Image": ui.Image{}}, options(tsx.Event("OnLoad", "pixels"))...)
	tsx.Register(module, map[string]core.Node{"ImageCompare": ui.ImageCompare{}}, options(tsx.Event("OnChange", "position"))...)
	tsx.Register(module, map[string]core.Node{"ScrollView": ui.ScrollView{}}, options()...)
	tsx.Register(module, map[string]core.Node{"ResizablePanelGroup": ui.ResizablePanelGroup{}}, options(tsx.Event("OnResize", "sizes"))...)
	tsx.Register(module, map[string]core.Node{"DockArea": ui.DockArea{}}, tsx.DockOptions()...)
	tsx.Register(module, map[string]core.Node{"DockTabLabel": ui.DockTabLabel{}}, options()...)
	tsx.Register(module, map[string]core.Node{"CommitGraph": commitgraph.View{}}, options(tsx.Omit("Selection"), tsx.Event("OnActivate", "commit"))...)
	tsx.Register(module, map[string]core.Node{"Flow": uiflow.View{}}, options(tsx.Omit("State"), tsx.Event("OnSelect", "id"), tsx.Event("OnOpen", "id"))...)
	tsx.Register(module, map[string]core.Node{"FlowCard": uiflow.Card{}}, options()...)
	tsx.Register(module, map[string]core.Node{"DiffView": uidiff.View{}}, tsxdiff.Options()...)
	tsx.Register(module, map[string]core.Node{"TextView": ui.TextView{}}, append(tsxtextview.Options(), tsx.Omit("StyleOf"))...)
	tsx.Register(module, map[string]core.Node{"Input": ui.Input{}, "Field": ui.Field{}}, options(tsx.Event("OnChange", "value"), tsx.Event("OnSubmit", "text"))...)
	tsx.Register(module, map[string]core.Node{"Icon": Icon{}}, options()...)
	tsx.Register(module, map[string]core.Node{"List": List{}}, options(tsx.Event("OnSelectedChange", "selected"), tsx.Event("OnActivate", "index"))...)
	tsx.Register(module, map[string]core.Node{"Tree": Tree{}}, options(tsx.Event("OnSelectedChange", "selected"), tsx.Event("OnActivate", "id"))...)
	tsx.Register(module, map[string]core.Node{"Toasts": Toasts{}}, options()...)
	tsx.Register(module, map[string]core.Node{"TabShell": TabShell{}}, options(tsx.Event("OnChange", "index"))...)
}

// Icon draws Name, Size points square in Color, the text's when nil.
type Icon struct {
	Name  func() *core.SVG
	Size  core.Points
	Color func() core.Color
}

// Render implements core.Node.
func (i Icon) Render(cx *core.Scope) core.Node {
	size := i.Size
	if size == 0 {
		size = 16
	}
	color := i.Color
	if color == nil {
		color = ui.ColorsOf(cx).Text
	}
	return ui.IconOf(i.Name, size, color)
}

// ShellTab is a tab of a TabShell.
type ShellTab struct {
	Text       string
	Icon       func() *core.SVG
	SystemIcon string
	Content    core.Node
	TestID     string
}

// TabShell is ui.TabShell with each tab's Content a node.
type TabShell struct {
	Tabs     []ShellTab
	Selected func() int
	OnChange func(index int)
	Label    string
	TestID   string
}

// Render implements core.Node.
func (t TabShell) Render(*core.Scope) core.Node {
	tabs := make([]ui.ShellTab, len(t.Tabs))
	for i, tab := range t.Tabs {
		tabs[i] = ui.ShellTab{Text: tab.Text, Icon: tab.Icon, SystemIcon: tab.SystemIcon, TestID: tab.TestID,
			Content: func(*core.Scope) core.Node { return tab.Content }}
	}
	return ui.TabShell{Tabs: tabs, Selected: t.Selected, OnChange: t.OnChange, Label: t.Label, TestID: t.TestID}
}
