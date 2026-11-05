//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// tabBar is core.TabBar's component, which the backend registers itself
// as NSH's system_tab_bar.rs: a UITabBar whose items are the app's tabs,
// their icons SF Symbols, reporting the user's choice as "change".
type tabBar struct {
	cx    *driver.NativeContext
	bar   objc.ID
	items []objc.ID
	// selected is the selected tab's index, kept for props in any order.
	selected int
}

var (
	tabBars        = map[objc.ID]*tabBar{}
	tabBarDelegate objc.ID
)

func registerTabBar() {
	class := register("NsgoTabBarDelegate", objc.GetClass("NSObject"),
		[]*objc.Protocol{objc.GetProtocol("UITabBarDelegate")}, []objc.MethodDef{
			{Cmd: sel("tabBar:didSelectItem:"), Fn: func(_ objc.ID, _ objc.SEL, bar, item objc.ID) {
				t := tabBars[bar]
				if t == nil {
					return
				}
				for i, it := range t.items {
					if it == item {
						t.cx.Emit("change", float64(i))
					}
				}
			}},
		})
	tabBarDelegate = objc.ID(class).Send(selNew)
}

func newTabBar(cx *driver.NativeContext) driver.NativeComponent {
	if tabBarDelegate == 0 {
		registerTabBar()
	}
	t := &tabBar{cx: cx, bar: objc.ID(objc.GetClass("UITabBar")).Send(selNew)}
	t.bar.Send(selSetDeleg, tabBarDelegate)
	tabBars[t.bar] = t
	t.measure()
	return t
}

// measure asks for the system's height with the home indicator below it,
// which a bar outside a window does not count.
func (t *tabBar) measure() {
	fit := objc.Send[apple.CGSize](t.bar, sel("sizeThatFits:"), apple.CGSize{Width: 390})
	if fit.Height <= 0 {
		fit.Height = 49
	}
	var bottom float64
	if objc.Send[objc.ID](t.bar, sel("window")) == 0 {
		app := objc.ID(objc.GetClass("UIApplication")).Send(sel("sharedApplication"))
		if window := objc.Send[objc.ID](app, sel("keyWindow")); window != 0 {
			bottom = objc.Send[edgeInsets](window, sel("safeAreaInsets")).Bottom
		}
	}
	t.cx.SetSize(driver.Size{Height: driver.Points(float64(fit.Height) + bottom)})
}

func (t *tabBar) View() any { return t.bar }

// showSelected selects the selected tab's item; one past them selects none.
func (t *tabBar) showSelected() {
	var item objc.ID
	if t.selected >= 0 && t.selected < len(t.items) {
		item = t.items[t.selected]
	}
	t.bar.Send(sel("setSelectedItem:"), item)
}

func (t *tabBar) SetProp(name string, value any) {
	switch name {
	case "items":
		list, _ := value.([]any)
		array := objc.ID(objc.GetClass("NSMutableArray")).Send(selNew)
		t.items = t.items[:0]
		for i, it := range list {
			tab, _ := it.(map[string]any)
			title, _ := tab["title"].(string)
			icon, _ := tab["icon"].(string)
			var image objc.ID
			if icon != "" {
				image = objc.ID(objc.GetClass("UIImage")).Send(sel("systemImageNamed:"), apple.NSString(icon))
			}
			item := objc.ID(objc.GetClass("UITabBarItem")).Send(selAlloc).
				Send(sel("initWithTitle:image:tag:"), apple.NSString(title), image, i)
			t.items = append(t.items, item)
			array.Send(sel("addObject:"), item)
			item.Send(selRelease)
		}
		t.bar.Send(sel("setItems:animated:"), array, false)
		array.Send(selRelease)
		t.showSelected()
	case "selectedIndex":
		i, _ := value.(float64)
		t.selected = int(i)
		t.showSelected()
	}
}

func (t *tabBar) Command(string, any) {}

func (t *tabBar) Close() {
	delete(tabBars, t.bar)
	t.bar.Send(selSetDeleg, objc.ID(0))
	t.bar.Send(selRelease)
}
