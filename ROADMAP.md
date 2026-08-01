# 🥦 Wits Roadmap 🏗️

Instead of overly complicating the development process with GitHub Issues,
we will keep it simple and list features and planned changes in this file.

## 🚀 Tracking Features and Changes

Each feature or change is tracked with:

- **Status**: Planned | In Progress | Implemented (since `<commitSha or tag>`)
- **Relevant Commits**: `<commitSha1>, <commitSha2>, ...`
- **Description**: A brief explanation of the feature or change.

## Detailed Changelog

A detailed changelog with all commits can be found in the [CHANGELOG.md](./CHANGELOG.md).

---

## 🧭 The Model

Wits is a **ledger**, not a database of current values. Everything the app shows is
derived by replaying an append-only journal of events. This is deliberate: the data
is a multi-year medical record, so history is never rewritten — a mistake is
corrected with a compensating event, exactly like a `git revert`.

### The four accounts

Material moves through four accounts. Every event is a transfer between two of them
(or an inflow from outside), so grams are conserved end to end.

```text
   pharmacy / prescription
        │
        │  purchase
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
                                            │  avb-collect (weighed, not estimated)
                                            ▼
                                      ┌───────────┐      avb-use
                                      │    AVB    │ ─────────────▶ edibles / tincture
                                      └───────────┘
```

**Three tins, not one.** The stash is kept per product, so every gram stays
attributable to a single product from purchase through to AVB. No blend maths,
no proportional estimates.

A **session** (`sesh`) is the act of drawing ground product out of a tin and
putting it through a device. Its residue becomes AVB once weighed.

### Event types

| Event         | From      | To        | Carries                                  |
| ------------- | --------- | --------- | ---------------------------------------- |
| `purchase`    | —         | storage   | product, grams, prescription reference   |
| `grind`       | storage   | stash     | product, grams                           |
| `sesh`        | stash     | consumed  | product, grams, device, temperature      |
| `avb-collect` | consumed  | avb       | product, grams **as weighed**            |
| `avb-use`     | avb       | —         | grams, purpose                           |
| `adjust`      | any       | any       | grams, reason (spill, scale correction)  |

Events are immutable and carry both an **occurred-at** and a **recorded-at**
timestamp, so a forgotten evening can be logged the next morning with `--date`
without lying about when it was entered.

### Cycles are derived, never stored

A cycle runs from one prescription fill to the next. It is not a calendar month,
which removes three problems the spreadsheet had: months with two prescriptions,
months with none, and cycles that run longer than 30 days.

Ending a cycle when storage reaches zero sounds tidier and usually amounts to the
same thing, but it does not survive the real data — see `ledger.CycleGap`. Cycles
routinely overlap, because a new sheet gets started before the old one is finished.

---

## 📌 Planned Features

### 🔹 `wits init` and the `.wits` repository

- **Status**: Implemented (unreleased)
- **Description**: `wits init .` creates a `.wits` directory the way `git init .`
  creates `.git`. This replaces the current hard dependency on a `.env` file in the
  working directory.
- **Layout**:

  ```text
  .wits/
    config.yml      # settings; replaces .env
    products.yml    # the catalog
    devices.yml     # vaporizers and their temperature ranges
    journal.ndjson  # append-only, one event per line, never rewritten
    index/          # cached fold — disposable, rebuilt by `wits reindex`
  ```

- **Tasks**:
  - [x] `wits init` with repository discovery from the current directory upwards
  - [x] `config.yml` replacing `.env` and `godotenv`
  - [x] `0700` on the directory and `0600` on its files — this is health data
- **Relevant Commits**: tbd

### 🔹 Append-only journal

- **Status**: Implemented (unreleased)
- **Description**: NDJSON, one event per line, opened `O_APPEND`. Appending cannot
  clobber existing data, which structurally removes the whole-file-rewrite data-loss
  path in the current `StrainStoreYMLFile`.
- **Tasks**:
  - [x] Event types and their (de)serialisation
  - [x] Append-only writer, streaming reader
  - [x] Hash chaining so tampering and truncation are detectable
  - [ ] `wits fsck` to expose `Verify` on the command line
- **Relevant Commits**: tbd

### 🔹 Fold engine

- **Status**: Implemented (unreleased)
- **Description**: Replay the journal to derive everything the spreadsheet computes
  by hand: per-product balances in each account, remaining percentage, therapy days,
  average and median per day, and estimated days of supply left.
- **Note**: The spreadsheet's `# therapy days` counts *elapsed* days because the date
  column is pre-filled with zero rows. Wits should report both, and label them:
  days elapsed vs. days with actual consumption.
- **Tasks**:
  - [x] Balance fold per account and product
  - [x] Cycle detection (fill to fill — see `ledger.CycleGap` for why not "to zero")
  - [x] Statistics: avg/day, median/day, estimated days remaining
  - [ ] Disposable index cache with `wits reindex` (1986 events fold in ~0.2s, so not yet needed)
- **Relevant Commits**: tbd

