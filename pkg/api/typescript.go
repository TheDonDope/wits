// Package api serves a repository over HTTP; this file writes the shapes the
// API answers in as TypeScript, so the browser's models are checked against
// what the server sends rather than kept alike by hand. `make types` writes
// wits-ui/src/app/data/api.gen.ts, and a test fails when it is out of date.
package api

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode"
)

// shapes is every answer the browser's models are checked against, by the
// name its TypeScript interface takes (Api and the name).
var shapes = map[string]any{
	"Entry":         entry{},
	"Jar":           jar{},
	"Device":        device{},
	"DeviceFields":  deviceFields{},
	"Written":       written{},
	"Preview":       preview{},
	"Accounts":      accounts{},
	"ForgottenPlan": forgottenPlan{},
	"PricedPreview": pricedPreview{},
	"JournalSince":  journalSince{},
}

// TypeScript is the shapes as TypeScript interfaces, each named Api and its
// name, a nested struct by its own: what api.gen.ts holds.
func TypeScript() string {
	g := &tsWriter{named: map[reflect.Type]string{}}
	for name, v := range shapes {
		g.named[reflect.TypeOf(v)] = name
	}
	names := make([]string, 0, len(shapes))
	for name := range shapes {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		g.queue = append(g.queue, reflect.TypeOf(shapes[name]))
	}
	var b strings.Builder
	b.WriteString("// What the API answers in, written by `make types` from pkg/api (typescript.go).\n")
	b.WriteString("// Do not edit: data/api-check.ts holds the models to these at compile time.\n")
	for len(g.queue) > 0 {
		t := g.queue[0]
		g.queue = g.queue[1:]
		if g.done[t] {
			continue
		}
		g.mark(t)
		fmt.Fprintf(&b, "\nexport interface Api%s {\n", g.nameOf(t))
		g.fields(&b, t)
		b.WriteString("}\n")
	}
	return b.String()
}

type tsWriter struct {
	named map[reflect.Type]string
	done  map[reflect.Type]bool
	queue []reflect.Type
}

func (g *tsWriter) mark(t reflect.Type) {
	if g.done == nil {
		g.done = map[reflect.Type]bool{}
	}
	g.done[t] = true
}

// nameOf is a struct's interface name: its shape's, or its Go name with a capital.
func (g *tsWriter) nameOf(t reflect.Type) string {
	if n, ok := g.named[t]; ok {
		return n
	}
	r := []rune(t.Name())
	if len(r) == 0 {
		return "Anonymous"
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// fields writes a struct's fields, an embedded one's in its place.
func (g *tsWriter) fields(b *strings.Builder, t reflect.Type) {
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			g.fields(b, f.Type)
			continue
		}
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		optional := strings.Contains(opts, "omit") || f.Type.Kind() == reflect.Pointer
		q := ""
		if optional {
			q = "?"
		}
		fmt.Fprintf(b, "  %s%s: %s;\n", name, q, g.typeOf(f.Type))
	}
}

var timeType = reflect.TypeFor[time.Time]()

func (g *tsWriter) typeOf(t reflect.Type) string {
	if t == timeType {
		return "string"
	}
	switch t.Kind() {
	case reflect.Pointer:
		return g.typeOf(t.Elem())
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return g.typeOf(t.Elem()) + "[]"
	case reflect.Map:
		return "Record<string, " + g.typeOf(t.Elem()) + ">"
	case reflect.Struct:
		g.queue = append(g.queue, t)
		return "Api" + g.nameOf(t)
	default:
		return "unknown"
	}
}
