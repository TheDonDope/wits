package catalog

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/journal"
	"gopkg.in/yaml.v3"
)

// Devices carry their own errors rather than borrowing the product ones. These
// read back to whoever typed the reference, and a mistyped `--device volcanoo`
// answering "no product matches that reference" sends them to the wrong
// catalog looking for a fault that is not there.
var (
	// ErrNoDevice is returned when no device matches a reference.
	ErrNoDevice = errors.New("no device matches that reference")
	// ErrDeviceAmbiguous is returned when a reference matches more than one device.
	ErrDeviceAmbiguous = errors.New("that reference matches more than one device")
	// ErrDeviceDuplicate is returned when adding a device that is already known.
	ErrDeviceDuplicate = errors.New("a device with that slug already exists")
)

// What a device is for: what a session is had with, what heats it, what grinds
// for it. A device written before categories is a vaporizer.
const (
	Vaporizer = "vaporizer"
	Induction = "induction-heater"
	Torch     = "torch"
	Grinder   = "grinder"
	Glass     = "glass"
)

// Categories is every category, in the order a list shows them.
var Categories = []string{Vaporizer, Induction, Torch, Grinder, Glass}

// How a device is powered, how it heats, and how it is used.
const (
	Electric = "electric"
	Manual   = "manual"

	Convection = "convection"
	Conduction = "conduction"
	Hybrid     = "hybrid"

	SessionUse  = "session"
	OnDemandUse = "on-demand"
	BothUses    = "both"
)

// Device is one of the things used to take what was ground: a vaporizer with
// the temperature range it can be set to, what heats a manual one (an
// induction heater, a torch), a grinder, glass. It carries what it is — its
// maker, model and the country the maker is from, how it is powered and how it
// heats, whether it is for a session or on demand — the modes it can be run in,
// and what it cost and where it was bought. Every field after the temperatures
// came later, and is left out when empty, so a catalog written before reads
// and writes as it did.
type Device struct {
	Slug        string `yaml:"slug"`
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind,omitempty"`
	MinTemp     int    `yaml:"min_temp,omitempty"`
	MaxTemp     int    `yaml:"max_temp,omitempty"`
	DefaultTemp int    `yaml:"default_temp,omitempty"`

	Category  string `yaml:"category,omitempty"`
	Maker     string `yaml:"maker,omitempty"`
	Model     string `yaml:"model,omitempty"`
	Country   string `yaml:"country,omitempty"`
	Power     string `yaml:"power,omitempty"`
	Signature string `yaml:"signature,omitempty"`
	Use       string `yaml:"use,omitempty"`
	// Water says glass holds water: a bubbler, a rig with water. Dry otherwise.
	Water bool `yaml:"water,omitempty"`
	// Modes are what the device is set to when used and is asked which of:
	// a vaporizer's boost or its session mode, an induction heater's wattage.
	Modes []Mode `yaml:"modes,omitempty"`
	// AutoOff is how long, in seconds, the device stays on unused before it
	// turns itself off: a session's end, when nothing else ends it.
	AutoOff int `yaml:"auto_off,omitempty"`
	// Load is how many grams one fill of the device holds, as it is usually
	// loaded: what a session on it takes, when no session on record says.
	Load     float64 `yaml:"load,omitempty"`
	Cents    int64   `yaml:"cents,omitempty"`
	Shop     string  `yaml:"shop,omitempty"`
	BoughtOn string  `yaml:"bought_on,omitempty"` // YYYY-MM-DD
	Link     string  `yaml:"link,omitempty"`
	Notes    string  `yaml:"notes,omitempty"`
	// Retired is a device no longer used: kept, since sessions name it.
	Retired bool `yaml:"retired,omitempty"`
}

// Mode is one way a device is run. A mode may heat its own way — a vaporizer
// on demand by convection, in its session mode hybrid, the oven held hot —
// and have a temperature it is usually run at, and a session timer, in
// seconds, for a mode that runs a session for a set time.
type Mode struct {
	Name      string `yaml:"name"`
	Signature string `yaml:"signature,omitempty"`
	Temp      int    `yaml:"temp,omitempty"`
	Timer     int    `yaml:"timer,omitempty"`
}

// ModeNamed is the device's mode of a name, if it has one.
func (d Device) ModeNamed(name string) (Mode, bool) {
	for _, m := range d.Modes {
		if m.Name == name {
			return m, true
		}
	}
	return Mode{}, false
}

// ModeNames is the names of a device's modes.
func (d Device) ModeNames() []string {
	names := make([]string, len(d.Modes))
	for i, m := range d.Modes {
		names[i] = m.Name
	}
	return names
}

// Is says what category a device is of; one written before categories is a vaporizer.
func (d Device) Is() string {
	if d.Category == "" {
		return Vaporizer
	}
	return d.Category
}

// Heats says whether a device is what heats another: an induction heater or a torch.
func (d Device) Heats() bool { return d.Is() == Induction || d.Is() == Torch }

// SessionWith says whether a session is had with the device: a vaporizer, or glass.
func (d Device) SessionWith() bool { return d.Is() == Vaporizer || d.Is() == Glass }

// NeedsHeat says whether a session with the device is heated by another one:
// a manual vaporizer, or glass.
func (d Device) NeedsHeat() bool {
	return (d.Is() == Vaporizer && d.Power == Manual) || d.Is() == Glass
}

