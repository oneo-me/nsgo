package ui

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// IconName names one of the gallery's line glyphs. NSH draws its own in a
// canvas; NSGo bundles Lucide, whose icons core.Icon draws in the tint.
type IconName string

const (
	IconPhotos     IconName = "photos"
	IconLists      IconName = "lists"
	IconScroll     IconName = "scroll"
	IconNav        IconName = "nav"
	IconForm       IconName = "form"
	IconMobile     IconName = "mobile"
	IconComponents IconName = "components"
	IconControls   IconName = "controls"
	IconGestures   IconName = "gestures"
	IconPhysics    IconName = "physics"
	IconAnimation  IconName = "animation"
	IconImage      IconName = "image"
	IconCanvas     IconName = "canvas"
	IconShader     IconName = "shader"
	IconStyles     IconName = "styles"
	IconText       IconName = "text"
	IconMedia      IconName = "media"
	IconSystem     IconName = "system"
	IconStorage    IconName = "storage"
	IconStocks     IconName = "stocks"
	IconChat       IconName = "chat"
	IconAccess     IconName = "accessibility"
	IconLayout     IconName = "layout"
	IconMenu       IconName = "menu"
	IconDock       IconName = "dock"
	IconOverlays   IconName = "overlays"
	IconChevron    IconName = "chevron"
	IconBack       IconName = "back"
	IconCheck      IconName = "check"
	IconPlus       IconName = "plus"
	IconMinus      IconName = "minus"
	IconClose      IconName = "close"
	IconArrowUp    IconName = "arrow-up"
	IconArrowDown  IconName = "arrow-down"
	IconPlay       IconName = "play"
	IconPause      IconName = "pause"
	IconSearch     IconName = "search"
	IconStar       IconName = "star"
	IconBell       IconName = "bell"
	IconSpark      IconName = "spark"
	IconSidebar    IconName = "sidebar"
	IconSun        IconName = "sun"
	IconMoon       IconName = "moon"
	IconRefresh    IconName = "refresh"
	IconSend       IconName = "send"
	IconStop       IconName = "stop"
	IconAttach     IconName = "attach"
	IconCopy       IconName = "copy"
	IconTrash      IconName = "trash"
	IconFile       IconName = "file"
	IconTerminal   IconName = "terminal"
	IconCode       IconName = "code"
	IconSliders    IconName = "sliders"
	IconReplace    IconName = "replace"
	IconReplaceAll IconName = "replace-all"
	IconProblems   IconName = "diagnostics"

	// The chat's.
	IconRegenerate  IconName = "regenerate"
	IconThumbUp     IconName = "thumb-up"
	IconThumbDown   IconName = "thumb-down"
	IconGlobe       IconName = "globe"
	IconCaretDown   IconName = "caret-down"
	IconAlert       IconName = "alert"
	IconOffline     IconName = "offline"
	IconBrain       IconName = "brain"
	IconFileText    IconName = "file-text"
	IconCheckbox    IconName = "checkbox"
	IconCollapsible IconName = "collapsible"
	IconTable       IconName = "table"
	IconDiff        IconName = "diff"
	IconChart       IconName = "chart"
	IconPopover     IconName = "popover"
	IconProgress    IconName = "progress"
	IconRadio       IconName = "radio"
	IconSheet       IconName = "sheet"
	IconSwitch      IconName = "switch"
	IconTabs        IconName = "tabs"
	IconTooltip     IconName = "tooltip"
	IconHeader      IconName = "header"
	IconSettings    IconName = "settings"

	// The components'.
	IconChevronUp   IconName = "chevron-up"
	IconChevronDown IconName = "chevron-down"
	IconMore        IconName = "more"
	IconImageOff    IconName = "image-off"
	IconSplitX      IconName = "split-x"
	IconSplitY      IconName = "split-y"
	IconInfo        IconName = "info"
	IconSuccess     IconName = "success"
	IconError       IconName = "error"
)

