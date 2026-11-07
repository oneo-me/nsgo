package tsx

import (
	"reflect"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// The controls of nsgo/base that NSH 0.4.0 added, as Go components, a
// Register per set of events, as each names what its OnChange gives.

var controlEnums = []Option{
	Enum(map[string]base.Axis{"horizontal": base.Horizontal, "vertical": base.Vertical}),
	Enum(map[string]base.LinkOpenWith{"system": base.LinkOpenSystem, "app": base.LinkOpenApp}),
	Enum(map[string]base.SliderScale{"linear": base.SliderLinear, "logarithmic": base.SliderLogarithmic}),
	Enum(map[string]base.AvatarStatus{"idle": base.AvatarIdle, "loading": base.AvatarLoading, "loaded": base.AvatarLoaded,
		"error": base.AvatarError}),
	Enum(map[string]base.PaginationItemKind{"previous": base.PaginationPrevious, "page": base.PaginationPage,
		"ellipsis": base.PaginationEllipsis, "next": base.PaginationNext}),
	Enum(map[string]base.ScrollbarMode{"scrolling": base.ScrollbarScrolling, "hover": base.ScrollbarHover,
		"always": base.ScrollbarAlways}),
	Enum(map[string]base.ScrollbarEntrance{"fade": base.ScrollbarFade, "slideAndFade": base.ScrollbarSlideAndFade}),
	Rename("StartLabel", "startAccessibilityLabel"), Rename("PreviousLabel", "previousAccessibilityLabel"),
	Rename("NextLabel", "nextAccessibilityLabel"), Rename("OverflowLabel", "overflowAccessibilityLabel"),
}

func init() {
	Optional[base.TreeItem]("Children", "Disabled")
	Fields[base.TreeItem](map[string]string{"ID": "id"})
	Fields[base.TreeEntry](map[string]string{"ID": "id"})
	Optional[base.ScrollbarMotion]("Idle", "Enter", "Exit", "Entrance", "ThumbHoverEntrance")
	Optional[base.ScrollbarLook]("Class", "Style", "ThumbClass", "ThumbStyle", "Label")
	Fields[base.ScrollbarLook](map[string]string{"Label": "accessibilityLabel"})
	Optional[base.ScrollbarTheme]("Mode", "Motion", "MinThumbLength", "Vertical", "Horizontal")
	Fields[base.PaginationItem](map[string]string{})
	options := func(extra ...Option) []Option { return append(append([]Option(nil), controlEnums...), extra...) }
	Register("nsgo/base", map[string]core.Node{"Toggle": base.Toggle{}}, options(Event("OnChange", "pressed"))...)
	Register("nsgo/base", map[string]core.Node{
		"ToggleGroup":      base.ToggleGroup{},
		"Toolbar":          base.Toolbar{},
		"ToolbarGroup":     base.ToolbarGroup{},
		"ToolbarSeparator": base.ToolbarSeparator{},
		"Link":             base.Link{},
		"AccordionItem":    base.AccordionItem{},
		"AccordionHeader":  base.AccordionHeader{},
		"AccordionTrigger": base.AccordionTrigger{},
		"AccordionPanel":   base.AccordionPanel{},
	}, options(Event("OnOpen", "href"))...)
	Register("nsgo/base", map[string]core.Node{"Accordion": base.Accordion{}}, options(Event("OnChange", "value"))...)
	Register("nsgo/base", map[string]core.Node{"Avatar": base.Avatar{}}, options(Event("OnStatusChange", "status"),
		custom("SrcFunc", func(p *props, v reflect.Value) {
			if p.has("src") {
				v.Set(reflect.ValueOf(func() core.ImageSource { return p.r.imageSource(p.get("src")) }))
			}
		}, "src?: ImageSource"),
		custom("Decorative", func(p *props, v reflect.Value) {
			v.SetBool(p.has("accessibilityLabel") && p.str("accessibilityLabel") == "")
		}))...)
	Register("nsgo/base", map[string]core.Node{"AvatarGroup": base.AvatarGroup{}}, options(
		custom("Children", func(p *props, v reflect.Value) {
			var nodes []core.Node
			for _, item := range p.r.arrayItems(p.get("children")) {
				nodes = append(nodes, lazy{p.r, p.n, func() goja.Value { return item }})
			}
			v.Set(reflect.ValueOf(nodes))
		}, "children?: JSX.Element | readonly JSX.Element[]"))...)
	Register("nsgo/base", map[string]core.Node{"Pagination": base.Pagination{}}, options(Event("OnChange", "page"))...)
	Register("nsgo/base", map[string]core.Node{"Slider": base.Slider{}}, options(
		custom("Value", func(p *props, v reflect.Value) {
			if p.has("value") {
				v.Set(reflect.ValueOf(func() base.SliderValue { return sliderValue(p.r, p.get("value")) }))
			}
		}, "value?: SliderValue"),
		custom("DefaultValue", func(p *props, v reflect.Value) {
			v.Set(reflect.ValueOf(sliderValue(p.r, p.get("defaultValue"))))
		}, "defaultValue?: SliderValue"),
		custom("OnChange", sliderEvent("onChange"), "onChange?: (e: { value: SliderValue }) => void"),
		custom("OnChangeEnd", sliderEvent("onChangeEnd"), "onChangeEnd?: (e: { value: SliderValue }) => void"))...)
	Register("nsgo/base", map[string]core.Node{"NumberInput": base.NumberInput{}}, options(Event("OnChange", "value"), Event("OnSubmit", "value"))...)
	Register("nsgo/base", map[string]core.Node{"OtpInput": base.OtpInput{}}, options(Event("OnChange", "value"), Event("OnComplete", "value"))...)
	Register("nsgo/base", map[string]core.Node{"Tree": base.Tree{}}, options(Event("OnExpandedChange", "expanded"),
		Event("OnSelectedChange", "selected"), Event("OnExpand", "id"), Event("OnCollapse", "id"), Event("OnAction", "id"))...)
	Register("nsgo/base", map[string]core.Node{
		"Carousel":               base.Carousel{},
		"CarouselContent":        base.CarouselContent{},
		"CarouselItem":           base.CarouselItem{},
		"CarouselPrevious":       base.CarouselPrevious{},
		"CarouselNext":           base.CarouselNext{},
		"CarouselPagination":     base.CarouselPagination{},
		"CarouselPaginationItem": base.CarouselPaginationItem{},
	}, options(Event("OnChange", "index"))...)
	Register("nsgo/base", map[string]core.Node{"Scrollbar": base.Scrollbar{}}, options(
		Flag("Horizontal", "axis", "horizontal", `"horizontal" | "vertical"`),
		custom("Scroll", func(p *props, v reflect.Value) {
			v.Set(reflect.ValueOf(func() *core.Ref {
				if n := p.r.nodeOf(p.get("scroll")); n != nil {
					return n.ref
				}
				return nil
			}))
		}, "scroll?: ViewRef | null"))...)
	RegisterStatics("nsgo/base", "Scrollbar", map[string]any{"setTheme": base.SetScrollbarTheme})
	Shape("SliderValue", `export type SliderValue = number | [number, number];`, func(base.SliderValue) map[string]any { return nil }, nil)
}

// sliderValue reads a number or [start, end].
func sliderValue(r *runtime, v goja.Value) base.SliderValue {
	if items := r.arrayItems(v); len(items) == 2 {
		return base.RangeValue(items[0].ToFloat(), items[1].ToFloat())
	}
	if !defined(v) {
		return base.SingleValue(0)
	}
	return base.SingleValue(v.ToFloat())
}

// sliderEvent gives prop's handler { value }, a number or [start, end].
func sliderEvent(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		h := p.handler(prop)
		if h == nil {
			return
		}
		r := p.r
		v.Set(reflect.ValueOf(func(value base.SliderValue) {
			o := r.vm.NewObject()
			if value.Range {
				_ = o.Set("value", r.vm.NewArray(value.Start, value.End))
			} else {
				_ = o.Set("value", value.End)
			}
			h(o)
		}))
	}
}

// scrollbarTheme is the ScrollbarTheme a view's scrollbar prop gives, nil
// for true, false and none.
func (r *runtime) scrollbarTheme(v goja.Value) core.ScrollbarMaker {
	o, ok := v.(*goja.Object)
	if !ok || !defined(v) {
		return nil
	}
	theme := new(base.ScrollbarTheme)
	reflect.ValueOf(theme).Elem().Set(r.decode(o, reflect.TypeFor[base.ScrollbarTheme]()))
	return theme
}
