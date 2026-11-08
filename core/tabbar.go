package core

import "github.com/oneo-me/nsgo/internal/driver"

// TabBar is the core's part of mobile.TabBar, which apps use: the
// system's tab bar, a native component iOS and Android register. The
// selected tab takes the theme's accent; the others keep the system's.
type TabBar struct {
	Items     []TabBarItem
	ItemsFunc func() []TabBarItem
	// Selected is the selected tab's index; one past the tabs selects
	// none. The bar shows the user's choice at once, as the system's does.
	Selected func() int
	OnChange func(index int)
	Style    Style
	TestID   string
}

// TabBarItem is one tab of a TabBar. Apps name it mobile.TabBarItem.
type TabBarItem struct {
	Title string
	// Icon is the platform's name for an icon: an SF Symbol on iOS
	// ("house"), and on Android a drawable of the app or, prefixed
	// "android:", of the system ("android:ic_menu_search").
	Icon string
}

// Render implements Node.
func (b TabBar) Render(cx *Scope) Node {
	mobile := func() bool { os := cx.Env().OS(); return os == OSIOS || os == OSAndroid }
	return Show{When: mobile, Then: b.native,
		Else: func(*Scope) Node { return View{Style: Style{Display: DisplayNone}, TestID: b.TestID} }}
}

// native is the bar where the platform has one.
func (b TabBar) native(cx *Scope) Node {
	items := func() []TabBarItem {
		if b.ItemsFunc != nil {
			return b.ItemsFunc()
		}
		return b.Items
	}
	return Native{Component: driver.TabBarComponent, Style: b.Style, TestID: b.TestID,
		PropsFunc: func() map[string]any {
			list := []any{}
			for _, it := range items() {
				tab := map[string]any{"title": it.Title}
				if it.Icon != "" {
					tab["icon"] = it.Icon
				}
				list = append(list, tab)
			}
			selected := 0
			if b.Selected != nil {
				selected = b.Selected()
			}
			// One color, for the appearance shown: sent again when it changes.
			return map[string]any{"items": list, "selectedIndex": float64(selected), "tint": float64(cx.Palette().Accent)}
		},
		OnEvent: func(name string, data any) {
			if name != "change" || b.OnChange == nil {
				return
			}
			switch i := data.(type) {
			case float64:
				b.OnChange(int(i))
			case int:
				b.OnChange(i)
			}
		}}
}
