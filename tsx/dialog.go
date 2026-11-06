package tsx

import (
	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Dialog (NSH's navigation.rs): base.Dialog, its handlers given NSH's
// names, and the status bar a page or a dialog asks for.

var reasonNames = map[base.Reason]string{base.ReasonCancel: "cancel", base.ReasonBackdropPress: "backdropPress",
	base.ReasonSwipe: "swipe"}

var orientations = map[string]core.Orientation{"portrait": core.OrientationPortrait,
	"portrait-upside-down": core.OrientationPortraitUpsideDown, "landscape": core.OrientationLandscape,
	"landscape-left": core.OrientationLandscapeLeft, "landscape-right": core.OrientationLandscapeRight}

func dialogFace(p *props) core.Node {
	r := p.r
	d := base.Dialog{
		Animation: map[string]core.ModalAnimation{"slide": core.ModalSlide, "fade": core.ModalFade,
			"none": core.ModalNone}[p.str("animation")],
		NoCloseOnEscape:        !p.boolOr("closeOnEscape", true),
		NoCloseOnBackdropPress: !p.boolOr("closeOnBackdropPress", true),
		NoCloseOnSwipe:         !p.boolOr("closeOnSwipe", true),
		Presentation: map[string]core.ModalPresentation{"fullScreen": core.ModalFullScreen,
			"pageSheet": core.ModalPageSheet, "formSheet": core.ModalFormSheet}[p.str("presentation")],
		StatusBarFunc:  func() core.StatusBarSetting { return r.statusBar(p.get("statusBar")) },
		PopupClass:     p.class(),
		PopupStyleFunc: p.style,
		BackdropClass:  p.str("backdropClass"),
		BackdropTestID: p.str("backdropTestID"),
		Children:       p.children(),
		Label:          p.str("accessibilityLabel"),
		TestID:         p.str("testID"),
		OnOpen:         p.on("onOpen"),
		OnClose:        p.on("onClose"),
		OnCancel:       p.on("onCancel"),
	}
	// NSH's layer lays the popup out as a column's child, the app's style
	// placing it.
	d.LayerStyle = core.Style{JustifyContent: core.JustifyStart, AlignItems: core.AlignStretch}
	if p.has("open") {
		d.Open = func() bool { return p.bool("open") }
	} else {
		// NSH's dialog shows unless open is false.
		d.DefaultOpen = true
	}
	if p.has("backdropStyle") {
		d.BackdropStyleFunc = func() core.Style { return r.style(p.get("backdropStyle")) }
	}
	for _, o := range r.arrayItems(p.get("supportedOrientations")) {
		d.IOS.SupportedOrientations |= orientations[o.String()]
	}
	d.Android.StatusBarTranslucent = p.bool("statusBarTranslucent")
	if h := p.handler("onOpenChange"); h != nil {
		d.OnOpenChange = func(open bool, reason base.Reason) {
			e := r.vm.NewObject()
			_ = e.Set("open", open)
			_ = e.Set("reason", reasonNames[reason])
			h(e)
		}
	}
	return d
}

// statusBar reads { foreground, hidden, animated }.
func (r *runtime) statusBar(v goja.Value) core.StatusBarSetting {
	o := r.options(v)
	var s core.StatusBarSetting
	if o.o == nil {
		return s
	}
	s.Foreground = map[string]core.StatusBarStyle{"auto": core.StatusBarAuto, "light": core.StatusBarLight,
		"dark": core.StatusBarDark}[o.str("foreground", "")]
	toggle := func(k string) core.Toggle {
		switch {
		case !o.has(k):
			return core.ToggleInherit
		case o.bool(k, false):
			return core.ToggleOn
		}
		return core.ToggleOff
	}
	s.Hidden, s.Animated = toggle("hidden"), toggle("animated")
	return s
}

// faceComponent makes a function exporting a face as a component,
// called with the props object compiled JSX passes. Its component is made
// as the node is inserted, so it knows its parents (a Plot's marks), and
// its children as the component places them (props.children).
func (r *runtime) faceComponent(name string, f face) goja.Value {
	return r.function(func(call goja.FunctionCall) goja.Value {
		obj, ok := arg(call, 0).(*goja.Object)
		if !ok {
			obj = r.vm.NewObject()
		}
		n := r.newFaceNode(name, f, obj)
		r.applyRef(obj.Get("ref"), n, "")
		return n.value()
	})
}

// faceExport exports a face as a component.
func faceExport(name string, f face) export {
	return export{name, func(r *runtime) goja.Value { return r.faceComponent(name, f) }}
}
