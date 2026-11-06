package core

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Tailwind is Tailwind CSS 4's default theme extended for the classes of
// a theme's subtree (Theme.Tailwind), as NSH's nsh.json tailwind extends
// an app's.
type Tailwind struct{ t *tailwindTheme }

// ParseTailwind reads an extension of Tailwind's default theme, an
// entry with an existing name replacing the default's:
//
//	colors: { brand: { 500: "#7c3aed", DEFAULT: "#6d28d9" }, accent: "#f59e0b" }
//	spacing: 4
//	breakpoints: { tablet: 600 }
//	fontFamily: { display: Inter }
//	fontSize: { huge: [48, 1.1] }
//	fontWeight: { heavy: 850 }
//	radius: { card: 14 }
//	shadow: { card: "0 2px 8px rgb(0 0 0 / 0.12)" }
//	tracking: { label: 0.08 }
//	leading: { dense: 1.15 }
//	blur: { glass: 20 }
//	aspect: { photo: 1.5 }
//	ease: { snappy: [0.2, 0, 0, 1] }
//	container: { content: 680 }
//
// insetShadow, dropShadow, perspective and rem take their scales too.
// Lengths are points; an unknown key is an error. JSON is YAML, so
// NSH's tailwind object reads as it is.
func ParseTailwind(data []byte) (*Tailwind, error) {
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("core: tailwind: %w", err)
	}
	t := defaultTailwind.clone()
	if err := t.extend(root); err != nil {
		return nil, fmt.Errorf("core: tailwind: %w", err)
	}
	return &Tailwind{t}, nil
}

// tailwind is the Tailwind theme t's classes resolve against.
func (t *Theme) tailwind() *tailwindTheme {
	if t == nil || t.Tailwind == nil {
		return defaultTailwind
	}
	return t.Tailwind.t
}

func (t *tailwindTheme) clone() *tailwindTheme {
	c := *t
	c.colors, c.containers, c.text, c.weights = maps.Clone(t.colors), maps.Clone(t.containers), maps.Clone(t.text), maps.Clone(t.weights)
	c.fonts, c.tracking, c.leading, c.radius = maps.Clone(t.fonts), maps.Clone(t.tracking), maps.Clone(t.leading), maps.Clone(t.radius)
	c.shadows, c.insetShadow, c.dropShadows = maps.Clone(t.shadows), maps.Clone(t.insetShadow), maps.Clone(t.dropShadows)
	c.textShadows, c.blur, c.perspective = maps.Clone(t.textShadows), maps.Clone(t.blur), maps.Clone(t.perspective)
	c.aspect, c.ease = maps.Clone(t.aspect), maps.Clone(t.ease)
	c.breakpoints = slices.Clone(t.breakpoints)
	return &c
}

