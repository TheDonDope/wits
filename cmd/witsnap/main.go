// Command witsnap photographs a wits repository: every screen of the
// interface as plain text, or the whole derived state as JSON.
//
// It exists for the work around the work. Pull requests want captures of the
// screens they change; new clients and new visualisations want the fold's
// numbers without scraping a terminal. Both come from the same seams the
// application itself uses, so what witsnap says is what wits would say.
//
//	witsnap screens --repo /tmp/wits-demo
//	witsnap screens --screen storage --press p,tick,tick
//	witsnap json --repo /tmp/wits-demo | jq .balances
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/tui"
	"github.com/TheDonDope/wits/pkg/workspace"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: witsnap <screens|json> [flags]")
	}
	var err error
	switch os.Args[1] {
	case "screens":
		err = screens(os.Args[2:])
	case "json":
		err = dump(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q; witsnap knows screens and json", os.Args[1])
	}
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

// open reads the repository the flags point at, or the working directory.
func open(dir string) (*workspace.Workspace, error) {
	if dir == "" {
		return workspace.Here()
	}
	return workspace.Open(dir)
}

// screens renders one screen or all of them, in the order of the tab bar.
func screens(args []string) error {
	fs := flag.NewFlagSet("screens", flag.ExitOnError)
	repo := fs.String("repo", "", "the repository to photograph; defaults to the working directory")
	name := fs.String("screen", "", "one screen rather than all of them")
	width := fs.Int("width", 100, "the terminal width to render at")
	height := fs.Int("height", 40, "the terminal height to render at")
	press := fs.String("press", "", "keys to press first, comma separated — p,tick,tick replays")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ws, err := open(*repo)
	if err != nil {
		return err
	}
	var presses []string
	if *press != "" {
		presses = strings.Split(*press, ",")
	}
	names := tui.ScreenNames()
	if *name != "" {
		names = []string{*name}
	}
	for _, n := range names {
		shot, err := tui.Snapshot(tui.From(ws), n, *width, *height, presses)
		if err != nil {
			return err
		}
		if len(names) > 1 {
			fmt.Printf("==== %s ====\n", n)
		}
		fmt.Println(strings.TrimRight(shot, "\n"))
	}
	return nil
}

// state is the JSON shape of everything the fold derives: the contract a new
// client can start from without reading a single screen.
type state struct {
	GeneratedAt time.Time                  `json:"generated_at"`
	Events      int                        `json:"events"`
	Balances    map[string]*ledger.Balance `json:"balances"`
	Cycles      []ledger.Cycle             `json:"cycles"`
	Current     *currentCycle              `json:"current_cycle,omitempty"`
	PerDay      map[string]dayTotals       `json:"per_day"`
	Devices     []deviceUse                `json:"device_usage"`
}

type currentCycle struct {
	Start        time.Time `json:"start"`
	Held         float64   `json:"held"`
	Remaining    float64   `json:"remaining"`
	RemainingPct float64   `json:"remaining_pct"`
	PerActiveDay float64   `json:"per_active_day"`
	DaysLeft     float64   `json:"days_left"`
}

type dayTotals struct {
	Ground float64 `json:"ground,omitempty"`
	Seshed float64 `json:"seshed,omitempty"`
}

type deviceUse struct {
	Device   string  `json:"device"`
	Sessions int     `json:"sessions"`
	Grams    float64 `json:"grams"`
	AvgTemp  int     `json:"avg_temp,omitempty"`
}

// dump writes the derived state as JSON.
func dump(args []string) error {
	fs := flag.NewFlagSet("json", flag.ExitOnError)
	repo := fs.String("repo", "", "the repository to read; defaults to the working directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ws, err := open(*repo)
	if err != nil {
		return err
	}

	out := state{
		GeneratedAt: time.Now().Truncate(time.Second),
		Events:      len(ws.State.Events),
		Balances:    ws.State.Balances,
		Cycles:      cyclesOf(ws),
		Current:     current(ws),
		PerDay:      perDay(ws),
		Devices:     deviceUsage(ws),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// cyclesOf strips the event lists out of the cycles: a client that wants the
// events can read the journal, and the summary should stay a summary.
func cyclesOf(ws *workspace.Workspace) []ledger.Cycle {
	out := make([]ledger.Cycle, len(ws.State.Cycles))
	for i, c := range ws.State.Cycles {
		c.Events = nil
		out[i] = c
	}
	return out
}

// current summarises the cycle in progress, if there is one.
func current(ws *workspace.Workspace) *currentCycle {
	c := ws.Cycle()
	if c == nil {
		return nil
	}
	stats := ledger.Summarise(c.Events)
	return &currentCycle{
		Start:        c.Start,
		Held:         c.Held(),
		Remaining:    c.Remaining(),
		RemainingPct: c.RemainingPct(),
		PerActiveDay: stats.PerActiveDay,
		DaysLeft:     stats.DaysLeft(c.Remaining()),
	}
}

// perDay totals the grams ground and seshed per calendar day.
func perDay(ws *workspace.Workspace) map[string]dayTotals {
	out := map[string]dayTotals{}
	for _, e := range ws.State.Events {
		day := e.OccurredAt.Format(time.DateOnly)
		t := out[day]
		switch e.Type {
		case journal.Grind:
			t.Ground = ledger.Round(t.Ground + e.Grams)
		case journal.Sesh:
			t.Seshed = ledger.Round(t.Seshed + e.Grams)
		default:
			continue
		}
		out[day] = t
	}
	return out
}

// deviceUsage totals the sessions per device, the way the sessions screen
// counts them: sessions without a device are owned rather than dropped.
func deviceUsage(ws *workspace.Workspace) []deviceUse {
	byDev := map[string]*deviceUse{}
	var order []string
	temps := map[string][2]int{}
	for _, e := range ws.State.Events {
		if e.Type != journal.Sesh {
			continue
		}
		name := e.Device
		if name == "" {
			name = "no device"
		}
		u, ok := byDev[name]
		if !ok {
			u = &deviceUse{Device: name}
			byDev[name] = u
			order = append(order, name)
		}
		u.Sessions++
		u.Grams = ledger.Round(u.Grams + e.Grams)
		if e.Temperature > 0 {
			t := temps[name]
			temps[name] = [2]int{t[0] + e.Temperature, t[1] + 1}
		}
	}
	out := make([]deviceUse, 0, len(order))
	for _, name := range order {
		u := byDev[name]
		if t := temps[name]; t[1] > 0 {
			u.AvgTemp = t[0] / t[1]
		}
		out = append(out, *u)
	}
	return out
}
