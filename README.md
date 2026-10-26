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
                                            │  avb collect · weighed out of the device
                                            ▼
                                      ┌───────────┐  avb use
                                      │    AVB    │ ──────▶ edibles / tincture
                                      └───────────┘
```

A **cycle** is one prescription fill: it opens with the purchase and closes
when its own jars are empty — not a calendar month, and not the arrival of the
next fill. A fill lasting six weeks, a month with two fills, a month with
none, and a remainder outliving its successors all work without special
cases; cycles simply overlap, each keeping account of what it dispensed until
the last of it is ground.

## Three ways in

| | |
| --- | --- |
| **The terminal** — `wits` and its commands, and the interface `wits` opens on its own | [docs/terminal.md](./docs/terminal.md) |
| **A browser or a window** — `wits-server` and the interface it serves, the AppImage's window | [docs/browser.md](./docs/browser.md) |
| **The phone** — the same interface with wits inside it, from GitHub Pages | [docs/browser.md#on-the-phone](./docs/browser.md#on-the-phone) |

All three read and write one repository through the same recorder, so they
refuse the same things in the same words. The browser interface carries a
**User guide** of its own, on the header's menu (⋮): every page with pictures,
and the command beside each that does the same in a terminal.

## Getting started

On Linux, the simplest is the AppImage attached to every
[release](https://github.com/TheDonDope/wits/releases): one file, about
130 MB, holding the server, the command line, and the interface in a window
of its own.

```sh
chmod +x wits-*-x86_64.AppImage
./wits-*-x86_64.AppImage              # serves ~/.local/share/wits, in its own window
./wits-*-x86_64.AppImage ~/wits       # or that repository, made there if new
./wits-*-x86_64.AppImage cli status   # the command line
```

The first launch makes its repository, and the Repository page opens another,
makes one, or imports the workbook. The window is the app: closing it stops
the server, and launching it again while it runs brings the window to the
front. Links to other sites, a terpene source say, open in your browser.
`WITS_ADDR` changes where it listens (127.0.0.1:7341, this machine only),
`WITS_BROWSER=1` opens the interface in the browser instead, as it used to,
and `WITS_NO_WINDOW=1` runs the server alone. Each release lists its
`SHA256SUMS`.

Why Electron, and what the window lets through:
[The desktop window](./docs/browser.md#the-desktop-window).

Or with Go:

```sh
go install github.com/TheDonDope/wits/cmd/wits@latest
```

From a checkout, `make install-wits` installs the working tree to the same
place instead — see [Development](#development).

A repository is created the way a git one is, and every command finds it by
walking up from the working directory.

```sh
mkdir -p ~/wits && cd ~/wits
wits init .
```

Record a fill, then use it:

```console
$ wits buy "Enua 36/1 Citrus Slap" 20g
New product Enua 36/1 Citrus Slap — refer to it as cslap-361
[ea8b2de] purchase 20.00g cslap-361 into storage
```

Every product gets a short handle: three to five characters from the cultivar,
then the THC/CBD ratio. The ratio is there because the same cultivar from the
same maker at two strengths is two prescriptions — `wcake-221` and `wcake-251`
are legible as a pair, and adding their grams together would be wrong. Pass
`--slug lemon` to choose your own, which is taken as written. A handle is
settled when the product first appears and never changes, because it is the
name every later entry refers to. Products brought in by `wits import` keep a
handle spelled out from their full name (`nice-morning-sun-321`), since years
of records were already written against those names.

```sh
wits grind cslap 0.75
wits sesh cslap-361 0.3 --device volcano --temp 185
wits status
```

References resolve by prefix, so `cslap` is enough while it is unambiguous.
Amounts are grams to two decimal places — what a jeweller's scale reads — with a
comma or a point and an optional `g`; anything finer is refused.
`buy` is the exception: it takes a handle or a full name, and refuses part of
one rather than guess, because a purchase that matches nothing adds a product.

![Recording a fill and a session](./assets/wits-cli.gif)

From here: [the day to day in the terminal](./docs/terminal.md#recording-day-to-day),
[every command](./docs/terminal.md#commands), [importing a
spreadsheet](./docs/terminal.md#bringing-a-spreadsheet-across), or [the
browser](./docs/browser.md).

## Layout

One Go module, several commands, and a web interface beside them. Not a
module per component: the domain is shared, and a boundary between a server and
the ledger it serves would mean versioning the ledger against itself.

```text
wits/                     module github.com/TheDonDope/wits
  cmd/
    wits/                 the terminal interface and the commands
    witsnap/              the camera and the tap: screens as text, the fold as JSON
    wits-server/          the JSON API, and the web interface it serves
    wits-wasm/            the same API, in WebAssembly, for the phone
  pkg/
    journal/              the append-only log
    ledger/               the fold: balances, cycles, statistics
    repo/                 finding and creating a .wits directory
    workspace/            opening a repository and holding its state
    catalog/              products and devices
    record/               applying entries, with the checks that guard them
    bundle/               the portable archive format
    remote/               push and pull through a shared folder
    importer/             reading the tracking spreadsheet
    cannabis/             cannabinoids, terpenes and their boiling points
    research/             a product's terpenes, looked up on the web
    api/                  the repository over HTTP, for wits-server
    tui/                  the screens
    version/              the build stamp every binary reports from
  wits-ui/                the web interface: Angular and Angular Material
  docs/                   the terminal and the browser, written out
