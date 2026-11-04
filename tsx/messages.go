package tsx

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dop251/goja"
)

// registeredMessages is a package of catalogs registered for TSX, for
// its declaration.
type registeredMessages struct {
	t      reflect.Type // its Messages
	params map[string][]string
}

var registeredCatalogs = map[string]registeredMessages{}

var timeType = reflect.TypeFor[time.Time]()

// RegisterMessages makes a package of catalogs' messages importable from
// module in TSX, as typed functions: in is the package's In, and params
// its Parameters, which name each message's parameters in the
// declarations. The module exports messages, whose methods say each
// message in the language the app shows and follow it as it changes,
// messagesIn(locale) for a language of the caller's choice, and
// language(), the language the app shows. Register them before an app
// starts:
//
//	tsx.RegisterMessages("app/locales", locales.In, locales.Parameters)
func RegisterMessages[M any](module string, in func(locale string) M, params map[string][]string) {
	t := reflect.TypeFor[M]()
	registryMu.Lock()
	defer registryMu.Unlock()
	registeredCatalogs[module] = registeredMessages{t, params}
	lang := func(r *runtime) string {
		if l := r.cx.Env().Language(); l != "" {
			return l
		}
		return "en"
	}
	object := func(r *runtime, locale func() string) goja.Value {
		o := r.vm.NewObject()
		for i := range t.NumMethod() {
			m := t.Method(i)
			name := messageName(m.Name)
			_ = o.Set(name, r.function(func(call goja.FunctionCall) goja.Value {
				recv := reflect.ValueOf(in(locale()))
				ft := m.Type
				args := []reflect.Value{recv}
				for j := 1; j < ft.NumIn(); j++ {
					args = append(args, messageArg(call.Argument(j-1), ft.In(j)))
				}
				return r.vm.ToValue(m.Func.Call(args)[0].String())
			}))
		}
		return o
	}
	registered[module] = append(registered[module],
		export{"messages", func(r *runtime) goja.Value { return object(r, func() string { return lang(r) }) }},
		export{"messagesIn", func(r *runtime) goja.Value {
			return r.function(func(call goja.FunctionCall) goja.Value {
				locale := call.Argument(0).String()
				return object(r, func() string { return locale })
			})
		}},
		export{"language", func(r *runtime) goja.Value {
			return r.function(func(goja.FunctionCall) goja.Value { return r.vm.ToValue(lang(r)) })
		}},
	)
}

// messageName is a message's function in TSX, its key: sortBy for SortBy.
func messageName(method string) string {
	r, n := utf8.DecodeRuneInString(method)
	return string(unicode.ToLower(r)) + method[n:]
}

// messageArg reads a JS argument as a message's parameter: a string, a
// number, a Date or any value.
func messageArg(v goja.Value, t reflect.Type) reflect.Value {
	out := reflect.New(t).Elem()
	if !defined(v) {
		return out
	}
	switch {
	case t == timeType:
		if d, ok := v.Export().(time.Time); ok {
			out.Set(reflect.ValueOf(d))
		}
	case t.Kind() == reflect.String:
		out.SetString(v.String())
	case t.Kind() == reflect.Int:
		out.SetInt(v.ToInteger())
	case t.Kind() == reflect.Float64:
		out.SetFloat(v.ToFloat())
	case t.Kind() == reflect.Interface:
		if e := v.Export(); e != nil {
			out.Set(reflect.ValueOf(e))
		}
	}
	return out
}

// messagesDecl declares a package of catalogs' module: its Messages,
// with each message's parameters named and typed.
func messagesDecl(c registeredMessages) string {
	var b strings.Builder
	b.WriteString("\n// Messages, declared from the package of catalogs.\n")
	b.WriteString("/** The messages of a package's catalogs, each in a language. */\nexport interface Messages {\n")
	for i := range c.t.NumMethod() {
		m := c.t.Method(i)
		names := c.params[m.Name]
		var params []string
		for j := 1; j < m.Type.NumIn(); j++ {
			name := fmt.Sprintf("arg%d", j-1)
			if j-1 < len(names) {
				name = names[j-1]
			}
			params = append(params, name+": "+messageTS(m.Type.In(j)))
		}
		fmt.Fprintf(&b, "  %s(%s): string;\n", messageName(m.Name), strings.Join(params, ", "))
	}
	b.WriteString("}\n")
	b.WriteString("/** The messages in the language the app shows, which follow it as it changes. */\nexport declare const messages: Messages;\n")
	b.WriteString("/** The messages in the catalogs' language that suits locale best. */\nexport declare function messagesIn(locale: string): Messages;\n")
	b.WriteString("/** The language the app shows, a BCP 47 tag. */\nexport declare function language(): string;\n")
	return b.String()
}

// messageTS is a message parameter's TypeScript type.
func messageTS(t reflect.Type) string {
	switch {
	case t == timeType:
		return "Date"
	case t.Kind() == reflect.String:
		return "string"
	case t.Kind() == reflect.Int, t.Kind() == reflect.Float64:
		return "number"
	}
	return "unknown"
}

// catalogModules returns the modules of registered catalogs not in
// modules.
func catalogModules(modules []string) []string {
	var out []string
	for m := range registeredCatalogs {
		if !slices.Contains(modules, m) {
			out = append(out, m)
		}
	}
	return out
}
