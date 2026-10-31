package main

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// iconName names one of the gallery's line glyphs. NSH draws its own in a
// canvas; NSGo bundles Lucide, whose icons core.Icon draws in the tint.
type iconName string

const (
	iconPhotos     iconName = "photos"
	iconLists      iconName = "lists"
	iconScroll     iconName = "scroll"
	iconNav        iconName = "nav"
	iconForm       iconName = "form"
	iconMobile     iconName = "mobile"
	iconComponents iconName = "components"
	iconControls   iconName = "controls"
	iconGestures   iconName = "gestures"
	iconPhysics    iconName = "physics"
	iconAnimation  iconName = "animation"
	iconImage      iconName = "image"
	iconCanvas     iconName = "canvas"
	iconShader     iconName = "shader"
	iconStyles     iconName = "styles"
	iconText       iconName = "text"
	iconMedia      iconName = "media"
	iconSystem     iconName = "system"
	iconStorage    iconName = "storage"
	iconStocks     iconName = "stocks"
	iconChat       iconName = "chat"
	iconAccess     iconName = "accessibility"
	iconLayout     iconName = "layout"
	iconMenu       iconName = "menu"
	iconDock       iconName = "dock"
	iconOverlays   iconName = "overlays"
	iconChevron    iconName = "chevron"
	iconBack       iconName = "back"
	iconCheck      iconName = "check"
	iconPlus       iconName = "plus"
	iconMinus      iconName = "minus"
	iconClose      iconName = "close"
	iconArrowUp    iconName = "arrow-up"
	iconArrowDown  iconName = "arrow-down"
	iconPlay       iconName = "play"
	iconPause      iconName = "pause"
	iconSearch     iconName = "search"
	iconStar       iconName = "star"
	iconBell       iconName = "bell"
	iconSpark      iconName = "spark"
	iconSidebar    iconName = "sidebar"
	iconSun        iconName = "sun"
	iconMoon       iconName = "moon"
	iconRefresh    iconName = "refresh"
	iconSend       iconName = "send"
	iconStop       iconName = "stop"
	iconAttach     iconName = "attach"
	iconCopy       iconName = "copy"
	iconTrash      iconName = "trash"
	iconFile       iconName = "file"
	iconTerminal   iconName = "terminal"
	iconCode       iconName = "code"
	iconSliders    iconName = "sliders"
	iconReplace    iconName = "replace"
	iconReplaceAll iconName = "replace-all"
	iconProblems   iconName = "diagnostics"
)

var glyphs = map[iconName]func() *core.SVG{
	iconPhotos:     lucide.Images,
	iconLists:      lucide.List,
	iconScroll:     lucide.ScrollText,
	iconNav:        lucide.PanelsTopLeft,
	iconForm:       lucide.TextCursorInput,
	iconMobile:     lucide.Smartphone,
	iconComponents: lucide.Component,
	iconControls:   lucide.SlidersHorizontal,
	iconGestures:   lucide.Hand,
	iconPhysics:    lucide.Atom,
	iconAnimation:  lucide.Zap,
	iconImage:      lucide.Image,
	iconCanvas:     lucide.PenTool,
	iconShader:     lucide.Sparkles,
	iconStyles:     lucide.Palette,
	iconText:       lucide.Type,
	iconMedia:      lucide.Film,
	iconSystem:     lucide.Cpu,
	iconStorage:    lucide.Database,
	iconStocks:     lucide.ChartCandlestick,
	iconChat:       lucide.MessageCircle,
	iconAccess:     lucide.Accessibility,
	iconLayout:     lucide.Columns2,
	iconMenu:       lucide.Menu,
	iconDock:       lucide.AppWindow,
	iconOverlays:   lucide.Layers,
	iconChevron:    lucide.ChevronRight,
	iconBack:       lucide.ChevronLeft,
	iconCheck:      lucide.Check,
	iconPlus:       lucide.Plus,
	iconMinus:      lucide.Minus,
	iconClose:      lucide.X,
	iconArrowUp:    lucide.ArrowUp,
	iconArrowDown:  lucide.ArrowDown,
	iconPlay:       lucide.Play,
	iconPause:      lucide.Pause,
	iconSearch:     lucide.Search,
	iconStar:       lucide.Star,
	iconBell:       lucide.Bell,
	iconSpark:      lucide.Sparkles,
	iconSidebar:    lucide.PanelLeft,
	iconSun:        lucide.Sun,
	iconMoon:       lucide.Moon,
	iconRefresh:    lucide.RefreshCw,
	iconSend:       lucide.Send,
	iconStop:       lucide.Square,
	iconAttach:     lucide.Paperclip,
	iconCopy:       lucide.Copy,
	iconTrash:      lucide.Trash,
	iconFile:       lucide.File,
	iconTerminal:   lucide.Terminal,
	iconCode:       lucide.Code,
	iconSliders:    lucide.SlidersHorizontal,
	iconReplace:    lucide.Replace,
	iconReplaceAll: lucide.ReplaceAll,
	iconProblems:   lucide.TriangleAlert,
}

// icon draws a glyph size points square in the tint, which it follows.
func icon(name iconName, size core.Points, tint func() core.Color) core.Node {
	var svg *core.SVG
	if name != "" {
		svg = glyphs[name]()
	}
	return core.Icon{SVG: svg, Style: core.Style{Width: core.Pt(size), Height: core.Pt(size)},
		StyleFunc: func() core.Style { return core.Style{Color: tint()} }}
}

// iconOf draws the glyph name returns, which it follows.
func iconOf(name func() iconName, size core.Points, tint func() core.Color) core.Node {
	ic := icon("", size, tint).(core.Icon)
	ic.SVGFunc = func() *core.SVG { return glyphs[name()]() }
	return ic
}

// The chat's glyphs.
const (
	glyphRegenerate iconName = "regenerate"
	glyphThumbUp    iconName = "thumb-up"
	glyphThumbDown  iconName = "thumb-down"
	glyphGlobe      iconName = "globe"
	glyphCaretDown  iconName = "caret-down"
	glyphAlert      iconName = "alert"
	glyphOffline    iconName = "offline"
	glyphBrain      iconName = "brain"
	glyphFileText   iconName = "file-text"
)

func init() {
	glyphs[glyphRegenerate] = lucide.RefreshCw
	glyphs[glyphThumbUp] = lucide.ThumbsUp
	glyphs[glyphThumbDown] = lucide.ThumbsDown
	glyphs[glyphGlobe] = lucide.Globe
	glyphs[glyphCaretDown] = lucide.ChevronDown
	glyphs[glyphAlert] = lucide.CircleAlert
	glyphs[glyphOffline] = lucide.WifiOff
	glyphs[glyphBrain] = lucide.Brain
	glyphs[glyphFileText] = lucide.FileText
}
