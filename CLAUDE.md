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
make serve             # wits-server + the Angular client on http://127.0.0.1:7341
make ui-test           # the client's Vitest suite (Node and npm; nothing else needs them)
make wasm-check        # the phone's engine (cmd/wits-wasm) synced with the CLI, both ways
make pages             # the phone's build, as GitHub Pages serves it under /wits/
make appimage          # dist/wits-<version>-x86_64.AppImage: server, CLI and the UI in an Electron window, checked by running it (eval "$(./doctor.sh env)" first here: it carries NSS)
make demo-seed         # assets/demo-seed.wits again: the workbook, invoices, devices, a year of sessions, terpenes — minutes, live web
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
- **A new non-test `.go` file needs its own package comment.** Revive runs one
  file at a time and can't see `doc.go`, so a file without one is flagged; a
  file with one passes (`pkg/chain/chain.go` in #131, the files split out in
  #133). It must sit directly on the package clause, no blank line between,
  and read `// Package <name> ...` — `Package name:` is not the form. Preflight
  runs revive on each new file (`./doctor.sh install` fetches it). Put code in
  the file it belongs in, not the nearest
  existing one: commands filed under other commands' names (`carry` in
  `status.go`) and four concerns in `report.go` came of avoiding new files.
- **Shell scripts** go through shellcheck — skill helpers in `.claude/` too.
- **Python** (the workbook helper) goes through pydocstyle with both D212 and
  D213 on — they contradict each other, so any multi-line docstring is flagged.
  Keep docstrings to one line and explain in comments. Preflight doesn't check
  Python.

