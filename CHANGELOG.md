<a name="unreleased"></a>
## [Unreleased]


<a name="v0.20.0"></a>
## [v0.20.0] - 2026-08-02
### Chore
- **deps:** bump the actions group across 1 directory with 3 updates

### Feat
- **seance:** summon the ledger as playing cards


<a name="v0.19.0"></a>
## [v0.19.0] - 2026-08-02
### Chore
- **deps:** bump golang.org/x/sys in the gomod-minor group

### Ci
- skip the coverage uploads where the secrets are withheld

### Docs
- update changelog for v0.19.0


<a name="v0.18.0"></a>
## [v0.18.0] - 2026-08-02
### Docs
- update changelog for v0.18.0

### Feat
- **replay:** adjustments ride along silently


<a name="v0.17.0"></a>
## [v0.17.0] - 2026-08-02
### Chore
- a cleanup round — dead code, version wiring, devops, renderings

### Docs
- update changelog for v0.17.0


<a name="v0.16.0"></a>
## [v0.16.0] - 2026-08-02
### Docs
- update changelog for v0.16.0

### Feat
- **cannabis:** a fuller terpene set, with where each is found


<a name="v0.15.0"></a>
## [v0.15.0] - 2026-08-02
### Build
- **witsnap:** the camera and the tap

### Docs
- update changelog for v0.15.0


<a name="v0.14.0"></a>
## [v0.14.0] - 2026-08-02
### Docs
- update changelog for v0.14.0

### Feat
- **tui:** the storage and stash screens replay too


<a name="v0.13.0"></a>
## [v0.13.0] - 2026-08-02
### Docs
- update changelog for v0.13.0

### Feat
- **analysis:** the ledger replays itself


<a name="v0.12.0"></a>
## [v0.12.0] - 2026-08-02
### Docs
- update changelog for v0.12.0

### Feat
- **tui:** a deeper analysis screen


<a name="v0.11.0"></a>
## [v0.11.0] - 2026-08-02
### Docs
- update changelog for v0.11.0

### Feat
- **tui:** a cover slider for the journal, and product bars on the storage card


<a name="v0.10.0"></a>
## [v0.10.0] - 2026-08-02
### Docs
- update changelog for v0.10.0

### Feat
- **tui:** deal the dashboard as cards

### Fix
- **tui:** stop shadowing any, and drop the cycle the stash card never read


<a name="v0.9.0"></a>
## [v0.9.0] - 2026-08-02
### Docs
- update changelog for v0.9.0

### Feat
- **tui:** a stash screen and a sessions screen


<a name="v0.8.0"></a>
## [v0.8.0] - 2026-08-02
### Build
- a preflight that says what the gate will say, first
- **preflight:** check the scripts too, as the gate taught it to

### Docs
- update changelog for v0.8.0

### Feat
- **tui:** a storage screen with a shelf, a history, and ticked weighing

### Refactor
- **tui:** keep the screen in products.go, whatever the tab says


<a name="v0.7.0"></a>
## [v0.7.0] - 2026-08-02
### Build
- finish the move to Bubble Tea v2
- bump go version to 1.26.3

### Chore
- bump go version to 1.26.3
- bump go version to 1.24.2
- make the build files tell the truth
- **deps:** bump github.com/spf13/cobra from 1.9.1 to 1.10.1
- **deps:** bump codecov/codecov-action from 5.4.3 to 5.5.1
- **deps:** bump codacy/codacy-coverage-reporter-action
- **deps:** bump github.com/spf13/cobra from 1.10.1 to 1.10.2
- **deps:** bump codecov/codecov-action from 5.5.1 to 5.5.2
- **deps:** bump actions/checkout from 5 to 6
- **deps:** bump github.com/charmbracelet/bubbletea
- **deps:** bump github.com/charmbracelet/huh from 0.7.0 to 0.8.0
- **deps:** bump github.com/charmbracelet/bubbletea from 1.3.7 to 1.3.9
- **deps:** bump github.com/charmbracelet/huh from 0.8.0 to 1.0.0
- **deps:** bump actions/setup-go from 5 to 6
- **deps:** bump codecov/codecov-action from 5.5.2 to 6.0.0
- **deps:** bump github.com/charmbracelet/bubbletea from 1.3.6 to 1.3.7
- **deps:** bump github.com/stretchr/testify from 1.10.0 to 1.11.1
- **deps:** bump actions/checkout from 4 to 5
- **deps:** bump github.com/charmbracelet/bubbletea from 1.3.5 to 1.3.6
- **deps:** bump codecov/codecov-action from 5.4.2 to 5.4.3
- **deps:** bump github.com/charmbracelet/bubbletea from 1.3.4 to 1.3.5
- **deps:** bump codecov/codecov-action
- **deps:** bump github.com/charmbracelet/huh from 0.6.0 to 0.7.0
- **deps:** bump github.com/charmbracelet/bubbles from 0.20.0 to 0.21.0
- **deps:** bump codecov/codecov-action
- **deps:** bump github.com/charmbracelet/bubbles

