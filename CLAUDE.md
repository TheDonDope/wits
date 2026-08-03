# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
make build         # build ./bin/wits
make snap          # build ./bin/witsnap — rebuild BOTH after TUI changes
make install-wits  # install the working tree into $GOBIN with the version stamp
make test          # go test -race ./... with coverage
make vet
make preflight     # everything CI and the Codacy gate will say, said here first
```

Single test: `go test ./pkg/ledger -run TestCycleGap -v`. TUI tests live in
`pkg/tui` and render screens to plain text — no terminal needed.

**Run `./preflight.sh` before every push.** It mirrors the Codacy gate; a red X
on the PR should never be news. `./preflight.sh watch <pr>` polls a PR's checks
and prints the exact issues Codacy will refuse. Codacy enforces a **50-line
method limit** — when a function grows past it, extract a named helper (see
`folder` in pkg/ledger, `grindTotals` in pkg/tui for the house pattern). The
local complexity check needs `pip install lizard`, otherwise preflight skips it
silently.

Releases: `make release VERSION=vX.Y.0` (regenerates CHANGELOG via git-chglog,
commits, tags, pushes main + tags — run from main after merging), then
`gh release create vX.Y.0 --title "vX.Y.0 — <short poetic title>" --notes ...`
in the style of the existing releases. Every merged PR so far has shipped as
its own minor release.

## Verifying TUI changes

`witsnap` photographs screens without a terminal:

```sh
./bin/witsnap screens --screen storage --press c,enter   # any screen, scripted keys
./bin/witsnap json                                       # the whole derived state
```

Seed a throwaway repo first (`./demo-seed.sh /tmp/demo && cd /tmp/demo`), or
import the real four-year workbook for realistic data:
`wits init . && wits import <repo>/assets/Tracking.2022.cleaned.xlsx --commit`.

The GIFs in `assets/` are recorded from the `*.tape` files and are kept current
with the screens they show — re-record when a change is user-visible
(`vhs wits-tour.tape`; needs vhs, gum, ttyd and the Hack font). **Render tapes
one at a time and run nothing heavy alongside** — a render spawns a Chromium
and the machine this repo is developed on has been OOM-killed by parallel
renders. Each finished gif lands on disk before the next starts, so a crash
loses at most one tape.

## Architecture

Wits is a **ledger**. `.wits/journal.ndjson` is an append-only, hash-chained
event log; everything on screen is derived by replaying it (`ledger.Fold`).
Nothing is edited in place — corrections are compensating entries (`wits
revert`, adjustments), and the log keeps both. This is load-bearing: the replay
transport, the Séance, and witsnap's JSON all exist because any state can be
rebuilt from any prefix of events.

Grams move between four accounts (storage → stash → consumed → avb), one stash
per product, every entry a transfer — mass is conserved end to end. `pkg/record`
applies entries with the guard rails (no overdrawing an account); both the CLI
commands and the TUI forms go through it, so they cannot disagree.

Dependency direction is strict and downward: `pkg/journal` depends on nothing
internal; `workspace` opens a repo and holds state; `tui` and `cmd/*` read from
the ledger and nothing else. A future server is another caller of
`pkg/workspace`, not a new layer.

### Cycles and the three scopes

A **cycle** is one prescription fill. It opens with a purchase (purchases
within `ledger.CycleGap` = 3 days are one pickup) and **closes when its own
jars are empty** — not when the next fill arrives. Cycles overlap. Storage is
tracked as per-fill **lots drained oldest-first (FIFO)**, so every gram stands
on the account of the cycle that dispensed it and nothing can read over 100%.

Every figure on screen speaks exactly one of three scopes, and must say which
(the table in ROADMAP.md is the doctrine — check new UI lines against it):

- **the fill** — one cycle's own grams (dashboard storage card, `wits status`)
- **the shelf** — every jar standing today (supply projection, "N older jars" lines)
- **the jar** — one product's lifetime (storage screen detail, history table)

Mixing scopes in one line, or leaving a figure's scope unlabeled, has been the
single most recurring bug class in this codebase.

### The TUI

Bubble Tea **v2** — imports are `charm.land/...`, not
`github.com/charmbracelet/...`. Do not add terminal charting libraries: they
target v1, and mixing the majors puts two renderers in one binary. Charts are
drawn in-tree (`pkg/tui/chart.go`).

Conventions that hold throughout `pkg/tui`:

- huh v2 forms are embedded as models driven by `Update` — never call
  `form.Run()` (it blocks the event loop; the previous interface died of this).
- Keybindings are declared once (`defaultKeys`, per-screen key structs) so the
  help line can never advertise a key the dispatch doesn't honour. `g` grinds
  (`n` is a silent alias); jump-to-top is `home` only.
- One replay transport (`player` in analysis.go) is shared by every screen that
  can play the ledger back; adjustments ride along silently (`skippable`).
- `Snapshot` (witsnap's entry point) drains Bubble Tea commands the way the
  runtime does — huh advances fields via commands, so dropping them renders
  forms blank. Test helpers (`send` in entry_test.go) do the same.
- Dialogs must keep their confirmation on screen at real terminal heights; a
  taller-than-terminal form silently submits its default (the clean-history
  bug). Fold long lists (`newCleanHistoryForm` is the pattern).

### Tests tell the truth

The importer is tested against the real committed workbook
(`assets/Tracking.2022.cleaned.xlsx`), not fixtures — a fixture agrees with
whatever the code believes, and this file does not. Figures asserted in tests
were reconciled against the workbook independently. Keep that spirit: when a
screen's arithmetic changes, verify it against the imported workbook via
witsnap, not only against synthetic events.

## Process

- **ROADMAP.md replaces GitHub Issues** — features, bugs and their rationale
  live there, and the "Built" section is kept exact against the code. When
  behaviour changes, update ROADMAP.md and README.md in the same PR; both are
  verified claims, not aspirations.
- Branch → PR → green checks → rebase-merge → release. Nothing merges red.
- Commit messages: conventional-commit prefixes with the repo's own voice —
  lowercase, declarative, a body that says *why* ("cycles close on empty, and
  every gram knows its fill"). Read `git log` before writing one.
- DEVDIARY.md is a per-day narrative log (newest first, see `dev-diary.sh`) —
  add an entry for a significant day of work, with the day's commits.
```
