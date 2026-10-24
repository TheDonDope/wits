# CLAUDE.md

Guidance for Claude Code in this repository. README.md is for people using
wits; ROADMAP.md is the plan and the record of what is built; this file is how
to work on it.

## Start here

```sh
./doctor.sh            # what's installed; `./doctor.sh install` fetches the rest
make build             # ./bin/wits
make snap              # ./bin/witsnap — rebuild BOTH after TUI changes
make test              # go test -race ./... with coverage
./preflight.sh         # what CI and Codacy will say, said first — before every push
```

Single test: `go test ./pkg/ledger -run TestCycleGap -v`. TUI tests live in
`pkg/tui` and render screens to plain text; no terminal needed.

Building and testing need only Go. Everything else — `gh`, `git-chglog`,
`shellcheck`, lizard, vhs and its Chromium — is the loop around the code, and
`./doctor.sh` checks all of it. This machine is a Steam Deck (SteamOS, Arch
underneath, no pip, no sudo-installed dev packages, ~3 GB of free memory), so
`install` puts everything user-local: binaries in `~/.local/bin` and
`$GOPATH/bin`, a Python venv and Chromium's missing libraries under
`~/.cache/wits-dev`.

## Skills

Project skills in `.claude/skills/` hold the loops that recur. Use them rather
than rediscovering the steps:

| skill | when |
| --- | --- |
| `ship` | "open a PR", "merge it", "cut the release" — branch → PR → green → rebase-merge → release notes in the house style |
| `workbook` | the maintainer committed new data to the workbook — did anything change, and every figure that depends on it |
| `tapes` | re-record a GIF; verify it by looking at the frames |
| `diary` | the day's DEVDIARY.md entry |

## Working with the maintainer

- Requests come as short imperatives, one step at a time: "open the PR", then
  "merge it and cut the release". Do the step, report the outcome first, offer
  the next step in one line. Merging, releasing and pushing to main each need
  their own go-ahead; a branch push and a PR do not.
- **Verify against the real data**, not only the tests: import the workbook,
  run `wits status`, photograph the screen with witsnap, read the frame of a
  GIF. Quote figures you measured, never ones you expect.
- When the answer is "nothing changed", say so plainly and stop. Don't invent
  follow-up work; offer it.
- Every merged PR ships as its own minor release (`ship`). A diary-only PR is
  merged and not released.
- A tool missing mid-task is not a blocker: `./doctor.sh install`, then say in
  the report what was installed and where.

## Codacy

The gate refuses any **new** issue. `./preflight.sh` mirrors it:

- **Functions over 50 lines of code or cyclomatic complexity 15** (lizard).
  Extract a named helper — `folder` in `pkg/ledger`, `grindTotals` in
  `pkg/tui` are the house pattern.
- **A new non-test `.go` file** gets flagged for a missing package comment,
  because revive runs per file and can't see `doc.go`. Grow an existing file
  where it fits.
- **Shell scripts** go through shellcheck — skill helpers in `.claude/` too.
- **Python** (the workbook helper) goes through pydocstyle with both D212 and
  D213 on — they contradict each other, so any multi-line docstring is flagged.
  Keep docstrings to one line and explain in comments. Preflight doesn't check
  Python.

`./preflight.sh watch <pr>` polls a PR and prints the issues Codacy will
refuse. It can return while `build` is still pending; confirm with
`gh pr checks <pr> --watch`.

## Verifying TUI changes

`witsnap` photographs screens without a terminal:

```sh
./bin/witsnap screens --screen storage --press c,enter   # any screen, scripted keys
./bin/witsnap json                                       # the whole derived state
```

Seed a throwaway repository first (`./demo-seed.sh <dir> && cd <dir>`), or
import the real workbook for realistic data:
`wits init . && wits import <repo>/assets/Tracking.2022.cleaned.xlsx --commit`.

The GIFs in `assets/` are recorded from the `*.tape` files and kept current with
the screens they show — re-record when a change is visible (`tapes` skill).
**One tape at a time, nothing heavy alongside**: each render starts a Chromium,
and parallel renders have OOM-killed this machine.

## Architecture

Wits is a **ledger**. `.wits/journal.ndjson` is an append-only, hash-chained
event log; everything on screen is derived by replaying it (`ledger.Fold`).
Nothing is edited in place — corrections are compensating entries (`wits
revert`, adjustments), and the log keeps both. This is load-bearing: the replay
transport, the Séance, and witsnap's JSON all exist because any state can be
rebuilt from any prefix of events.

Grams move between four accounts (storage → stash → consumed → avb), one stash
per product, every entry a transfer — mass is conserved end to end.
`pkg/record` applies entries with the guard rails (no overdrawing an account);
the CLI commands and the TUI forms both go through it, so they cannot disagree.

Dependencies point strictly downward: `pkg/journal` depends on nothing
internal; `workspace` opens a repository and holds state; `tui` and `cmd/*`
read from the ledger and nothing else. A future server is another caller of
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

Mixing scopes in one line, or leaving a figure's scope unlabelled, has been the
single most recurring bug class in this codebase.

### The TUI

Bubble Tea **v2** — imports are `charm.land/...`, not
`github.com/charmbracelet/...`. Don't add terminal charting libraries: they
target v1, and mixing the majors puts two renderers in one binary. Charts are
drawn in-tree (`pkg/tui/chart.go`).

- huh v2 forms are embedded as models driven by `Update` — never call
  `form.Run()` (it blocks the event loop; the previous interface died of this).
- Keybindings are declared once (`defaultKeys`, per-screen key vars) so the help
  line can never advertise a key the dispatch doesn't honour. `g` grinds (`n` is
  a silent alias); jump-to-top is `home` only.
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
are reconciled against the cells read **independently of the importer**
(`.claude/skills/workbook/cells.py`). When a screen's arithmetic changes,
verify it against the imported workbook via witsnap, not only against synthetic
events.

## Process

- **ROADMAP.md replaces GitHub Issues** (blank issues are off, and the issue
  chooser points at the roadmap — `.github/ISSUE_TEMPLATE/config.yml`). Features, bugs and their rationale live
  there, and "Built" is kept exact against the code. When behaviour changes,
  update ROADMAP.md and README.md in the same PR — both are verified claims, not
  aspirations.
- Branch → PR → green checks → rebase-merge → release. Nothing merges red.
- Commit messages: conventional prefixes in the repo's own voice — lowercase,
  declarative, a body that says *why* ("cycles close on empty, and every gram
  knows its fill"). Read `git log` before writing one.
- Releases: `make release VERSION=vX.Y.0` from an up-to-date main (changelog,
  tag, push), then `gh release create` with a short poetic title — `ship`
  has the style.
- DEVDIARY.md is a per-day narrative, newest first — `diary` skill.
- Dependabot opens grouped weekly PRs (`.github/dependabot.yml`). Security
  alerts are checked with
  `gh api 'repos/TheDonDope/wits/dependabot/alerts?state=open'`.