### 🔹 Git-shaped command surface

- **Status**: Implemented (unreleased)
- **Description**: Take the parts of git's vocabulary that have a real referent here
  and stop. Branches and merges do not map onto this domain and will not be forced.

  ```text
  wits init .
  wits buy "Enua 22/1 Wedding Cake" 20g
  wits grind wedding-cake 0.75 [--date 2026-07-29]
  wits sesh wedding-cake 0.3 --device volcano --temp 185
  wits status
  wits log [--oneline] [--product X] [--cycle current]
  wits show <ref>
  wits revert <ref>
  wits export --format markdown
  wits bundle --out history.wits
  wits restore history.wits
  ```

- **Tasks**:
  - [x] `init`, `buy`, `grind`, `status`, `log`
  - [x] `revert` (compensating event, never a rewrite)
  - [ ] `show`
  - [x] `sesh` with a device and a temperature
  - [ ] Short aliases per product for fast daily entry
- **Relevant Commits**: tbd

### 🔹 Markdown export

- **Status**: Implemented (unreleased)
- **Description**: Export the journal to plain Markdown, so the record is readable
  and portable without Wits — printable for a doctor's appointment, diffable in git,
  and durable if the app is ever abandoned.
- **Tasks**:
  - [x] `wits export --format markdown` for a cycle or all cycles
  - [x] Summary header table plus the event log
  - [ ] A date range rather than whole cycles
  - [ ] JSON/CSV through the same interface
- **Relevant Commits**: tbd

### 🔹 Spreadsheet importer

- **Status**: Implemented (unreleased)
- **Description**: Reads the tracking spreadsheet Wits grew out of, turning each
  worksheet into a prescription fill and the daily grinds that followed it.
- **Products are resolved by position, not by name.** The strain column holds
  dropdown values that were not always renamed as products changed, so in later
  sheets they are stale — a sheet headed "Ice Cream Cake" still says "WW" in every
  row. Each running-balance column binds a label to a header row by formula
  (`=IF(B6="WW",B1-C6,B1)`), so reading that binding inherits the spreadsheet's own
  arithmetic rather than second-guessing it, and it is self-checking.
- **Nothing is written without `--commit`,** and a repository that already holds
  entries is refused: importing the same workbook twice would double every gram in
  it, and a re-import cannot be told from a genuine second helping of the same
  product on the same day.
- **Tasks**:
  - [x] Fills and grinds, with the header row detected per sheet
  - [x] Bindings read from the balance formulas
  - [x] Dry run by default, reporting anomalies and merged product names
  - [ ] An interactive pass to split products the slug merged

### 🔹 Repository bundles

- **Status**: Implemented (unreleased)
- **Description**: `wits bundle` writes the catalogs and every event to a single
  file, and `wits restore` reads it back into an empty repository. It is to a Wits
  repository what `git bundle` is to a git one: a portable copy of the history for
  backup, for moving to another machine, or for keeping beside the Markdown export
  in a published repository.
- **Round trip**: restoring reproduces the journal exactly, hash chain included,
  which is what makes a bundle worth trusting as a backup. This is why the event
  schema has no identifier of its own — the hash names the event, as a commit hash
  names a commit — and why timestamps are kept to second precision.
- **Format**: plain text, line oriented. The journal is a medical record that may
  outlive this program, so an archive of it should be legible with nothing but a
  text editor, and should diff cleanly in git. It is small regardless, because
  most of what the journal stores is derivable and is therefore left out:

  | | bytes | |
  | --- | ---: | --- |
  | journal (1986 events, 4 years) | 822,472 | |
  | `xz -9` of the journal | 140,560 | 5.9× |
  | **bundle** | **37,650** | **21×** |
  | bundle, gzipped | 8,159 | 100× |

- **Tasks**:
  - [x] Compact text encoding with dictionaries for products, devices and notes
  - [x] Deltas for timestamps and amounts, with zone offsets carried explicitly
  - [x] Exact round trip, proven byte for byte against four years of real history
  - [x] `--gzip` for transport
  - [ ] Merge on restore, if two machines ever need reconciling
- **Relevant Commits**: tbd

### 🔹 Devices

- **Status**: Implemented (unreleased)
- **Description**: Vaporizers as first-class entities so `consume` can record which
  device and which temperature. This is what makes the existing cannabinoid and
  terpene boiling-point tables in `pkg/cannabis` actionable rather than decorative.
- **Tasks**:
  - [x] Device catalog in `devices.yml`, via `wits device add`
  - [x] Temperature on session events, bounded by the device's own maximum
  - [x] `wits temps <celsius>` reports what a setting releases, and warns at 205 °C
- **Relevant Commits**: tbd

### 🔹 AVB tracking

- **Status**: Planned
- **Description**: Already-vaped bud is decarbed and reusable. Track it as a real
  account fed by weighed `avb-collect` events, and drawn down by `avb-use`. The
  event types and the account exist; the commands do not yet.