var glyphs = map[IconName]func() *core.SVG{
	IconPhotos:      lucide.Images,
	IconLists:       lucide.List,
	IconScroll:      lucide.ScrollText,
	IconNav:         lucide.PanelsTopLeft,
	IconForm:        lucide.TextCursorInput,
	IconMobile:      lucide.Smartphone,
	IconComponents:  lucide.Component,
	IconControls:    lucide.SlidersHorizontal,
	IconGestures:    lucide.Hand,
	IconPhysics:     lucide.Atom,
	IconAnimation:   lucide.Zap,
	IconImage:       lucide.Image,
	IconCanvas:      lucide.PenTool,
	IconShader:      lucide.Sparkles,
	IconStyles:      lucide.Palette,
	IconText:        lucide.Type,
	IconMedia:       lucide.Film,
	IconSystem:      lucide.Cpu,
	IconStorage:     lucide.Database,
	IconStocks:      lucide.ChartCandlestick,
	IconChat:        lucide.MessageCircle,
	IconAccess:      lucide.Accessibility,
	IconLayout:      lucide.Columns2,
	IconMenu:        lucide.Menu,
	IconDock:        lucide.AppWindow,
	IconOverlays:    lucide.Layers,
	IconChevron:     lucide.ChevronRight,
	IconBack:        lucide.ChevronLeft,
	IconCheck:       lucide.Check,
	IconPlus:        lucide.Plus,
	IconMinus:       lucide.Minus,
	IconClose:       lucide.X,
	IconArrowUp:     lucide.ArrowUp,
	IconArrowDown:   lucide.ArrowDown,
	IconPlay:        lucide.Play,
	IconPause:       lucide.Pause,
	IconSearch:      lucide.Search,
	IconStar:        lucide.Star,
	IconBell:        lucide.Bell,
	IconSpark:       lucide.Sparkles,
	IconSidebar:     lucide.PanelLeft,
	IconSun:         lucide.Sun,
	IconMoon:        lucide.Moon,
	IconRefresh:     lucide.RefreshCw,
	IconSend:        lucide.Send,
	IconStop:        lucide.Square,
	IconAttach:      lucide.Paperclip,
	IconCopy:        lucide.Copy,
	IconTrash:       lucide.Trash,
	IconFile:        lucide.File,
	IconTerminal:    lucide.Terminal,
	IconCode:        lucide.Code,
	IconSliders:     lucide.SlidersHorizontal,
	IconReplace:     lucide.Replace,
	IconReplaceAll:  lucide.ReplaceAll,
	IconProblems:    lucide.TriangleAlert,
	IconRegenerate:  lucide.RefreshCw,
	IconThumbUp:     lucide.ThumbsUp,
	IconThumbDown:   lucide.ThumbsDown,
	IconGlobe:       lucide.Globe,
	IconCaretDown:   lucide.ChevronDown,
	IconAlert:       lucide.CircleAlert,
	IconOffline:     lucide.WifiOff,
	IconBrain:       lucide.Brain,
	IconFileText:    lucide.FileText,
	IconCheckbox:    lucide.SquareCheck,
	IconCollapsible: lucide.ChevronsUpDown,
	IconTable:       lucide.Table,
	IconDiff:        lucide.FileDiff,
	IconChart:       lucide.ChartSpline,
	IconPopover:     lucide.MessageSquare,
	IconProgress:    lucide.LoaderCircle,
	IconRadio:       lucide.CircleDot,
	IconSheet:       lucide.PanelBottom,
	IconSwitch:      lucide.ToggleRight,
	IconTabs:        lucide.NotebookTabs,
	IconTooltip:     lucide.MessageSquareText,
	IconHeader:      lucide.PanelTop,
	IconSettings:    lucide.Settings,
	IconChevronUp:   lucide.ChevronUp,
	IconChevronDown: lucide.ChevronDown,
	IconMore:        lucide.Ellipsis,
	IconImageOff:    lucide.ImageOff,
	IconSplitX:      lucide.ChevronsLeftRight,
	IconSplitY:      lucide.ChevronsUpDown,
	IconInfo:        lucide.Info,
	IconSuccess:     lucide.CircleCheck,
	IconError:       lucide.CircleX,
}

// RegisterIcon names svg name, for the app's own glyphs beside these.
// Call it before building a tree that names it.
func RegisterIcon(name IconName, svg func() *core.SVG) { glyphs[name] = svg }

// glyph is the glyph name names, nil for none.
func glyph(name IconName) *core.SVG {
	if fn := glyphs[name]; fn != nil {
		return fn()
	}
	return nil
}

// Icon draws a glyph size points square in the tint, which it follows;
// nothing for a name not registered.
func Icon(name IconName, size core.Points, tint func() core.Color) core.Node {
	return core.Icon{SVG: glyph(name), Style: core.Style{Width: core.Pt(size), Height: core.Pt(size)},
		StyleFunc: func() core.Style { return core.Style{Color: tint()} }}
}

// IconOf draws the glyph name returns, which it follows.
func IconOf(name func() IconName, size core.Points, tint func() core.Color) core.Node {
	ic := Icon("", size, tint).(core.Icon)
	ic.SVGFunc = func() *core.SVG { return glyph(name()) }
	return ic
}
