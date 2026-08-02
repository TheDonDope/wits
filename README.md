# 🥦 Wits — the Weed Information Tracking System

[![Codacy Badge](https://app.codacy.com/project/badge/Grade/582a945a5bf24ec79fc6b3894b24544d)](https://app.codacy.com/gh/TheDonDope/wits/dashboard?utm_source=gh&utm_medium=referral&utm_content=&utm_campaign=Badge_grade) [![Codecov Badge](https://codecov.io/gh/TheDonDope/wits/graph/badge.svg?token=9sWIVhEeIX)](https://codecov.io/gh/TheDonDope/wits)

Wits tracks what a cannabis patient was dispensed, what they have used, and what
is left.

It is a **ledger**. Everything it shows is derived by replaying an append-only
log, so nothing is edited in place: a mistake is corrected by recording a
correction, the way `git revert` adds a commit rather than rewriting one. The
data is a multi-year medical record, and that is how a medical record should
behave.

![The Wits interface](./assets/wits-tour.gif)

## How it thinks

Grams move through four accounts. Every entry is a transfer between two of them,
so nothing is lost between the pharmacy and the ashes.

```text
   pharmacy / prescription
        │  buy
        ▼
  ┌───────────┐        grind          ┌───────────┐
  │  STORAGE  │ ────────────────────▶ │   STASH   │
  │  sealed,  │                       │  ground,  │
  │ per product                       │  one per product
  └───────────┘                       └─────┬─────┘
                                            │  sesh · device + temperature
                                            ▼
                                      ┌───────────┐
                                      │ CONSUMED  │
                                      └─────┬─────┘
                                            │  weighed after collection
                                            ▼
                                      ┌───────────┐
                                      │    AVB    │ ──────▶ edibles / tincture
                                      └───────────┘
```

A **cycle** is one prescription fill, running until the next one — not a calendar
month. That is why a fill lasting six weeks, a month with two fills, and a month
with none all work without special cases.

## Getting started

```sh
go install github.com/TheDonDope/wits/cmd/wits@latest
```

A repository is created the way a git one is, and every command finds it by
walking up from the working directory.

```sh
mkdir -p ~/wits && cd ~/wits
wits init .
```

Record a fill, then use it:

```console
$ wits buy "Enua 22/1 Wedding Cake" 20g
New product Enua 22/1 Wedding Cake — refer to it as wcake-221
[029efe8] purchase 20.00g wcake-221 into storage
```

Every product gets a short handle: three to five characters from the cultivar,
then the THC/CBD ratio. The ratio is there because the same cultivar from the
same maker at two strengths is two prescriptions — `wcake-221` and `wcake-251`
are legible as a pair, and adding their grams together would be wrong. Pass
`--slug lemon` to choose your own, which is taken as written. A handle is
settled when the product first appears and never changes, because it is the
name every later entry refers to.

```sh
wits grind wcake 0.75
wits sesh wcake-221 0.3 --device volcano --temp 185
wits status
```

References resolve by prefix, so `wcake` is enough while it is unambiguous.

![Recording a fill and a session](./assets/wits-cli.gif)

Tab completion offers the handles, with the full name and how much is left
beside each — and only the ones the command can act on, so `sesh` offers what is
in a stash rather than everything ever dispensed:

```console
$ wits grind <TAB>
lcook-281   10.00 g · Cannamedical 28/1 Lemon Cookie
mac1-251    15.00 g · Cantourage 25/1 MAC1+
wcake-221   18.00 g · Enua 22/1 Wedding Cake
```

Install it with `wits completion bash` (or `zsh`, `fish`).

Anything can be backdated with `--date 2026-07-29`, which is what makes a
forgotten evening loggable the next morning without pretending it was entered
then.

```console
$ wits status
On cycle 29, opened 2026-07-09 (day 24)

PRODUCT                           STORAGE  STASH   AVB    LEFT
420-evolution-ice-cream-cake-271  16.90g   3.10g   0.00g  84%
enua-citrus-slap-361              17.09g   2.91g   0.00g  85%
cantourage-mac1-251               17.23g   47.77g  0.00g  86%

total                             41.00g                  68%

41.00g of 60.00g left over 23 days, 11 of them with an entry
1.73g per active day, 1.37g median, 0.83g per elapsed day
About 24 days left at that rate
```

`wits` on its own opens the interface: a dashboard of cards — storage, stash,
sessions, devices, two rhythm calendars and a projection of the supply's
decline to its empty day — then the journal, an analysis view scoping from the
current cycle out to the whole history, the storage, the stash, the sessions
and the devices. Entries can be recorded there too — `n` to grind, `s` for a
session, `b` for a fill, `r` to weigh.

The analysis view draws the daily amounts as a braille area chart with a
seven-day average riding over it, and the longer scopes as a calendar heatmap —
one cell per day, colour carrying the amount — so a year of habit reads the way
a contribution graph does: the heavy weeks, the pauses, whether weekends
differ.

## Commands

| | |
| --- | --- |
| `wits init [dir]` | Create a repository |
| `wits buy <product> <amount>` | Record a prescription fill, `--slug` to name it |
| `wits grind <product> <amount>` | Move product from storage into its stash |
| `wits sesh <product> <amount>` | Record a session, drawing on the stash |
| `wits status` | What is left, and how long it will last |
| `wits log` | The journal, newest first |
| `wits revert <entry>` | Undo an entry by recording a correction |
| `wits reconcile [account] [product] [weight]` | Make an account agree with the scale; interactive with no arguments |
| `wits device add <name>` | Register a vaporizer |
| `wits temps <celsius>` | What a temperature is hot enough to release |
| `wits import <file.xlsx>` | Import a tracking spreadsheet |
| `wits export` | Markdown, for reading or publishing |
| `wits bundle` | The whole repository as one compact file |
| `wits restore <file>` | Rebuild a repository from a bundle |

Every command takes `--help`. `import` writes nothing unless given `--commit`.

## Bringing a spreadsheet across

`wits import` reads a tracking workbook and turns each worksheet into a
prescription fill and the grinds that followed it. The default is a dry run: it
reports what it would record, and anything about the spreadsheet that does not
add up, so years of history can be checked before any of it is written.

![Importing four years of a spreadsheet](./assets/wits-import.gif)

Products are resolved **by position**, through the bindings in the running
balance formulas, rather than by the label in the strain column — those labels
are dropdown values that were not always renamed as products changed. On the
records this was written for, trusting the labels misplaces 1116.97 g.

## Correcting a mistake

Nothing is edited in place. An entry is undone by recording a correction that
moves the same grams back the way they came, so both stay in the log:

```sh
wits log --oneline -n 1
wits revert 8297238 --reason "misread the scale"
```

The storage screen is two tables: what still holds something, and the history
of every jar weighed down to zero, newest first. The stash screen drills into
the ground product the same way — the stashes holding something above, and
under them every stash worked down to nothing, grouped under the day it was
consumed. The sessions screen tells the other half of the story: how much came
out of the stash, when, through which device and how hot, drawn with the same
charts the analysis view uses. Space ticks jars in either
table and `r` weighs the ticked ones together; `e` corrects a name; `c` records
stale stash remainders from earlier cycles as consumed, which is what four
imported years of grind-only records leave behind. Product names are never
abbreviated — the name column takes what the longest name needs. In the
journal, `e` amends an amount and `d` undoes an entry. The log shows what
currently stands; `v` reveals the corrections behind it.

## When the ledger and the scale disagree

Ledgers drift. A little is spilled, a session goes unlogged, a scale is read
wrong. The past is not edited to hide it, because nobody knows which entry was
wrong — instead the difference is recorded, and the account agrees with the jar
again:

```console
$ wits reconcile storage wcake-221 17.6 --dry-run
storage holds 18.00g by the ledger and 17.60g on the scale: -0.40g

$ wits reconcile storage wcake-221 17.6 --reason "spilled on the desk"
[21ee6ae] adjusted 0.40g out of storage of wcake-221, now 17.60g
```

![Reconciling against the scale](./assets/wits-reconcile.gif)

For weighing day, `wits reconcile` on its own is interactive: pick storage or
the stash, tick the jars to weigh — all of them by default — and each is asked
for in turn, with the ledger's figure beside the prompt. A blank reading skips
a jar; `wits reconcile stash` skips the first question. In the interface this
is `r`, on any screen — it weighs the jars ticked on the storage screen, the
one under the cursor, or otherwise the fullest one, since that is the one
worth checking.

## The repository

```text
.wits/
  config.yml      # settings
  products.yml    # the catalog
  devices.yml     # vaporizers and their temperature ranges
  journal.ndjson  # append-only, one entry per line, never rewritten
  index/          # reserved for a cached fold; nothing writes it yet
```

The journal is only ever appended to, and each entry is chained to the one before
it with a SHA-256 hash, so an edit made outside Wits is detectable. Appending
takes an advisory file lock as well as a mutex, because it is a read-then-write
and two processes reading the same tip would fork the chain.

The directory is created `0700` and its files `0600`. Nothing is transmitted
anywhere; the application makes no network calls at all.

## Bundles

`wits bundle` writes the catalogs and every entry to a single file that
`wits restore` reads back, reproducing the journal **exactly, hash chain
included**. That is what makes it worth trusting as a backup.

It is plain text, so the record stays legible with nothing but a text editor and
diffs cleanly in git. Small, too, because most of what the journal stores is
derivable and is left out: sequence numbers, account pairs and the whole hash
chain are recomputed on restore.

Nearly three years of real history, 1369 entries across 50 products:

| | bytes | |
| --- | ---: | --- |
| journal | 506,205 | |
| **bundle** | **27,968** | **18×** |
| bundle, gzipped | 6,399 | 79× |

![Bundling and restoring](./assets/wits-bundle.gif)

## Temperatures

Wits knows the boiling point of every cannabinoid and terpene, so a number on a
dial reads as what it actually does — including the point at which it starts
producing benzene.

```console
$ wits temps 210
COMPOUND         BOILS AT  EFFECTS
THCA             120°C     anti-inflammatory, anti-epileptic, anti-proliferic
CBDA             130°C     anti-inflammatory, anti-proliferic
β-Caryophyllene  130°C     anti-malarial, cytoprotective, anti-inflammatory
…
⚠️  210°C is at or above the 205°C boiling point of Benzene.
```

The devices screen shows the same for whichever device is selected, at its
default setting.

## Layout

One Go module, several commands, and a web interface beside them. Not a module
per component: the domain is shared, and a boundary between a server and the
ledger it serves would mean versioning the ledger against itself.

```text
wits/                     module github.com/TheDonDope/wits
  cmd/
    wits/                 the terminal interface and the commands
    witsnap/              the camera and the tap: screens as text, the fold as JSON
    wits-server/          the REST API                        (planned)
  pkg/
    journal/              the append-only log
    ledger/               the fold: balances, cycles, statistics
    repo/                 finding and creating a .wits directory
    workspace/            opening a repository and holding its state
    catalog/              products and devices
    record/               applying entries, with the checks that guard them
    bundle/               the portable archive format
    importer/             reading the tracking spreadsheet
    cannabis/             cannabinoids, terpenes and their boiling points
    tui/                  the screens
    version/              the build stamp every binary reports from
  wits-ui/                the web interface, in Angular       (planned)
```

`pkg/journal` depends on nothing else here and everything above depends downwards
only, so a server is another caller of `pkg/workspace`, not a new layer.

## Built with

- [bubbletea](https://github.com/charmbracelet/bubbletea) — the TUI framework 🏗
- [bubbles](https://github.com/charmbracelet/bubbles) — components 🫧
- [huh](https://github.com/charmbracelet/huh) — forms and prompts 🤷
- [lipgloss](https://github.com/charmbracelet/lipgloss) — layout and style 👄
- [vhs](https://github.com/charmbracelet/vhs) — the recording above 📼
- [cobra](https://github.com/spf13/cobra) — the command line

All on the v2 line, which lives under `charm.land/…` rather than
`github.com/charmbracelet/…`.

The charts are drawn in-tree. The terminal charting libraries still target Bubble
Tea v1, and mixing the majors puts two renderers and two colour-profile detectors
in one binary, which shows up on screen as inconsistent colour.

## Development

```sh
make build      # build ./bin/wits
make run        # build it and run it
make test       # test with coverage
make cover      # coverage as HTML
make vet
make preflight  # everything CI and the Codacy gate will say, said here first
```

`./preflight.sh watch <pr>` polls a pull request's checks and reads the Codacy
delta the way the gate does, so a red X never comes as a surprise.

`make snap` builds `witsnap`, the camera and the tap: `witsnap screens` renders
any screen as plain text — `--press p,tick,tick` photographs a replay mid-run —
and `witsnap json` writes the whole derived state as JSON, which is the seam a
new client or a new visualisation starts from.

`make build-windows` cross-compiles `bin/wits.exe`. `make render-tapes` re-records
every GIF in `assets/` from the `*.tape` files; it needs `vhs`, `gum` and `ttyd`
(`make install` covers the first two). The tapes seed a throwaway repository
with `demo-seed.sh` first, because `wits` reads a `.wits` directory and a source
checkout has none.

`coverage.out` and `coverage.html` are ignored from source control.

## Changelog & Roadmap

Features, bugs and refactorings are tracked in [ROADMAP.md](./ROADMAP.md) rather
than GitHub Issues, so the plan and the code stay in one place. A detailed
changelog is in [CHANGELOG.md](./CHANGELOG.md).
