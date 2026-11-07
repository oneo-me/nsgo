package driver

import "testing"

func TestRim(t *testing.T) {
	base := ViewStyle{CornerRadius: 12, BorderWidth: 4, BorderColor: 0x00000080, Background: 0xffffffff}
	rim, inset, ok := base.Rim()
	if !ok || rim != 0x7f7f7fff || inset != 3 {
		t.Errorf("Rim() = %#x, %v, %v", uint32(rim), inset, ok)
	}
	for name, change := range map[string]func(*ViewStyle){
		"square":      func(vs *ViewStyle) { vs.CornerRadius = 0 },
		"no border":   func(vs *ViewStyle) { vs.BorderWidth = 0 },
		"clear":       func(vs *ViewStyle) { vs.BorderColor = 0 },
		"see-through": func(vs *ViewStyle) { vs.Background = 0xffffffeb },
		"dashed":      func(vs *ViewStyle) { vs.BorderStyle = BorderDashed },
		"sides":       func(vs *ViewStyle) { vs.Borders = Edges{Top: 1} },
		"gradient":    func(vs *ViewStyle) { vs.Effects = &Effects{Gradients: []GradientPaint{{}}} },
	} {
		vs := base
		change(&vs)
		if _, _, ok := vs.Rim(); ok {
			t.Errorf("%s: has a rim", name)
		}
	}
}
