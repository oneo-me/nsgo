//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// uiSwitch is core.SystemSwitch's component, which the backend registers
// itself: a UISwitch reporting the user's choice as "change".
type uiSwitch struct {
	cx   *driver.NativeContext
	view objc.ID
}

var (
	uiSwitches   = map[objc.ID]*uiSwitch{}
	switchTarget objc.ID
)

func registerSwitch() {
	class := register("NsgoSwitchTarget", apple.Class("NSObject"), nil, []objc.MethodDef{
		{Cmd: sel("changed:"), Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) {
			if s := uiSwitches[sender]; s != nil {
				s.cx.Emit("change", objc.Send[bool](sender, sel("isOn")))
			}
		}},
	})
	switchTarget = objc.ID(class).Send(selNew)
}

func newSwitch(cx *driver.NativeContext) driver.NativeComponent {
	if switchTarget == 0 {
		registerSwitch()
	}
	s := &uiSwitch{cx: cx, view: objc.ID(apple.Class("UISwitch")).Send(selNew)}
	const valueChanged = 1 << 12
	s.view.Send(sel("addTarget:action:forControlEvents:"), switchTarget, sel("changed:"), uint(valueChanged))
	uiSwitches[s.view] = s
	fit := objc.Send[apple.CGSize](s.view, sel("intrinsicContentSize"))
	if fit.Width <= 0 || fit.Height <= 0 {
		fit = apple.CGSize{Width: 51, Height: 31}
	}
	cx.SetSize(driver.Size{Width: driver.Points(fit.Width), Height: driver.Points(fit.Height)})
	return s
}

func (s *uiSwitch) View() any { return s.view }

func (s *uiSwitch) SetProp(name string, value any) {
	switch name {
	case "checked":
		on, _ := value.(bool)
		if objc.Send[bool](s.view, sel("isOn")) != on {
			s.view.Send(sel("setOn:animated:"), on, objc.Send[objc.ID](s.view, sel("window")) != 0)
		}
	case "disabled":
		off, _ := value.(bool)
		s.view.Send(sel("setEnabled:"), !off)
	}
}

func (s *uiSwitch) Command(string, any) {}

func (s *uiSwitch) Close() {
	delete(uiSwitches, s.view)
	s.view.Send(sel("removeTarget:action:forControlEvents:"), switchTarget, sel("changed:"), uint(1<<12))
	s.view.Send(selRelease)
}
