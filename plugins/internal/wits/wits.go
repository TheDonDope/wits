// Package wits is what the plugins in this folder share: calling the wits
// that ran them for a list as JSON, and the shapes of the lists they read.
// A plugin sees only what `wits get <kind> -o json` prints, so it is built
// apart from wits and keeps working while wits's own code changes.
package wits

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Session is one session, as `wits get sessions -o json` prints it.
type Session struct {
	Hash        string         `json:"hash"`
	At          time.Time      `json:"at"`
	Product     string         `json:"product"`
	Grams       float64        `json:"grams"`
	Device      string         `json:"device"`
	Temperature int            `json:"temperature"`
	Scores      map[string]int `json:"scores"`
	Note        string         `json:"note"`
}

// Product is one product, as `wits get products -o json` prints it.
type Product struct {
	Slug     string   `json:"slug"`
	Name     string   `json:"name"`
	THC      float64  `json:"thc"`
	CBD      float64  `json:"cbd"`
	Storage  float64  `json:"storage"`
	Stash    float64  `json:"stash"`
	Terpenes []string `json:"terpenes"`
}

// Get runs `wits get <kind> -o json`, with any more flags, and reads what
// it prints into v. The wits is the one that ran the plugin ($WITS), or the
// first on the PATH when the plugin was run by hand.
func Get(v any, kind string, flags ...string) error {
	bin := cmp.Or(os.Getenv("WITS"), "wits")
	args := append([]string{"get", kind, "-o", "json"}, flags...)
	var out, errs bytes.Buffer
	c := exec.Command(bin, args...)
	c.Stdout, c.Stderr = &out, &errs
	if err := c.Run(); err != nil {
		return fmt.Errorf("wits %s: %s", strings.Join(args, " "), cmp.Or(strings.TrimSpace(errs.String()), err.Error()))
	}
	return json.Unmarshal(out.Bytes(), v)
}

// Fail prints an error the way wits prints its own, and exits.
func Fail(err error) {
	fmt.Fprintf(os.Stderr, "🚨 %v\n", err)
	os.Exit(1)
}
