package catalog

import (
	"fmt"
	"os"
	"sort"
	"strings"

	can "github.com/TheDonDope/wits/pkg/cannabis"
	"gopkg.in/yaml.v3"
)

// Device is a vaporizer, with the temperature range it can be set to.
type Device struct {
	Slug        string `yaml:"slug"`
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind,omitempty"`
	MinTemp     int    `yaml:"min_temp,omitempty"`
	MaxTemp     int    `yaml:"max_temp,omitempty"`
	DefaultTemp int    `yaml:"default_temp,omitempty"`
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
	return os.WriteFile(path, data, 0600)
}

// Add puts a device in the catalog, refusing to shadow an existing slug.
func (d *Devices) Add(device *Device) error {
	if device.Slug == "" {
		device.Slug = Slugify(device.Name)
	}
	for _, existing := range d.Devices {
		if existing.Slug == device.Slug {
			return fmt.Errorf("%w: %s", ErrDuplicate, device.Slug)
		}
	}
	d.Devices = append(d.Devices, device)
	return nil
}

// Find resolves a reference to a device, by slug, name, or unambiguous part of
// either.
func (d *Devices) Find(ref string) (*Device, error) {
	if ref == "" {
		return nil, ErrNotFound
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
		return nil, fmt.Errorf("%w: %q", ErrNotFound, ref)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, device := range matches {
			names = append(names, device.Slug)
		}
		return nil, fmt.Errorf("%w: %q matches %s", ErrAmbiguous, ref, strings.Join(names, ", "))
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