### Ci
- run the Go workflow for v2 as well as main

### Docs
- update changelog for v0.7.0
- re-record the demo against the corrected figures
- seed a finished cycle and re-record the demo
- make the templates and comments tell the truth
- bring the README and roadmap up to the current behaviour
- split the demo into five clips and re-record them
- added historic xlsx import data
- rewrite the README, re-record the demo, audit the roadmap
- rewrite the roadmap around a ledger model

### Feat
- correct entries from the journal, manage devices, refresh the README
- add a products screen and reconciling against the scale
- bring back the spreadsheet importer
- short product handles, editing a product, and completion
- **catalog:** add product and device catalogs
- **catalog:** put the THC/CBD ratio at the end of every slug
- **cmd:** add the git-shaped command surface
- **importer:** import the tracking spreadsheet into the journal
- **journal:** add an append-only, hash-chained event log
- **ledger:** derive balances, cycles and statistics from the journal
- **reconcile:** weigh a whole account interactively, or one jar in a line
- **repo:** add the .wits repository and discovery
- **tui:** braille area charts, a rhythm heatmap, and heat-tinted bars
- **tui:** record entries with huh v2 forms

### Fix
- address Codacy static analysis findings
- add importer
- **cli:** say device when a device is missing, and count in the singular
- **ledger:** give a cycle its carry-over, so nothing reads over 100%
- **record:** refuse an amend whole, and name the account it refuses

### Perf
- **journal:** cache the tip, and stop logging every append

### Refactor
- speak of the stash, not the tin
- share how a repository is opened, and lock the journal
- **catalog:** split NewHandle along its seams
- **cmd:** open the interface the way every other command does
- **tui:** fold the new charts into chart.go and split the big ones
- **tui:** one binding per key, one field per meaning

### Style
- gofmt existing tests

### Test
- cover the import command and its report
- **importer:** run against the real workbook
- **journal:** cover a repository that cannot be written to
- **tui:** cover the devices screen and its forms
- **workspace:** cover opening from the working directory

### BREAKING CHANGE

the import path is now github.com/TheDonDope/wits.

pkg/tui is rewritten, and pkg/storage, pkg/service and
cannabis.Strain are removed. The reference tables in pkg/cannabis stay,
since the catalog and the temperature lookups use them.

events no longer have an `id`, and a v1 journal will not
verify against v2 hashes. Migrate by bundling the old repository and
restoring it into a new one, which rebuilds the chain.


<a name="v0.6.0"></a>
## [v0.6.0] - 2025-03-25
### Chore
- update rendered tapes
- set font options for tapes

### Docs
- update changelog for v0.6.0
- render env example source in readme
- add dependecy logos to readme
- add initial dev diary

### Feat
- make the main menu more beautiful
- add tape for dev-diary
- add dev-diary script

### Fix
- use correct font family in vhs tapes
- remove unnecessary variable interpolation


<a name="v0.5.0"></a>
## [v0.5.0] - 2025-03-20
### Chore
- **ci:** pin 3rd party github actions to specific commit
- **vhs:** update rendered tapes for docs

### Docs
- update changelog for v0.5.0
- add used tech to readme

### Feat
- add charmbracelt/vhs to gifs for documentation


<a name="v0.4.1"></a>
## [v0.4.1] - 2025-03-17
### Docs
- update changelog for v0.4.1

### Fix
- repair release target in makefile


<a name="v0.4.0"></a>
## [v0.4.0] - 2025-03-17
### Chore
- add release target to makefile
- clean up makefile
- **ci:** upload test coverage results to codacy
- **ci:** fix bug report template formatting
- **ci:** upload test coverage results on github build
- **deps:** bump github.com/charmbracelet/lipgloss from 1.0.0 to 1.1.0

### Docs
- add changelog for v0.4.0
- fix formatting
- update roadmap
- add codacy badge
- update roadmap
- add changelog for v0.3.0