// extend applies NSH's Theme::extend.
func (t *tailwindTheme) extend(root map[string]any) error {
	for key, v := range root {
		var err error
		switch key {
		case "spacing":
			t.spacing, err = twNumber[Points](v, "spacing")
		case "rem":
			t.rem, err = twNumber[Points](v, "rem")
		case "colors":
			err = twEntries(v, key, func(name string, v any) error {
				_, ok := v.(map[string]any)
				if _, isAny := v.(map[any]any); !ok && !isAny {
					c, err := twColor(v, name)
					t.colors[name] = c
					return err
				}
				return twEntries(v, key, func(shade string, v any) error {
					name := name + "-" + shade
					if shade == "DEFAULT" {
						name = strings.TrimSuffix(name, "-"+shade)
					}
					c, err := twColor(v, name)
					t.colors[name] = c
					return err
				})
			})
		case "breakpoints":
			err = twEntries(v, key, func(name string, v any) error {
				w, err := twNumber[Points](v, "breakpoint "+name)
				t.breakpoints = slices.DeleteFunc(t.breakpoints, func(b breakpoint) bool { return b.name == name })
				t.breakpoints = append(t.breakpoints, breakpoint{name, w})
				return err
			})
			slices.SortStableFunc(t.breakpoints, func(a, b breakpoint) int { return int(a.width - b.width) })
		case "container":
			err = twScale(v, key, t.containers)
		case "fontSize":
			err = twEntries(v, key, func(name string, v any) error {
				what := "font size " + name
				if pair, ok := v.([]any); ok && len(pair) == 2 {
					size, err := twNumber[Points](pair[0], what)
					if err != nil {
						return err
					}
					leading, err := twNumber[float32](pair[1], what)
					t.text[name] = textSize{size, leading}
					return err
				}
				size, err := twNumber[Points](v, what)
				t.text[name] = textSize{size: size}
				return err
			})
		case "fontWeight":
			err = twEntries(v, key, func(name string, v any) error {
				w, err := twNumber[float32](v, "font weight "+name)
				t.weights[name] = FontWeight(min(max(w, 1), 1000))
				return err
			})
		case "fontFamily":
			err = twEntries(v, key, func(name string, v any) error {
				f, ok := v.(string)
				if !ok {
					return fmt.Errorf("font family %s must be a string", name)
				}
				t.fonts[name] = f
				return nil
			})
		case "tracking":
			err = twScale(v, key, t.tracking)
		case "leading":
			err = twScale(v, key, t.leading)
		case "radius":
			err = twScale(v, key, t.radius)
		case "shadow", "insetShadow", "dropShadow":
			target := map[string]map[string][]BoxShadow{"shadow": t.shadows, "insetShadow": t.insetShadow,
				"dropShadow": t.dropShadows}[key]
			err = twEntries(v, key, func(name string, v any) error {
				s, _ := v.(string)
				list, err := ParseBoxShadows(s)
				if err != nil || s == "" {
					return fmt.Errorf("%s %s must be a CSS box-shadow", key, name)
				}
				if key == "insetShadow" {
					for i := range list {
						list[i].Inset = true
					}
				}
				target[name] = list
				return nil
			})
		case "blur":
			err = twScale(v, key, t.blur)
		case "perspective":
			err = twScale(v, key, t.perspective)
		case "aspect":
			err = twScale(v, key, t.aspect)
		case "ease":
			err = twEntries(v, key, func(name string, v any) error {
				what := fmt.Errorf("ease %s must be [x1, y1, x2, y2]", name)
				list, ok := v.([]any)
				if !ok || len(list) != 4 {
					return what
				}
				var n [4]float32
				for i, x := range list {
					f, err := twNumber[float32](x, "")
					if err != nil {
						return what
					}
					n[i] = f
				}
				t.ease[name] = Curve{X1: n[0], Y1: n[1], X2: n[2], Y2: n[3]}
				return nil
			})
		default:
			err = fmt.Errorf("unknown theme key %q", key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func twEntries(v any, what string, f func(name string, v any) error) error {
	m, ok := v.(map[string]any)
	if km, isAny := v.(map[any]any); isAny {
		// YAML reads keys such as 500 as numbers.
		m, ok = map[string]any{}, true
		for k, v := range km {
			m[fmt.Sprint(k)] = v
		}
	}
	if !ok {
		return fmt.Errorf("%s must be a map", what)
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if err := f(k, m[k]); err != nil {
			return err
		}
	}
	return nil
}

func twScale[T ~float32](v any, what string, scale map[string]T) error {
	return twEntries(v, what, func(name string, v any) error {
		n, err := twNumber[T](v, what+" "+name)
		scale[name] = n
		return err
	})
}

func twNumber[T ~float32](v any, what string) (T, error) {
	switch n := v.(type) {
	case int:
		return T(n), nil
	case float64:
		return T(n), nil
	}
	return 0, fmt.Errorf("%s must be a number", what)
}

func twColor(v any, name string) (Color, error) {
	if s, ok := v.(string); ok {
		if c, err := ParseColor(s); err == nil {
			return c, nil
		}
	}
	return 0, fmt.Errorf("color %s must be a CSS color", name)
}
