# Development Diary

## 2026-08-02

### From v0.7.0 to v0.20.0 in one day

Fourteen releases in one sitting, all of them growing the interface out of the ledger. Weighing day became `wits reconcile` run bare — pick the account, tick the jars, read the scale — and then the storage, stash and sessions screens arrived to show what the reconciling is for. The dashboard was dealt as cards, the analysis screen learned braille area charts and a rhythm heatmap, and then the whole thing learned to replay: any screen can start from empty and grow the way the record grew, adjustments riding along silently. The day ended with the Séance, where each event takes the table as a playing card and `x` flips it to the record on its back, jars and tins filling underneath as the replay walks the history. Along the way: a preflight script that says what the CI gate will say first, `witsnap` as the camera and the tap for screenshots and JSON, a fuller terpene set, and a cleanup round for dead code and version wiring. All five tapes were re-recorded against the current behaviour.

- `e69d741` refactor(tui): fold the new charts into chart.go and split the big ones
- `aabdebc` refactor: speak of the stash, not the tin
- `db29fb0` feat(reconcile): weigh a whole account interactively, or one jar in a line
- `5516581` feat(tui): a storage screen with a shelf, a history, and ticked weighing
- `a869ffc` refactor(tui): keep the screen in products.go, whatever the tab says
- `d9dcd1e` build: a preflight that says what the gate will say, first
- `d871fa5` build(preflight): check the scripts too, as the gate taught it to
- `783e821` feat(tui): a stash screen and a sessions screen
- `aab3af8` feat(tui): deal the dashboard as cards
- `29d7679` fix(tui): stop shadowing any, and drop the cycle the stash card never read
- `31d4232` feat(tui): a cover slider for the journal, and product bars on the storage card
- `980d016` feat(tui): a deeper analysis screen
- `fa9ca48` feat(analysis): the ledger replays itself
- `951c772` feat(tui): the storage and stash screens replay too
- `8b393ad` build(witsnap): the camera and the tap
- `f5e2239` feat(cannabis): a fuller terpene set, with where each is found
- `ba3b0e7` chore: a cleanup round — dead code, version wiring, devops, renderings
- `b77fed5` feat(replay): adjustments ride along silently
- `722bec7` ci: skip the coverage uploads where the secrets are withheld
- `534ffaf` feat(seance): summon the ledger as playing cards

### Notes for 2026-08-02

Dependabot rode along with three actions bumps and a golang.org/x/sys bump; the changelog was cut at every release from v0.7.0 through v0.20.0.

## 2026-08-01

### The rebuild: a ledger, not a spreadsheet

The ground-up rebuild landed: Wits is now a ledger, everything derived by replaying an append-only, hash-chained journal, with the roadmap rewritten around that model before the first line. The day laid down the whole stack — journal, repository discovery, the fold in `pkg/ledger`, product and device catalogs, the git-shaped command surface — then rebuilt the interface on Bubble Tea v2 with huh v2 forms embedded as models instead of blocking the event loop. The spreadsheet importer went away in favour of repository bundles, then came back properly, tested against the real four-year workbook rather than fixtures. Corrections, device management, a products screen and reconciling against the scale rounded out the surface, and the module was renamed to `wits` with the monorepo layout settled. The README was rewritten and the demo re-recorded to match.

- `a921d37` docs: rewrite the roadmap around a ledger model
- `05f967b` feat(journal): add an append-only, hash-chained event log
- `c07e03a` feat(repo): add the .wits repository and discovery
- `b62ad65` feat(ledger): derive balances, cycles and statistics from the journal
- `38e83a8` feat(catalog): add product and device catalogs
- `6dad63a` feat(importer): import the tracking spreadsheet into the journal
- `6c159f6` feat(cmd): add the git-shaped command surface
- `eb00771` style: gofmt existing tests
- `cf53493` fix: address Codacy static analysis findings
- `eb3f067` feat!: replace the spreadsheet importer with repository bundles
- `8f055ca` ci: run the Go workflow for v2 as well as main
- `61f1cd8` feat!: rebuild the interface on Bubble Tea v2
- `e1f9fe3` feat(tui): record entries with huh v2 forms
- `8625283` feat: correct entries from the journal, manage devices, refresh the README
- `2933d07` test(tui): cover the devices screen and its forms
- `febbb48` feat: bring back the spreadsheet importer
- `a92e8aa` test: cover the import command and its report
- `a9c1719` refactor: share how a repository is opened, and lock the journal
- `f08d908` test(workspace): cover opening from the working directory
- `41eca80` test(journal): cover a repository that cannot be written to
- `54fe4bc` refactor!: rename the module to wits and settle the monorepo layout
- `c2690cf` refactor(cmd): open the interface the way every other command does
- `c8273b4` docs: rewrite the README, re-record the demo, audit the roadmap
- `669d0de` docs: added historic xlsx import data
- `0cbb689` test(importer): run against the real workbook
- `b94dcb3` feat: add a products screen and reconciling against the scale

