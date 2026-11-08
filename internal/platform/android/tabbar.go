//go:build android

package android

import (
	"encoding/json"

	"github.com/oneo-me/nsgo/internal/driver"
)

// tabBar is core.TabBar's component, which the backend registers itself
// as NSH's TabBarComponent.java: the runtime's TabBar, a Material 3
// navigation bar, reporting the user's choice as "change".
type tabBar struct{ cx *driver.NativeContext }

// tabBarHeight is Material 3's navigation bar's height; the runtime's
// view measures itself taller by the system's navigation bar it reaches
// under.
const tabBarHeight driver.Points = 80

func newTabBar(cx *driver.NativeContext) driver.NativeComponent {
	cx.SetSize(driver.Size{Height: tabBarHeight})
	return &tabBar{cx: cx}
}

func (*tabBar) View() any { return "dev.oneo.nsgo.TabBar" }

func (*tabBar) SetProp(string, any) {}

// ViewProps implements driver.AndroidPropMapper: the items cross as one
// JSON string.
func (*tabBar) ViewProps(name string, value any) []driver.AndroidViewProp {
	switch name {
	case "items":
		data, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		return []driver.AndroidViewProp{{Name: "items", Value: string(data)}}
	case "selectedIndex":
		i, _ := value.(float64)
		return []driver.AndroidViewProp{{Name: "selectedIndex", Value: int(i)}}
	case "tint":
		// As ARGB, which an int would overflow on its way through JSON.
		c, _ := value.(float64)
		rgba := uint32(c)
		return []driver.AndroidViewProp{{Name: "tint", Value: int64(rgba>>8 | rgba<<24)}}
	}
	return nil
}

// ViewEvent implements driver.AndroidEventMapper.
func (t *tabBar) ViewEvent(name string, args []any) {
	if name == "onChange" && len(args) == 1 {
		if i, ok := args[0].(float64); ok {
			t.cx.Emit("change", i)
		}
	}
}

func (*tabBar) Command(string, any) {}

func (*tabBar) Close() {}