- **TypeScript** (wits-ui) is excluded from Codacy's ESLint in `.codacy.yml`:
  it runs without `node_modules`, so every Angular type is unresolved and its
  type-aware rules flag every use. The client is linted with types instead —
  `npx ng lint` (strict type-checked rules) and `npm run lint:styles`
  (stylelint-config-standard-scss, the standard Codacy's Stylelint applies),
  run by preflight and by the wits-ui workflow. Keep both clean.

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
revert`, adjustments), and the log keeps both. The one rewrite is `wits pull`
(`Journal.Rebase`): entries not yet pushed are chained again on top of the
remote's, with new hashes, and the effect log follows them. This is
load-bearing: the replay transport and witsnap's JSON both exist because any
state can be rebuilt from any prefix of events.

Two orders, never mixed: the journal is in **recorded order** (the hash chain),
the fold in **occurred order** (`ledger.Chronological`, ties kept recorded).
`State.Events` is the fold's order, every entry. Anything that needs the chain
— bundles — reads `Journal().Events()`. Nothing is dated ahead: the recorder
refuses an entry dated after now (`record.ErrAhead`), so there is one fold and
no "upcoming". Entries can be backdated anywhere, so the recorder
checks a new one against every date in the replay (`ledger.Shortfalls`), not
only against today's balance. Grams are physical: nothing models a negative
balance or an overdraft — an entry either had its grams in the jar or is
refused. A pull can bring one in that did not (two devices, the same last
grams); the jar then takes nothing but an undo (`record.ErrClash`) until one
of the two is reverted.

The **effect log** (`pkg/effects`, `.wits/effects.ndjson`) sits beside the
journal, not in it: a rating moves no grams, and the journal's every entry is a
transfer. It has its own hash chain and lock, names sessions by hash, and a
re-rating supersedes (`effects.Latest`). The **price log** (`pkg/prices`,
`.wits/prices.ndjson`) is its twin for what purchases cost, named by hash,
re-pricing superseding (`prices.Latest`), cents in euros. Anything that copies
the repository — bundles, restore, fsck, push and pull — must carry both, and
the **seals** (`pkg/seal`, `.wits/seals/`) beside them: a timestamp proof cannot
be made again later. `.wits/seal.key` and `.wits/remote.key` never travel.

Grams move between four accounts (storage → stash → consumed → avb), one stash
per product, every entry a transfer — mass is conserved end to end. Add amounts
in `journal.Centigrams` (`journal.CG`), not floats with `ledger.Round` after;
read typed amounts with `ledger.ParseGrams`.
`pkg/record` applies entries with the guard rails (nothing takes more than a jar holds);
the CLI commands and the TUI forms both go through it, so they cannot disagree.
Every recorder entry is a builder run by `Journal.AppendWith`, under the lock,
against a fold of the journal as it stands — a check made outside a builder is
a check of a snapshot another process may have overtaken. The same goes for the
catalogs and the config: change them through `repo.UpdateProducts` /
`UpdateDevices` / `SetTheme` (locked, re-read, written atomically), never by
saving a copy held in memory.

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
single most recurring bug class in this codebase. The scopes are types —
`ledger.Fill`, `ledger.Shelf`, `ledger.Jar` — so a mix does not compile. Keep
new figures in their type for as long as possible; a conversion between them,
or to `float64`, is a claim about scope, and belongs where a comment can say
why it holds. Helpers that take one figure of either side (`percent`, `grams`)
are generic over `~float64`, so both sides must be of one scope.

### The TUI

Bubble Tea **v2** — imports are `charm.land/...`, not
`github.com/charmbracelet/...`. Don't add terminal charting libraries: they
target v1, and mixing the majors puts two renderers in one binary. Charts are
drawn in-tree (`pkg/tui/chart.go`).

- huh v2 forms are embedded as models driven by `Update` — never call
  `form.Run()` (it blocks the event loop; the previous interface died of this).
- A form goes in front through the one `App.front` slot: implement `modal`
  (`init`, `update`, `dismiss`, `view`) and open it with `a.show(...)`. Keys,
  esc and the notice are handled there once. A screen's edit/delete/add keys
  are a row in `corrections`, not another case in a switch.
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

## The browser interface

`pkg/api` is the server, `cmd/wits-server` only wires it; `wits-ui/` is the
Angular client (signals, zoneless, standalone components, `httpResource`).

- Every write goes through `ws.Recorder` — the API adds no checks of its own and
  must not skip any. Name a figure for its scope (`dispensed`, `shelf_projection`).
- The client fetches nothing from the network: fonts and icons are npm packages
  (`@fontsource-variable/*`, `material-symbols`). An `ng add` that links a font
  service has to be undone.
- Charts are SVG drawn by components in `wits-ui/src/app/viz`, geometry from d3
  (`d3-shape`, `d3-scale`), never a chart library. Honour reduced motion.
- **The phone** runs the same API in the page: `cmd/wits-wasm` (`GOOS=js`) on
  `data/memfs.ts`, answered through `engineInterceptor` when `ENGINE` is set
  (the `pages` configuration swaps in `mode.pages.ts`). Anything the server
  does, the phone does by the same code — so a new endpoint needs nothing
  phone-side, but a new `os` call Go makes may need MemFS to answer it, and
  `make wasm-check` is what finds out. The phone has the desk too
  (`pkg/api/desk.go`: opening another repository, browsing folders, importing
  a workbook) on its own file system, home `/`, the first repository at
  `/wits`; the spreadsheet reader costs its engine 450 KB gzipped. Its
  scratch folders are hidden (`/.remote`, `/.tmp`) so the Repository page
  lists only repositories, and `kept` in `data/engine.ts` keeps everything
  else in IndexedDB — a new scratch path must be added to both.
- **Colours are the theme's.** A theme is data in `src/themes.scss`, listed for
  the picker in `data/theme.ts` (the two must agree, id for id). Components
  read `var(--w-…)` or `color-mix` one — never a literal, or one theme in
  twenty-one looks wrong; a canvas reads the variables off the page.
- **A phone is 375 px upright** (iPhone 11 Pro), the narrowest width laid out
  for. Its rules go under `@include bp.phone` (`src/_breakpoints.scss`, with
  `@use 'breakpoints' as bp;`), never a written-out width. Every page is checked
  there: no page wider than the screen (`scrollWidth`), nothing paged or cut off.
- A figure the form shows and the server writes is worked out the same way on
  both sides, step for step and rounded once (`rowCents` in `fill-form.ts`,
  `lineCents` in `pkg/api/fill.go`), with a test on each pinning one case.
- **Verify by looking**: `make serve` on a copy of the workbook, then a headless
  Chromium screenshot (`~/.cache/rod/browser/*/chrome --headless=new
  --force-prefers-reduced-motion --screenshot=… --window-size=1440,1700 <url>`,
  with `./doctor.sh env` for its libraries) and read the PNG.
  `?record=grind|session|fill|weigh|avb` opens the form, `?theme=<id>` wears a
  theme, for photographing them. A phone is emulated through the DevTools
  protocol (`Emulation.setDeviceMetricsOverride`, `mobile: true`), not by
  `--window-size`, which headless widens past 375. That Chromium is old — no
  `Promise.try`, no WebCrypto Ed25519 — so a library that needs either needs
  its legacy build or a fallback. Start each run with a fresh `--user-data-dir`:
  a reused one keeps the app's service worker, which serves yesterday's chunks.

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
