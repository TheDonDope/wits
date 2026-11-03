package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// fieldsOf lists the JSON names of a type's fields, those of an embedded
// struct as its own and those of a nested one under its field's name.
func fieldsOf(t reflect.Type, prefix string) []string {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || t == reflect.TypeFor[time.Time]() {
		return nil
	}
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		switch {
		case f.Anonymous && name == "":
			out = append(out, fieldsOf(f.Type, prefix)...)
		case name == "-" || !f.IsExported():
		default:
			out = append(out, prefix+name)
			out = append(out, fieldsOf(f.Type, prefix+name+".")...)
		}
	}
	return out
}

// TestShapes pins what -o json prints, kind by kind: an entry's too, though
// pkg/journal holds its shape, since a script reads it. Adding a field is
// adding it here. Renaming, retyping or taking one away breaks every script
// that reads it, and is wits/v2: change APIVersion, and say so in the
// release.
func TestShapes(t *testing.T) {
	assert.Equal(t, "wits/v1", APIVersion)
	for kind, want := range map[string]struct {
		shape  any
		fields string
	}{
		"Product": {productRow{}, "slug name maker cultivar thc cbd storage stash avb consumed terpenes"},
		"Product (described)": {productDetail{}, "slug name maker cultivar thc cbd storage stash avb consumed terpenes " +
			"taste overall ratings rating rating.overall rating.taste rating.aroma rating.again rating.note rating.at " +
			"like like.slug like.name like.alike like.shared like.overall like.ratings"},
		"Device": {deviceRow{}, "slug name type maker heat min_temp max_temp usual_temp load retired sessions heated grams last_used"},
		"Device (described)": {deviceDetail{}, "slug name type maker heat min_temp max_temp usual_temp load retired sessions heated grams last_used " +
			"released released.name released.boils_at released.effects released.harmful"},
		"Fill":             {fillRow{}, "fill opened closed dispensed left products"},
		"Fill (described)": {fillDetail{}, "fill opened closed dispensed left products carried jars jars.product jars.name jars.left"},
		"Temperature":      {temperatureDetail{}, "celsius released released.name released.boils_at released.effects released.harmful"},
		"Session":          {sessionRow{}, "hash at product grams device temperature rated scores note"},
		"Plugin":           {pluginRow{}, "name path hidden"},
		"Entry":            {journal.Event{}, entryFields},
	} {
		got := fieldsOf(reflect.TypeOf(want.shape), "")
		assert.Equal(t, strings.Fields(want.fields), got, "%s: a field renamed or taken away is wits/v2", kind)
		assert.NotContains(t, got, "kind", "%s: kind names the shape", kind)
		assert.NotContains(t, got, "apiVersion", kind)
	}
}

// entryFields are a journal entry's, as the journal holds them and -o json
// prints them.
const entryFields = "seq type occurred_at recorded_at product grams from to device temperature mode heater setting timer note reverts merge prev hash"

// TestDescribeJSON: each kind described as one object, its version and kind
// first.
func TestDescribeJSON(t *testing.T) {
	dir := stockedRepo(t)
	for _, c := range []struct {
		args []string
		kind string
		has  string
	}{
		{[]string{"describe", "product", "wedding", "-o", "json"}, "Product", `"storage": 18`},
		{[]string{"describe", "device", "mighty", "-o", "json"}, "Device", `"type": "vaporizer"`},
		{[]string{"describe", "fill", "-o", "json"}, "Fill", `"product": "wcake-221"`},
		{[]string{"describe", "temperature", "230", "-o", "json"}, "Temperature", `"harmful": true`},
	} {
		out, err := run(t, dir, c.args...)
		require.NoError(t, err, c.args)
		assert.True(t, strings.HasPrefix(out, "{\n  \"apiVersion\": \"wits/v1\",\n  \"kind\": \""+c.kind+"\","), out)
		assert.Contains(t, out, c.has, c.args)
		var v map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &v), "one object")
	}
	_, err := run(t, dir, "describe", "fill", "-o", "yaml")
	assert.ErrorContains(t, err, "text or json")
}

// syncBuffer is a buffer two goroutines can share.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// TestWatch: a watched list is printed once, then again when an entry is
// written, a line of JSON each time, until it is stopped.
func TestWatch(t *testing.T) {
	dir := stockedRepo(t)
	t.Chdir(dir)
	defer func(every time.Duration) { watchEvery = every }(watchEvery)
	watchEvery = 10 * time.Millisecond

	var out syncBuffer
	Root.SetOut(&out)
	Root.SetErr(&out)
	Root.SetArgs([]string{"get", "products", "-l", "slug=wcake-221", "-o", "json", "--watch"})
	defer func() { Root.SetArgs(nil); resetFlags(Root) }()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- Root.ExecuteContext(ctx) }()

	lines := func() []string { return strings.Split(strings.TrimSpace(out.String()), "\n") }
	require.Eventually(t, func() bool { return out.String() != "" }, 5*time.Second, 5*time.Millisecond)
	assert.Len(t, lines(), 1, "a list a line")
	assert.Contains(t, lines()[0], `"stash":1.7`)

	ws, err := workspace.Open(dir)
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("wcake-221", 1, time.Now())
	require.NoError(t, err)
	require.Eventually(t, func() bool { return len(lines()) == 2 }, 5*time.Second, 5*time.Millisecond)
	assert.Contains(t, lines()[1], `"stash":2.7`, "printed again with the grind")

	cancel()
	require.NoError(t, <-done, "stopping is not an error")
}