```

`pkg/journal` depends on nothing else here and everything above depends downwards
only, so a server is another caller of `pkg/workspace`, not a new layer.

## Built with

- [bubbletea](https://github.com/charmbracelet/bubbletea) — the TUI framework 🏗
- [bubbles](https://github.com/charmbracelet/bubbles) — components 🫧
- [huh](https://github.com/charmbracelet/huh) — forms and prompts 🤷
- [lipgloss](https://github.com/charmbracelet/lipgloss) — layout and style 👄
- [vhs](https://github.com/charmbracelet/vhs) — the recordings above 📼
- [cobra](https://github.com/spf13/cobra) — the command line
- [excelize](https://github.com/xuri/excelize) — reading the spreadsheet

The Charm libraries are all on the v2 line, which lives under `charm.land/…`
rather than `github.com/charmbracelet/…`.

The charts are drawn in-tree. The terminal charting libraries still target Bubble
Tea v1, and mixing the majors puts two renderers and two colour-profile detectors
in one binary, which shows up on screen as inconsistent colour.

## Development

```sh
./doctor.sh        # check the tools; `./doctor.sh install` fetches what is missing
make build         # build ./bin/wits
make run           # build it and run it
make install-wits  # install the working tree as `wits`, replacing a release
make test          # test with coverage
make cover         # coverage as HTML
make vet
make preflight     # everything CI and the Codacy gate will say, said here first
```

Building and testing need only Go. `./doctor.sh` checks the rest of the loop —
`gh` and `git-chglog` for releases, `shellcheck` and lizard for the gate, and
vhs with `ttyd`, `ffmpeg`, the Hack font and a Chromium that can start, for the
recordings — and `install` puts whatever is missing in your home directory,
without `sudo`.

`make build` writes to `./bin/wits`, which is fine for a session in the
checkout and wrong for daily use — the shell keeps finding whatever `wits` is
on the PATH. `make install-wits` installs the working tree where
`go install …@latest` puts a release: `$GOBIN`, or `$GOPATH/bin` when GOBIN is
unset (`go env GOPATH` says which). It goes through the same ldflags as
`make build`, so `wits --version` names the tag and commit it was built from;
a bare `go install ./cmd/wits` skips the stamp and reports
`unknown (built from source)`.

`./preflight.sh watch <pr>` polls a pull request's checks and reads the Codacy
delta the way the gate does, so a red X never comes as a surprise.

`make snap` builds `witsnap`, the camera and the tap: `witsnap screens` renders
any screen as plain text — `--press p,tick,tick` photographs a replay mid-run —
and `witsnap json` writes the whole derived state as JSON, which is the seam a
new client or a new visualisation starts from.

`make build-windows` cross-compiles `bin/wits.exe`. `make render-tapes`
re-records every GIF in `assets/` from the `*.tape` files, and
`./render-vhs-tapes.sh wits-cli.tape` just the ones named — one at a time,
since each render starts a browser. The tapes seed a throwaway repository with
`demo-seed.sh` first, because `wits` reads a `.wits` directory and a source
checkout has none.

## Changelog & Roadmap

Features, bugs and refactorings are tracked in [ROADMAP.md](./ROADMAP.md) rather
than GitHub Issues, so the plan and the code stay in one place. A detailed
changelog is in [CHANGELOG.md](./CHANGELOG.md), and the story of how it got
here in [DEVDIARY.md](./DEVDIARY.md).