### Feat
- integrate cobra commands
- configure debug logging
- enable configuration through environment variables
- **cmd-wits:** add logging to main cmd
- **pkg:** add stringer methods for strain and store
- **pkg-service:** add logging to strain service
- **pkg-storage:** add logging to strain store
- **pkg-tui:** format strain list item more nicely

### Fix
- **pkg:** correctly initialize strain store and load from yml file
- **pkg-tui:** initialize strain editor properly
- **pkg-tui:** wire up home view correctly
- **pkg-tui:** trigger list loading from menus
- **pkg-tui:** properly update strain list

### Test
- **pkg-service:** add tests for strain service
- **pkg-storage:** add tests for strain store
- **pkg-tui:** add menu model tests
- **pkg-tui:** add statistics home model tests
- **pkg-tui:** add settings home model tests
- **pkg-tui:** add devices home model tests
- **pkg-tui:** add home model tests


<a name="v0.3.0"></a>
## [v0.3.0] - 2025-03-13
### Chore
- **build:** add windows build target to makefile

### Docs
- add roadmap and update readme
- update application run instructions
- add changelog for v0.2.0

### Feat
- **cmd-wits:** run wits in fullscreen
- **pkg-tui:** wire up strain add action
- **pkg-tui:** separate side effects into tea.Cmds
- **pkg-tui:** add mnemonics for appliance actions
- **pkg-tui:** render appliance titles
- **pkg-tui:** render mnemonics with marked text on menu items
- **pkg-tui:** add appliances
- **pkg-tui:** add home view model
- **pkg-tui:** add home view builder
- **pkg-tui:** sort options for strain editor selects alphabetically

### Fix
- **cmd-wits:** remove wrong ignore and re-add wits command
- **pgk-tui:** update documentations
- **pkg-tui:** initialize appliances properly
- **pkg-tui:** handle ctrl+c program exit
- **pkg-tui:** use non-emoji cursor
- **pkg-tui:** use correct cursor emojis

### Refac
- **cmd-wits:** rename command from tui to wits
- **pkg-storage:** extract wits directory
- **pkg-tui:** clean up strains home model
- **pkg-tui:** use tea.Cmd messaging for side effects
- **pkg-tui:** drop the term appliances and instead use home model
- **pkg-tui:** rename HomeView to HomeModel to closer align to bubbletea terminology
- **pkg-tui:** privatize menu model properties


<a name="v0.2.0"></a>
## [v0.2.0] - 2025-03-11
### Chore
- **deps:** bump go version to v1.24.1

### Docs
- add changelog for v0.1.0

### Feat
- add file persistance


<a name="v0.1.0"></a>
## v0.1.0 - 2025-03-11
### Feat
- initial commit


[Unreleased]: https://github.com/TheDonDope/wits/compare/v0.20.0...HEAD
[v0.20.0]: https://github.com/TheDonDope/wits/compare/v0.19.0...v0.20.0
[v0.19.0]: https://github.com/TheDonDope/wits/compare/v0.18.0...v0.19.0
[v0.18.0]: https://github.com/TheDonDope/wits/compare/v0.17.0...v0.18.0
[v0.17.0]: https://github.com/TheDonDope/wits/compare/v0.16.0...v0.17.0
[v0.16.0]: https://github.com/TheDonDope/wits/compare/v0.15.0...v0.16.0
[v0.15.0]: https://github.com/TheDonDope/wits/compare/v0.14.0...v0.15.0
[v0.14.0]: https://github.com/TheDonDope/wits/compare/v0.13.0...v0.14.0
[v0.13.0]: https://github.com/TheDonDope/wits/compare/v0.12.0...v0.13.0
[v0.12.0]: https://github.com/TheDonDope/wits/compare/v0.11.0...v0.12.0
[v0.11.0]: https://github.com/TheDonDope/wits/compare/v0.10.0...v0.11.0
[v0.10.0]: https://github.com/TheDonDope/wits/compare/v0.9.0...v0.10.0
[v0.9.0]: https://github.com/TheDonDope/wits/compare/v0.8.0...v0.9.0
[v0.8.0]: https://github.com/TheDonDope/wits/compare/v0.7.0...v0.8.0
[v0.7.0]: https://github.com/TheDonDope/wits/compare/v0.6.0...v0.7.0
[v0.6.0]: https://github.com/TheDonDope/wits/compare/v0.5.0...v0.6.0
[v0.5.0]: https://github.com/TheDonDope/wits/compare/v0.4.1...v0.5.0
[v0.4.1]: https://github.com/TheDonDope/wits/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/TheDonDope/wits/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/TheDonDope/wits/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/TheDonDope/wits/compare/v0.1.0...v0.2.0
