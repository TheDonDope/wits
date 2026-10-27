// Package api serves a repository over HTTP; this file serves the devices:
// each with what it is, what it cost and what has been done with it, and
// adding, changing or removing one, through the catalog's own locked update.
package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/views"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// deviceFields is a device as the client sends it and reads it back.
type deviceFields struct {
	Name        string `json:"name"`
	Kind        string `json:"kind,omitempty"`
	MinTemp     int    `json:"min_temp,omitempty"`
	MaxTemp     int    `json:"max_temp,omitempty"`
	DefaultTemp int    `json:"default_temp,omitempty"`
	Category    string `json:"category"`
	Maker       string `json:"maker,omitempty"`
	Model       string `json:"model,omitempty"`
	Country     string `json:"country,omitempty"`
	Power       string `json:"power,omitempty"`
	Signature   string `json:"signature,omitempty"`
	Use         string `json:"use,omitempty"`
	Water       bool   `json:"water,omitempty"`
	Modes       []mode `json:"modes"`
	AutoOff     int    `json:"auto_off,omitempty"`
	// Load is the grams one fill holds, as it is usually loaded.
	Load     float64 `json:"load,omitempty"`
	Cents    int64   `json:"cents,omitempty"`
	Shop     string  `json:"shop,omitempty"`
	BoughtOn string  `json:"bought_on,omitempty"`
	Link     string  `json:"link,omitempty"`
	Notes    string  `json:"notes,omitempty"`
	Retired  bool    `json:"retired,omitempty"`
}

// mode is a device's mode as the client reads it: a name, and how it heats.
type mode struct {
	Name      string `json:"name"`
	Signature string `json:"signature,omitempty"`
	Temp      int    `json:"temp,omitempty"`
	Timer     int    `json:"timer,omitempty"`
}

// device is one device, and what has been done with it: the sessions had
// with it and their grams, the sessions it heated, and when it was last used.
type device struct {
	Slug string `json:"slug"`
	deviceFields
	Sessions int       `json:"sessions"`
	Grams    float64   `json:"grams"`
	Heated   int       `json:"heated"`
	LastUsed time.Time `json:"last_used,omitzero"`
}

func fieldsOf(d *catalog.Device) deviceFields {
	modes := make([]mode, len(d.Modes))
	for i, m := range d.Modes {
		modes[i] = mode(m)
	}
	return deviceFields{Name: d.Name, Kind: d.Kind, MinTemp: d.MinTemp, MaxTemp: d.MaxTemp, DefaultTemp: d.DefaultTemp,
		Category: d.Is(), Maker: d.Maker, Model: d.Model, Country: d.Country, Power: d.Power, Signature: d.Signature,
		Use: d.Use, Water: d.Water, Modes: modes, AutoOff: d.AutoOff, Load: d.Load, Cents: d.Cents, Shop: d.Shop, BoughtOn: d.BoughtOn, Link: d.Link,
		Notes: d.Notes, Retired: d.Retired}
}

// toDevice is the catalog's device for what the client sent, trimmed.
func (f deviceFields) toDevice() *catalog.Device {
	var modes []catalog.Mode
	for _, m := range f.Modes {
		if name := strings.TrimSpace(m.Name); name != "" {
			modes = append(modes, catalog.Mode{Name: name, Signature: strings.TrimSpace(m.Signature), Temp: m.Temp, Timer: m.Timer})
		}
	}
	t := strings.TrimSpace
	return &catalog.Device{Name: t(f.Name), Kind: t(f.Kind), MinTemp: f.MinTemp, MaxTemp: f.MaxTemp, DefaultTemp: f.DefaultTemp,
		Category: t(f.Category), Maker: t(f.Maker), Model: t(f.Model), Country: t(f.Country), Power: t(f.Power),
		Signature: t(f.Signature), Use: t(f.Use), Water: f.Water, Modes: modes, AutoOff: f.AutoOff, Load: journal.CG(f.Load).Grams(), Cents: f.Cents, Shop: t(f.Shop),
		BoughtOn: t(f.BoughtOn), Link: t(f.Link), Notes: t(f.Notes), Retired: f.Retired}
}

func devicesOf(ws *workspace.Workspace) any {
	use := views.Devices(ws.State.Standing)
	out := []device{}
	for _, d := range ws.Devices.Devices {
		u := use[d.Slug]
		if u == nil {
			u = &views.Use{}
		}
		out = append(out, device{Slug: d.Slug, deviceFields: fieldsOf(d), Sessions: u.Sessions,
			Grams: u.Grams.Grams(), Heated: u.Heated, LastUsed: u.LastUsed})
	}
	return out
}

// addDevice puts a device in the catalog.
func (s *Server) addDevice(w http.ResponseWriter, r *http.Request) {
	s.changeDevices(w, r, func(c *catalog.Devices, d *catalog.Device) (string, error) {
		if err := c.Add(d); err != nil {
			return "", err
		}
		return fmt.Sprintf("added %s", d.Name), nil
	})
}

// editDevice replaces what a device is, its slug kept: the sessions name it by that.
func (s *Server) editDevice(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	s.changeDevices(w, r, func(c *catalog.Devices, d *catalog.Device) (string, error) {
		if err := c.Update(slug, d); err != nil {
			return "", err
		}
		return fmt.Sprintf("changed %s", d.Name), nil
	})
}

// removeDevice takes a device out of the catalog. Its sessions keep naming
// it, so what is lost is its description, not its history.
func (s *Server) removeDevice(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.writable(w, r)
	if !ok {
		return
	}
	var gone *catalog.Device
	if _, err := ws.Repo.UpdateDevices(func(c *catalog.Devices) error {
		var err error
		gone, err = c.Remove(r.PathValue("slug"))
		return err
	}); err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	used := 0
	if u := views.Devices(ws.State.Standing)[gone.Slug]; u != nil {
		used = u.Sessions
	}
	reply(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("removed %s; %d sessions keep its name", gone.Name, used)})
}

// changeDevices reads a device from the request, checks it, and changes the
// catalog with it as it stands on disk, under its lock.
func (s *Server) changeDevices(w http.ResponseWriter, r *http.Request, change func(*catalog.Devices, *catalog.Device) (string, error)) {
	ws, ok := s.writable(w, r)
	if !ok {
		return
	}
	var f deviceFields
	if err := decode(r, &f); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	d := f.toDevice()
	if err := d.Check(); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	var message string
	if _, err := ws.Repo.UpdateDevices(func(c *catalog.Devices) error {
		var err error
		message, err = change(c, d)
		return err
	}); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	reply(w, http.StatusCreated, map[string]string{"message": message, "slug": d.Slug})
}
