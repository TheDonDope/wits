// Package catalog holds the products and devices; this file keeps when each
// of their fields was last changed, so two devices' catalogs merge field by
// field, the newer change winning, rather than one copy taken over the other
// whole — which lost a terpene profile written on one device to the next pull
// of the other's older copy.
package catalog

import (
	"maps"
	"reflect"
	"strings"
	"time"
)

// Changed is when each field of a product or a device was last changed, by
// the field's name in the catalog file. A field never changed since stamps
// began has no stamp, which is older than any.
type Changed map[string]time.Time

// unstamped are the fields that are not merged by their stamps: what a
// product or device is (its slug), the stamps themselves, when it was added,
// and a strain's rating, which carries its own moment.
var unstamped = map[string]bool{"slug": true, "changed": true, "added_at": true, "rating": true}

// stamped is a struct's merged fields by their names in the catalog file.
func stamped(v reflect.Value) map[string]reflect.Value {
	out := map[string]reflect.Value{}
	t := v.Type()
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		if name != "" && name != "-" && !unstamped[name] {
			out[name] = v.Field(i)
		}
	}
	return out
}

// changedOf is the stamps of a product or a device, made if it has none.
func changedOf(v reflect.Value) Changed {
	f := v.FieldByName("Changed")
	if f.IsNil() {
		f.Set(reflect.ValueOf(Changed{}))
	}
	return f.Interface().(Changed)
}

// stamp marks with now every field of after that differs from before and
// whose stamp the change left alone: a field set by a merge carries the
// stamp it came with, and keeps it. A thing with no before is new, and every
// field it has that carries no stamp yet is stamped.
func stamp(before, after reflect.Value, now time.Time) {
	stamps := changedOf(after)
	var was Changed
	var old map[string]reflect.Value
	if before.IsValid() {
		was, _ = before.FieldByName("Changed").Interface().(Changed)
		old = stamped(before)
	}
	for name, v := range stamped(after) {
		if old == nil {
			// New here: stamped now, unless it came with its stamps, as a
			// product a pull brings in does.
			if !v.IsZero() && stamps[name].IsZero() {
				stamps[name] = now
			}
			continue
		}
		if !reflect.DeepEqual(old[name].Interface(), v.Interface()) && stamps[name].Equal(was[name]) {
			stamps[name] = now
		}
	}
	if len(stamps) == 0 {
		after.FieldByName("Changed").Set(reflect.Zero(reflect.TypeFor[Changed]()))
	}
}

// merge is ours with every field theirs changed later taken from theirs,
// with its stamp; and whether anything was taken. Neither stamped is the
// older catalogs' case, and theirs is taken then, as every pull did before
// fields had stamps.
func merge(ours, theirs reflect.Value) (reflect.Value, bool) {
	out := reflect.New(ours.Type()).Elem()
	out.Set(ours)
	mine, _ := ours.FieldByName("Changed").Interface().(Changed)
	other, _ := theirs.FieldByName("Changed").Interface().(Changed)
	stamps := maps.Clone(mine)
	if stamps == nil {
		stamps = Changed{}
	}
	took := false
	fields := stamped(out)
	for name, v := range stamped(theirs) {
		if reflect.DeepEqual(fields[name].Interface(), v.Interface()) {
			continue
		}
		at, was := other[name], mine[name]
		if at.After(was) || (at.IsZero() && was.IsZero()) {
			fields[name].Set(v)
			if !at.IsZero() {
				stamps[name] = at
			}
			took = true
		}
	}
	if len(stamps) > 0 {
		out.FieldByName("Changed").Set(reflect.ValueOf(stamps))
	}
	return out, took
}

// StampProducts marks what a change did to the catalog: every field of a
// product that differs from before, at now.
func StampProducts(before, after *Catalog, now time.Time) {
	was := map[string]*Product{}
	for _, p := range before.Products {
		was[p.Slug] = p
	}
	for _, p := range after.Products {
		var b reflect.Value
		if old := was[p.Slug]; old != nil {
			b = reflect.ValueOf(old).Elem()
		}
		stamp(b, reflect.ValueOf(p).Elem(), now.Truncate(time.Second))
	}
}

// StampDevices is StampProducts for the devices.
func StampDevices(before, after *Devices, now time.Time) {
	was := map[string]*Device{}
	for _, d := range before.Devices {
		was[d.Slug] = d
	}
	for _, d := range after.Devices {
		var b reflect.Value
		if old := was[d.Slug]; old != nil {
			b = reflect.ValueOf(old).Elem()
		}
		stamp(b, reflect.ValueOf(d).Elem(), now.Truncate(time.Second))
	}
}

// MergeProduct is ours with every field theirs changed later, and whether
// anything was taken. The rating is the caller's: it carries its own moment.
func MergeProduct(ours, theirs *Product) (*Product, bool) {
	v, took := merge(reflect.ValueOf(ours).Elem(), reflect.ValueOf(theirs).Elem())
	p := v.Interface().(Product)
	return &p, took
}

// MergeDevice is MergeProduct for a device.
func MergeDevice(ours, theirs *Device) (*Device, bool) {
	v, took := merge(reflect.ValueOf(ours).Elem(), reflect.ValueOf(theirs).Elem())
	d := v.Interface().(Device)
	return &d, took
}