// Check refuses a device whose fields say what no device can be.
func (d Device) Check() error {
	in := func(v string, allowed ...string) bool { return v == "" || slices.Contains(allowed, v) }
	switch {
	case strings.TrimSpace(d.Name) == "":
		return errors.New("a device has a name")
	case !in(d.Category, Categories...):
		return fmt.Errorf("%q is no category; one of %s", d.Category, strings.Join(Categories, ", "))
	case !in(d.Power, Electric, Manual):
		return fmt.Errorf("%q is neither electric nor manual", d.Power)
	case !in(d.Signature, Convection, Conduction, Hybrid):
		return fmt.Errorf("%q is no way of heating; convection, conduction or hybrid", d.Signature)
	case slices.ContainsFunc(d.Modes, func(m Mode) bool { return strings.TrimSpace(m.Name) == "" }):
		return errors.New("a mode has a name")
	case slices.ContainsFunc(d.Modes, func(m Mode) bool { return !in(m.Signature, Convection, Conduction, Hybrid) }):
		return errors.New("a mode heats by convection, conduction or both")
	case !in(d.Use, SessionUse, OnDemandUse, BothUses):
		return fmt.Errorf("%q is no way of using it; session, on-demand or both", d.Use)
	case d.Cents < 0:
		return errors.New("a device cost nothing or more")
	case d.AutoOff < 0:
		return errors.New("a device turns itself off after no time or more")
	case d.MaxTemp != 0 && d.MinTemp > d.MaxTemp:
		return errors.New("a device's lowest temperature is below its highest")
	}
	return nil
}

// Update replaces the device of a slug with another, its slug kept.
func (d *Devices) Update(slug string, next *Device) error {
	for i, existing := range d.Devices {
		if existing.Slug == slug {
			next.Slug = slug
			d.Devices[i] = next
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrNoDevice, slug)
}

// String returns the display name of the device.
func (d Device) String() string { return d.Name }

// Devices is the set of known devices.
type Devices struct {
	Devices []*Device `yaml:"devices"`
}

// LoadDevices reads the device catalog from path. A missing file is an empty
// catalog, but an unreadable or malformed one is an error.
func LoadDevices(path string) (*Devices, error) {
	d := &Devices{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return d, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, d); err != nil {
		return nil, fmt.Errorf("reading the device catalog: %w", err)
	}
	return d, nil
}

// Save writes the device catalog to path.
func (d *Devices) Save(path string) error {
	sort.Slice(d.Devices, func(i, j int) bool { return d.Devices[i].Slug < d.Devices[j].Slug })
	data, err := yaml.Marshal(d)
	if err != nil {
		return err
	}
	// Written whole, so a crash must leave the old catalog or the new one.
	return journal.WriteFile(path, data)
}

// Add puts a device in the catalog, refusing to shadow an existing slug.
func (d *Devices) Add(device *Device) error {
	if device.Slug == "" {
		device.Slug = Slugify(device.Name)
	}
	for _, existing := range d.Devices {
		if existing.Slug == device.Slug {
			return fmt.Errorf("%w: %s", ErrDeviceDuplicate, device.Slug)
		}
	}
	d.Devices = append(d.Devices, device)
	return nil
}

// Find resolves a reference to a device, by slug, name, or unambiguous part of
// either.
func (d *Devices) Find(ref string) (*Device, error) {
	if ref == "" {
		return nil, ErrNoDevice
	}
	needle := strings.ToLower(ref)

	for _, device := range d.Devices {
		if device.Slug == ref || strings.EqualFold(device.Name, ref) {
			return device, nil
		}
	}

	var matches []*Device
	for _, device := range d.Devices {
		if strings.Contains(strings.ToLower(device.Slug), needle) || strings.Contains(strings.ToLower(device.Name), needle) {
			matches = append(matches, device)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%w: %q", ErrNoDevice, ref)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, device := range matches {
			names = append(names, device.Slug)
		}
		return nil, fmt.Errorf("%w: %q matches %s", ErrDeviceAmbiguous, ref, strings.Join(names, ", "))
	}
}

// Released is a compound a given temperature is hot enough to volatilise.
type Released struct {
	Name         string
	BoilingPoint int
	Effects      []string
	Harmful      bool
}

// ReleasedAt returns the compounds a temperature in degrees Celsius reaches,
// hottest last. This is what the boiling points in pkg/cannabis are for: they
// turn a number on a dial into what it actually does.
func ReleasedAt(celsius int) []Released {
	var released []Released
	for _, c := range can.Cannabinoids {
		if c.BoilingPoint <= celsius {
			released = append(released, Released{
				Name:         c.ShortName,
				BoilingPoint: c.BoilingPoint,
				Effects:      c.Effects,
				Harmful:      harmful(c.Effects),
			})
		}
	}
	for _, t := range can.Terpenes {
		if t.BoilingPoint <= celsius {
			released = append(released, Released{
				Name:         t.Name,
				BoilingPoint: t.BoilingPoint,
				Effects:      t.Effects,
			})
		}
	}
	sort.Slice(released, func(i, j int) bool {
		if released[i].BoilingPoint != released[j].BoilingPoint {
			return released[i].BoilingPoint < released[j].BoilingPoint
		}
		return released[i].Name < released[j].Name
	})
	return released
}

// Hazards returns the compounds at a temperature that are worth avoiding,
// which above 205 °C means benzene.
func Hazards(celsius int) []Released {
	var hazards []Released
	for _, r := range ReleasedAt(celsius) {
		if r.Harmful {
			hazards = append(hazards, r)
		}
	}
	return hazards
}

// harmful reports whether a compound's effects mark it as one to avoid.
func harmful(effects []string) bool {
	for _, e := range effects {
		if e == "toxic" || e == "carcinogenic" {
			return true
		}
	}
	return false
}
