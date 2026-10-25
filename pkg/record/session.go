// Package record applies entries to a repository with the guard rails; this
// file records a session: the grams drawn from a stash, and what they were
// taken with — the device and its mode, and for a manual one what heated it
// and at which setting.
package record

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
)

// Sesh is a session to record. Device, Mode, Heater and Setting are each
// optional; a mode must be one of the device's when it lists any, the heater
// an induction heater or a torch, and its setting one of the heater's when it
// lists any.
type Sesh struct {
	Product string
	Grams   float64
	At      time.Time
	Device  string
	Temp    int
	Mode    string
	Heater  string
	Setting string
	Timer   int // seconds, for a mode that runs a session for a set time
	Note    string
}

// SessionWith records a session drawing on a product's stash.
func (r *Recorder) SessionWith(s Sesh) (journal.Event, error) {
	if err := notAhead(journal.Event{Type: journal.Sesh, OccurredAt: s.At}, time.Now()); err != nil {
		return journal.Event{}, err
	}
	product, err := r.products.Find(s.Product)
	if err != nil {
		return journal.Event{}, err
	}
	e := journal.Event{Type: journal.Sesh, Product: product.Slug, Grams: s.Grams, OccurredAt: s.At,
		Temperature: s.Temp, Mode: strings.TrimSpace(s.Mode), Setting: strings.TrimSpace(s.Setting), Timer: s.Timer, Note: s.Note}
	if s.Timer < 0 {
		return journal.Event{}, fmt.Errorf("a timer runs for no time or more, not %d s", s.Timer)
	}
	if err := r.takenWith(&e, s); err != nil {
		return journal.Event{}, err
	}
	return r.one(func() (journal.Event, error) {
		if err := r.check(product.Slug, s.Grams, journal.Stash); err != nil {
			return journal.Event{}, err
		}
		return e, nil
	})
}

// takenWith fills in what a session was taken with, refusing what the
// devices say cannot be.
func (r *Recorder) takenWith(e *journal.Event, s Sesh) error {
	if s.Device == "" {
		if s.Heater != "" || e.Mode != "" {
			return fmt.Errorf("a mode and a heater are a device's: name the device")
		}
		return nil
	}
	d, err := r.devices.Find(s.Device)
	if err != nil {
		return err
	}
	if err := runOn(e, d); err != nil {
		return err
	}
	return r.heatedWith(e, s.Heater)
}

// runOn puts the session on its device: a mode it has, and the temperature
// and timer of that mode, or the device's, when none was given.
func runOn(e *journal.Event, d *catalog.Device) error {
	if !d.SessionWith() {
		return fmt.Errorf("%s is a %s, not what a session is had with", d.Name, strings.ReplaceAll(d.Is(), "-", " "))
	}
	e.Device = d.Slug
	if err := oneOf(d, e.Mode, "mode"); err != nil {
		return err
	}
	if m, ok := d.ModeNamed(e.Mode); ok {
		if e.Temperature == 0 {
			e.Temperature = m.Temp
		}
		if e.Timer == 0 {
			e.Timer = m.Timer
		}
	}
	if e.Temperature == 0 {
		e.Temperature = d.DefaultTemp
	}
	if d.MaxTemp > 0 && e.Temperature > d.MaxTemp {
		return fmt.Errorf("%s only goes up to %d°C", d.Name, d.MaxTemp)
	}
	return nil
}

// heatedWith puts what heated the session on it, and its setting.
func (r *Recorder) heatedWith(e *journal.Event, heater string) error {
	if heater == "" {
		if e.Setting != "" {
			return fmt.Errorf("a setting is a heater's: name what heated it")
		}
		return nil
	}
	h, err := r.devices.Find(heater)
	if err != nil {
		return err
	}
	if !h.Heats() {
		return fmt.Errorf("%s does not heat a device; an induction heater or a torch does", h.Name)
	}
	e.Heater = h.Slug
	return oneOf(h, e.Setting, "setting")
}

// oneOf refuses a mode a device does not have, when it lists the ones it has.
func oneOf(d *catalog.Device, mode, what string) error {
	names := d.ModeNames()
	if mode == "" || len(names) == 0 || slices.Contains(names, mode) {
		return nil
	}
	return fmt.Errorf("%s has no %s %q; it has %s", d.Name, what, mode, strings.Join(names, ", "))
}