- **Tasks**:
  - [ ] `avb-collect` / `avb-use`
  - [ ] Per-product AVB balances
  - [ ] Observed yield ratio (input grams vs. collected grams) over time
- **Relevant Commits**: tbd

### 🔹 The interface

- **Status**: Implemented (unreleased)
- **Description**: Rebuilt on Bubble Tea v2 as a reader over the ledger, replacing
  the interface that sat on the storage and service layers. Three screens: a
  dashboard of what is left and how long it will last, the journal, and an
  analysis view scoping from the current cycle out to all four years. Entries are
  made with huh v2 forms embedded as models — never by calling `Run`, which is what
  the previous version did and why it blocked the event loop.
- **Tasks**:
  - [x] Dashboard, journal and analysis screens
  - [x] Charts drawn in-tree, sharing one colour per account with the log
  - [x] Grind, session and fill forms, applying the same checks as the commands
  - [x] Devices screen, with add, edit and remove
  - [x] Amending and undoing an entry from the journal view
  - [ ] Products screen
- **Relevant Commits**: tbd

### 🔹 Statistics with inline plots

- **Status**: Implemented (unreleased)
- **Description**: Charts are drawn in-tree rather than pulled in.
  [NimbleMarkets/ntcharts](https://github.com/NimbleMarkets/ntcharts) is still on
  the Bubble Tea v1 line, and mixing the majors would put two renderers and two
  colour-profile detectors in one binary, which shows up on screen as inconsistent
  colour. The README still lists it and should be corrected.
- **Tasks**:
  - [x] Gauge, column chart, stacked bar and sparkline primitives
  - [x] Grams per day across a cycle, a year, or the whole history
  - [x] Per-product and per-cycle breakdowns
- **Relevant Commits**: tbd

---

## 🔧 Improvements & Refactoring

### 🔹 File permissions on health data

- **Status**: Planned
- **Description**: `strains.yml` is written `0644` and the `.wits` directory is
  created with `os.ModePerm` (0777). A multi-year medical record should be `0600`
  inside a `0700` directory.
- **Relevant Commits**: tbd

### 🔹 Superseded by the ledger design

These earlier roadmap items are kept for traceability but are resolved by the
model above rather than fixed in place:

- *Persistent Local Storage for Strains* — becomes the append-only journal.
  The YAML store's whole-file rewrite on every add, and its silent fallback to an
  empty store when the file cannot be read or parsed, are removed rather than
  patched.
- *Refactor `strain_store.go`* — the in-memory and YAML implementations are ~95%
  duplicated. Both go away; there is one journal with a disposable index.
- *Reading/Writing of Configuration & Settings* — becomes `.wits/config.yml`,
  written by `wits init`.
- *`Strain.Amount`* — deleted. An amount is a fold over the journal, not a field on
  a product. A product is catalog data that outlives any single purchase.
- *`LOG_LEVEL`* — documented in the README and `.env.example` but never read by any
  code. Either wire it up in `config.yml` or drop it.

---

### 🔹 Toward a server and a web interface

- **Status**: Planned
- **Description**: A `wits-server` exposing a REST API, and a `wits-ui` in Angular.
  The domain packages are already shared by the commands and the interface, and
  `pkg/workspace` is what a request handler would open a repository with, so the
  seam exists. Two things do not yet:
  - **Concurrency.** Appending is a read-then-write: the tip is read to chain
    onto. That is now guarded by an advisory file lock as well as a mutex, so two
    processes cannot fork the chain, but a long-lived server should also cache the
    fold rather than replay the journal per request.
  - **Layout.** Done: the module is `github.com/TheDonDope/wits`, with `cmd/wits`
    today and room for `cmd/wits-server` and `wits-ui/` beside it. One module
    rather than one per component, because the domain is shared and a boundary
    between a server and the ledger it serves would mean versioning the ledger
    against itself.

## 📜 Notes

- Changes will be updated as development progresses.
- Feature status will be marked as **Implemented** once merged into `main`.
- Commits are referenced for traceability and rollback if needed.

---

## 🩺 Findings from the one-off spreadsheet import

The tracking spreadsheet was imported once, in v1, and the importer has since
been removed: the history now lives in the journal, which is the only thing that
needs reading. These are the things the import turned up in the source data,
kept because they describe the imported records rather than the tool:

- **`2025-03` is dated a year early.** All 32 of its entries fall in March 2024.
- **`2025-10` has 2.40 g with no product selected**, on 24 and 26 November, which
  the spreadsheet never subtracted from anything either. Those grams are missing
  from the imported history for the same reason.
- **`2025-06` lists Ghost Train Haze at 0.01 g** against 10 g for the others.
- **`cantourage-mac-1` and `cantourage-mac1` are the same product**, spelled
  "MAC 1+" in one sheet header and "MAC1+" in another.

Imported history holds no `sesh` events, because the spreadsheet only recorded
grinding. Nothing was invented to fill that gap, so the stash balance of a
product that recurs across cycles reads high until it is worked down. Past
sessions can still be entered by hand with `wits sesh --date`.
