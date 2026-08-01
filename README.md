# 🥦 Wits — the Weed Information Tracking System

[![Codacy Badge](https://app.codacy.com/project/badge/Grade/582a945a5bf24ec79fc6b3894b24544d)](https://app.codacy.com/gh/TheDonDope/wits/dashboard?utm_source=gh&utm_medium=referral&utm_content=&utm_campaign=Badge_grade) [![Codecov Badge](https://codecov.io/gh/TheDonDope/wits/graph/badge.svg?token=9sWIVhEeIX)](https://codecov.io/gh/TheDonDope/wits)

Wits tracks what a cannabis patient was dispensed, what they have used, and what
is left.

It is a **ledger**. Everything it shows is derived by replaying an append-only
log, so nothing is edited in place: a mistake is corrected by recording a
correction, the way `git revert` adds a commit rather than rewriting one. The
data is a multi-year medical record, and that is how a medical record should
behave.

![Wits](./assets/wits-demo.gif)

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
  │ per product                       │  one tin per product
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

```sh
wits buy "Enua 22/1 Wedding Cake" 20g
wits grind wedding-cake 0.75
wits sesh wedding-cake 0.3 --device volcano --temp 185
wits status
```

Anything can be backdated with `--date 2026-07-29`, which is what makes a
forgotten evening loggable the next morning without pretending it was entered
then.

```console
$ wits status
On cycle 29, opened 2026-07-09 (day 24)

PRODUCT                       STORAGE  STASH   AVB    LEFT
420-evolution-ice-cream-cake  16.90g   3.10g   0.00g  84%
enua-citrus-slap              17.09g   2.91g   0.00g  85%
cantourage-mac1               17.23g   47.77g  0.00g  86%

total                         41.00g                  68%

41.00g of 60.00g left over 23 days, 11 of them with an entry
1.73g per active day, 1.37g median, 0.83g per elapsed day
About 24 days left at that rate
```

`wits` on its own opens the interface: a dashboard of what is left and how long
it will last, the journal, an analysis view scoping from the current cycle out to
the whole history, the products and the devices. Entries can be recorded there
too — `n` to grind, `s` for a session, `b` for a fill, `r` to weigh.

## Commands

| | |
| --- | --- |
| `wits init [dir]` | Create a repository |
| `wits buy <product> <amount>` | Record a prescription fill into storage |
| `wits grind <product> <amount>` | Move product from storage into its tin |
| `wits sesh <product> <amount>` | Record a session, drawing on the tin |
| `wits status` | What is left, and how long it will last |
| `wits log` | The journal, newest first |
| `wits revert <entry>` | Undo an entry by recording a correction |
| `wits reconcile <product> <weight>` | Make an account agree with the scale |
| `wits device add <name>` | Register a vaporizer |
| `wits temps <celsius>` | What a temperature is hot enough to release |
| `wits import <file.xlsx>` | Import a tracking spreadsheet |
| `wits export` | Markdown, for reading or publishing |
| `wits bundle` | The whole repository as one compact file |
| `wits restore <file>` | Rebuild a repository from a bundle |

Every command takes `--help`. `import` writes nothing unless given `--commit`.

## Correcting a mistake

Nothing is edited in place. An entry is undone by recording a correction that
moves the same grams back the way they came, so both stay in the log:

```sh
wits log --oneline -n 1
wits revert 8297238 --reason "misread the scale"
```

In the interface, `e` amends an amount and `d` undoes an entry. The log shows what
currently stands; `v` reveals the corrections behind it.

## When the ledger and the scale disagree

Ledgers drift. A little is spilled, a session goes unlogged, a scale is read
wrong. The past is not edited to hide it, because nobody knows which entry was
wrong — instead the difference is recorded, and the account agrees with the jar
again:

```console
$ wits reconcile wedding-cake 17.6 --dry-run
storage holds 18.00g by the ledger and 17.60g on the scale: -0.40g

$ wits reconcile wedding-cake 17.6 --reason "spilled on the desk"
[21ee6ae] adjusted 0.40g out of storage, now 17.60g
```

`--stash` weighs the tin instead, `--avb` the already vaped bud. In the interface
this is `r`, on any screen — it offers the jar under the cursor on the products
screen, or otherwise the fullest one, since that is the one worth checking.

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

Nearly three years of real history, 1369 entries across 48 products:

| | bytes | |
| --- | ---: | --- |
| journal | 500,729 | |
| **bundle** | **27,510** | **18×** |
| bundle, gzipped | 6,299 | 79× |

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
make            # build ./bin/wits
make test       # test with coverage
make cover      # coverage as HTML
make vet
```

`make build-windows` cross-compiles `bin/wits.exe`. `make render-tapes` re-records
`assets/wits-demo.gif` from `wits-demo.tape`; it needs `vhs`, `gum` and `ttyd`
(`make install` covers the first two).

`coverage.out` and `coverage.html` are ignored from source control.

## Changelog & Roadmap

Features, bugs and refactorings are tracked in [ROADMAP.md](./ROADMAP.md) rather
than GitHub Issues, so the plan and the code stay in one place. A detailed
changelog is in [CHANGELOG.md](./CHANGELOG.md).
