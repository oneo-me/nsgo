package main

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	tsxui "github.com/oneo-me/nsgo/tsx/ui"
)

// The Lucide icons the TSX names.
func init() {
	tsxui.Icons(map[string]func() *core.SVG{
		"app-window":          lucide.AppWindow,
		"bell":                lucide.Bell,
		"bold":                lucide.Bold,
		"calendar":            lucide.Calendar,
		"chart-spline":        lucide.ChartSpline,
		"chevron-right":       lucide.ChevronRight,
		"chevrons-up-down":    lucide.ChevronsUpDown,
		"code":                lucide.Code,
		"credit-card":         lucide.CreditCard,
		"eye":                 lucide.Eye,
		"file":                lucide.File,
		"file-diff":           lucide.FileDiff,
		"file-text":           lucide.FileText,
		"folder":              lucide.Folder,
		"gallery-horizontal":  lucide.GalleryHorizontal,
		"git-graph":           lucide.GitGraph,
		"image":               lucide.Image,
		"inbox":               lucide.Inbox,
		"italic":              lucide.Italic,
		"link":                lucide.Link,
		"list":                lucide.List,
		"loader-circle":       lucide.LoaderCircle,
		"menu":                lucide.Menu,
		"message-square":      lucide.MessageSquare,
		"moon":                lucide.Moon,
		"mouse-pointer-click": lucide.MousePointerClick,
		"notebook-tabs":       lucide.NotebookTabs,
		"palette":             lucide.Palette,
		"panel-left":          lucide.PanelLeft,
		"panel-top":           lucide.PanelTop,
		"panels-top-left":     lucide.PanelsTopLeft,
		"pin":                 lucide.Pin,
		"plus":                lucide.Plus,
		"redo-2":              lucide.Redo2,
		"rows-3":              lucide.Rows3,
		"search":              lucide.Search,
		"send":                lucide.Send,
		"share":               lucide.Share,
		"sliders-horizontal":  lucide.SlidersHorizontal,
		"square-check":        lucide.SquareCheck,
		"star":                lucide.Star,
		"sun":                 lucide.Sun,
		"table":               lucide.Table,
		"tag":                 lucide.Tag,
		"terminal":            lucide.Terminal,
		"text-cursor-input":   lucide.TextCursorInput,
		"trash":               lucide.Trash,
		"underline":           lucide.Underline,
		"undo-2":              lucide.Undo2,
		"user":                lucide.User,
		"users":               lucide.Users,
	})
}