### Notes for 2026-08-01

The long quiet between March 2025 and here was 34 commits of dependency bumps — the diary has nothing to say about a version number. Work carried on past midnight; the ratio-in-the-slug fix, the five-clip demo and the carry-over fix are the first hours of 2026-08-02 in the log.

## 2025-03-20

### Integrate more charmbracelet tools

Today we integrated the tool vhs to create videos for the documentation

- `592cb6e` feat: add charmbracelt/vhs to gifs for documentation
- `3e86fdf` docs: add used tech to readme
- `441930d` chore(vhs): update rendered tapes for docs
- `fc32c6a` docs: update changelog for v0.5.0
- `058c7cd` docs: add initial dev diary
- `de1ef5f` feat: add dev-diary script

### Notes for 2025-03-20

No further notes for today

## 2025-03-18

### CI Hardening

Pinned third-party GitHub Actions to specific commit SHAs to ensure deterministic builds and mitigate supply chain risks. Maintained compatibility with existing workflows while improving security posture.

- `c519bfd` chore(ci): pin 3rd party github actions to specific commit

### Notes for 2025-03-18

Nope

## 2025-03-17

### Testing Blitz & Release Automation

Added comprehensive test coverage for TUI components and strain management logic. Implemented Codacy integration for test reporting and quality metrics. Fixed release automation in Makefile, added versioned changelog entries through v0.4.1, and improved CI pipeline reliability. Updated documentation with badges and roadmap progress.

- `87a6465` fix(pkg-tui): wire up home view correctly
- `8075465` fix(pkg-tui): initialize strain editor properly
- `73db715` chore(deps): bump lipgloss to 1.1.0
- `65f5b81` test(pkg-storage): strain store tests
- `a9ae565` test(pkg-service): strain service tests
- `da5a8e3` test(pkg-tui): devices home model
- `56a6a66` test(pkg-tui): home model tests
- `53df9ac` test(pkg-tui): settings home model
- `2c83a30` test(pkg-tui): statistics model
- `7f84de4` test(pkg-tui): menu model tests
- `2c86440` chore(ci): upload coverage to Codacy
- `52c4ee1` docs: add codacy badge
- `aabff4c` chore(ci): fix bug report template
- `8db798a` docs: update roadmap
- `9621bef` chore(ci): upload coverage on build
- `d149c32` docs: fix formatting
- `29e0b52` chore: add release target
- `01e728d` docs: changelog v0.4.0
- `f456e72` fix: repair release target
- `2727ccc` docs: changelog v0.4.1

### Notes for 2025-03-17

Nope

## 2025-03-16

### Core Functionality & Observability

Implemented proper initialization for strain store persistence and TUI data binding. Added structured logging throughout service and storage layers. Integrated Cobra CLI framework and refined Makefile targets. Improved list rendering and menu navigation in TUI.

- `68f653b` fix(pkg-tui): trigger list loading
- `0562b72` feat: integrate cobra commands
- `eed3770` feat(pkg-tui): format strain list
- `5400544` fix(pkg): init strain store
- `63b3bf9` feat(pkg-storage): strain store logging
- `dc369f0` feat(pkg-service): strain service logging
- `05bb7d8` feat(cmd-wits): main cmd logginga

### Notes for 2025-03-16

Nope

## 2025-03-13

### Configuration & TUI Architecture

Added environment variable configuration support and debug logging initialization. Refactored TUI components to use proper Bubble Tea command patterns. Published v0.3.0 changelog and updated roadmap documentation.

- `43314a7` docs: roadmap update
- `4a5afbf` docs: changelog v0.3.0
- `fb098e4` feat: enable env config
- `5137f20` fix(pkg-tui): strain list update
- `ff8ddc3` feat(pkg-tui): tea.Cmd messaging

### Notes for 2025-03-13

Nope

## 2025-03-12

### TUI Foundation

Built core TUI components including home view model, menu navigation, and strain editor. Implemented fullscreen mode and proper application exit handling. Added Windows build target and improved documentation for application usage.

- `9278519` feat(pkg-tui): home view builder
- `432bc97` feat(pkg-tui): home view model
- `1b8ecff` feat(pkg-tui): add appliances
- `99da745` fix(pkg-tui): handle ctrl+c exit
- `0016bb1` feat(cmd-wits): fullscreen mode
- `47a13ae` fix(cmd-wits): command rename

### Notes for 2025-03-12

Nope

## 2025-03-11

### Initial Implementation

Established core architecture with file persistence and initial TUI rendering. Configured logging infrastructure and added basic strain management capabilities. Set up Makefile build system and published initial v0.1.0 changelog.

- `8128889` docs: changelog v0.1.0
- `62d5175` feat: file persistence
- `1b4bc06` fix(pkg-tui): non-emoji cursor
- `6f440f4` refac(cmd): rename tui to wits
- `30b4d4a` refac(pkg-storage): extract dir
- `0016bb1` feat(cmd): fullscreen mode

### Notes for 2025-03-11

Nope
