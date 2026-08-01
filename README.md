# Wits - The 🥦 Information Tracking System

[![Codacy Badge](https://app.codacy.com/project/badge/Grade/582a945a5bf24ec79fc6b3894b24544d)](https://app.codacy.com/gh/TheDonDope/wits/dashboard?utm_source=gh&utm_medium=referral&utm_content=&utm_campaign=Badge_grade) [![Codecov Badge](https://codecov.io/gh/TheDonDope/wits/graph/badge.svg?token=9sWIVhEeIX)](https://codecov.io/gh/TheDonDope/wits)

Wits helps cannabis patients track what they were dispensed, what they have used
and what is left.

It is a **ledger**. Everything it shows is derived by replaying an append-only log
of events, so nothing is ever edited in place: a mistake is corrected by recording
a correction, the way `git revert` adds a commit rather than rewriting one. The
data is a multi-year medical record, and that is how a medical record should
behave.

## The model

Grams move through four accounts. Every entry is a transfer between two of them,
so nothing is lost track of between the pharmacy and the ashes.

```text
   pharmacy / prescription
        │  buy
        ▼
  ┌───────────┐        grind          ┌───────────┐
  │  STORAGE  │ ────────────────────▶ │   STASH   │
  │  sealed,  │                       │  ground,  │
  │ per product                       │  one tin per product
  └───────────┘                       └─────┬─────┘
                                            │  sesh (device + temperature)
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

A **cycle** is one prescription fill, running until the next one. It is not a
calendar month, which is why a fill that lasts six weeks, a month with two fills,
and a month with none all work without special cases.

## Getting started

A repository is created the way a git one is, and commands find it by walking up
from the working directory.

```sh
mkdir -p ~/wits && cd ~/wits
wits init .
```

Then record a prescription fill and use it:

```sh
wits buy "Enua 22/1 Wedding Cake" 20g
wits grind wedding-cake 0.75
wits sesh wedding-cake 0.3 --device volcano --temp 185
wits status
```

Anything can be backdated with `--date 2026-07-29`, which is what makes a
forgotten evening loggable the next morning without pretending it was entered
then.

`wits` on its own opens the interface: a dashboard of what is left and how long
it will last, the journal, and an analysis view scoping from the current cycle out
to the whole history. Entries can be recorded there too.

## Commands

| Command | |
| --- | --- |
| `wits init [dir]` | Create a repository |
| `wits buy <product> <amount>` | Record a prescription fill into storage |
| `wits grind <product> <amount>` | Move product from storage into its tin |
| `wits sesh <product> <amount>` | Record a session, drawing on the tin |
| `wits status` | What is left, and how long it will last |
| `wits log` | The journal, newest first |
| `wits revert <entry>` | Undo an entry by recording a correction |
| `wits device add <name>` | Register a vaporizer |
| `wits temps <celsius>` | What a temperature is hot enough to release |
| `wits import <file.xlsx>` | Import a tracking spreadsheet (dry run unless `--commit`) |
| `wits export` | Markdown, for reading or publishing |
| `wits bundle` | The whole repository as one compact file |
| `wits restore <file>` | Rebuild a repository from a bundle |

## Correcting a mistake

Nothing is edited in place. An entry is undone by recording a correction that
moves the same grams back the way they came, so both stay in the log:

```sh
wits log --oneline -n 1        # find the entry
wits revert 8297238 --reason "misread the scale"
```

In the interface, `e` amends an amount and `d` undoes an entry. The log shows
what currently stands; `v` reveals the corrections behind it.

## Layout

One Go module, several commands, and a web interface beside them. A module per
component would mean a boundary between a server and the ledger it serves, and so
versioning the ledger against itself on every change; the domain is shared, so it
stays in one module.

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

`pkg/journal` depends on nothing else here, and everything above it depends
downwards only. A server is another caller of `pkg/workspace`, not a new layer.

## The repository

```text
.wits/
  config.yml      # settings
  products.yml    # the catalog
  devices.yml     # vaporizers and their temperature ranges
  journal.ndjson  # append-only, one event per line, never rewritten
  index/          # cached fold — disposable
```

The journal is only ever appended to, and each entry is chained to the one before
it with a SHA-256 hash, so an edit made outside Wits is detectable. The directory
is created `0700` and its files `0600`.

## Bundles

`wits bundle` writes the catalogs and every event to a single file that
`wits restore` reads back, reproducing the journal exactly, hash chain included.
It is plain text, so the record stays legible with nothing but a text editor and
diffs cleanly in git.

It is small regardless, because most of what the journal stores is derivable and
is therefore left out — sequence numbers, account pairs and the hash chain are all
recomputed on restore. Nearly three years of real history, 1369 entries across 48
products:

| | bytes | |
| --- | ---: | --- |
| journal | 500,729 | |
| **bundle** | **27,510** | **18×** |
| bundle, gzipped | 6,299 | 79× |

## Temperatures

Wits knows the boiling point of each cannabinoid and terpene, so a number on a
dial can be read as what it actually does — including the point at which it starts
producing benzene.

```sh
$ wits temps 210
COMPOUND         BOILS AT  EFFECTS
THCA             120°C     anti-inflammatory, anti-epileptic, anti-proliferic
CBDA             130°C     anti-inflammatory, anti-proliferic
β-Caryophyllene  130°C     anti-malarial, cytoprotective, anti-inflammatory
…
⚠️  210°C is at or above the 205°C boiling point of Benzene.
```

## Built with

- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea): A powerful little TUI framework 🏗
- [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles): TUI components for Bubble Tea 🫧
- [charmbracelet/huh](https://github.com/charmbracelet/huh): Build terminal forms and prompts 🤷
- [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss): Style definitions for nice terminal layouts 👄
- [charmbracelet/vhs](https://github.com/charmbracelet/vhs): Your CLI home video recorder 📼
- [spf13/cobra](https://github.com/spf13/cobra): A Commander for modern Go CLI interactions

All on the v2 line, which lives under `charm.land/…` import paths rather than
`github.com/charmbracelet/…`.

The charts are drawn in-tree rather than pulled in. The terminal charting
libraries still target Bubble Tea v1, and mixing the majors would put two
renderers and two colour-profile detectors in one binary, which shows up on screen
as inconsistent colour.

## Changelog & Roadmap

A detailed changelog can be found in the [CHANGELOG.md](./CHANGELOG.md) and the
current development progress is tracked in the [ROADMAP.md](./ROADMAP.md). We do
not use GitHub Issues but instead track our features, bugfixes and refactorings
there.

## Building & Running

Building the binary and running it requires only a simple invocation to `make`:

```sh
make
```

![Wits Make Video](./vhs-output/wits-make.gif)

For windows, the `wits.exe` can be built by invoking the `make build-windows`
command:

```sh
make build-windows
```

![Wits Make Windows Video](./vhs-output/wits-make-windows.gif)

## Running Tests

- Run the testsuite with coverage enabled:

```sh
make test
```

![Wits Make Test Video](./vhs-output/wits-make-test.gif)

- Generate the coverage results as html:

```sh
make cover
```

![Wits Make Cover Video](./vhs-output/wits-make-cover.gif)

- Open the results in the browser:

```sh
make show-cover
```

![Wits Make Show Cover Video](./vhs-output/wits-make-show-cover.gif)

Both the `coverage.out` as well as the `coverage.html` are explicitly ignored from
source control (see [.gitignore](.gitignore)).
