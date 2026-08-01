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
  ```

- **Tasks**:
  - [x] `init`, `buy`, `grind`, `status`, `log`
  - [ ] `show`, `revert` (compensating event, never a rewrite)
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
- **Description**: One-time backfill of `Tracking 2022.xlsx` — 47 worksheets,
  2022-04 through 2026-07, **1503.93 g across 47 cycles**, imported as 1986
  events. Without it, meaningful statistics are years away.
- **Known data quirks**:
  - Product headers follow `{Manufacturer} {THC}/{CBD} {Cultivar} (g)` most of the
    time, but not reliably: the cultivar sometimes precedes the ratio, product lines
    are inlined (`Cannamedical Hybrid Ultra DK 28/1 …`), and there are loose codes
    (`CA`, `DNK`, `PT Ku.`, `DAB`, `T01`, `GM`).
  - The 2022 sheets are in German (`Liefermenge`, `Datum`, `Sorte`) and track by
    genetic type, not by product — there is no product identity to recover there.
  - Row codes froze as `WW` / `FLA` / `MAC1+` in 2025-01 and no longer match the
    products in the sheet headers. They are positional slots, not identities.
- **Tasks**:
  - [x] Parse worksheets into purchase and grind events
  - [x] Resolve products from the balance-column formulas, not the stale dropdown labels
  - [x] Dry-run by default, with a report of everything that does not add up
  - [ ] Interactive review of the imported products, to merge near-duplicates
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

### 🔹 Statistics with inline plots

- **Status**: Planned
- **Description**: A table view first, then inline charts. The README already lists
  [NimbleMarkets/ntcharts](https://github.com/NimbleMarkets/ntcharts) as a used
  technology, but it is not yet a dependency — this is where it earns its place.
- **Tasks**:
  - [ ] `bubbles/table` for the status and log views
  - [ ] Time series of grams per day with a cycle-average line
  - [ ] Per-product and per-cycle breakdowns
- **Relevant Commits**: tbd

---

## 🔧 Improvements & Refactoring

### 🔹 Navigation loses the parent model (bug)

- **Status**: Planned
- **Description**: `HomeModel.Update` returns `hm.listView.Update(msg)`, which makes
  the inner list the new top-level model. Selecting Strains from the main menu fires
  `onStrainsListed()` immediately, so `StrainsHomeModel` — and with it the only
  `StrainService` — is discarded before the user can press anything. Every strain
  added through the normal path is parsed and then silently dropped, and the header
  disappears.
- **Tasks**:
  - [ ] Have the parent keep ownership and forward messages to its children
  - [ ] Regression test that drives menu → strains → add as a real key sequence
- **Relevant Commits**: tbd

### 🔹 Swallowed store errors (bug)

- **Status**: Planned
- **Description**: `shm.service.AddStrain(msg.strain)` ignores its error, so a
  duplicate silently does nothing and the user is not told why.
- **Relevant Commits**: tbd

### 🔹 Form blocks the event loop (bug)

- **Status**: Planned
- **Description**: `onStrainAdded()` calls `form.Run()` synchronously inside
  `Update`, starting a second Bubble Tea program while the outer one owns the
  terminal. The form should be embedded as a model in its parent.
- **Relevant Commits**: tbd

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

## 📜 Notes

- Changes will be updated as development progresses.
- Feature status will be marked as **Implemented** once merged into `main`.
- Commits are referenced for traceability and rollback if needed.

---

## 🩺 Findings from the first import

Running the importer over `Tracking 2022.xlsx` surfaced things in the source
data that are worth correcting at the source rather than in Wits:

- **`2025-03` is dated a year early.** All 32 of its entries fall in March 2024.
- **`2025-10` has 2.40 g with no product selected**, on 24 and 26 November. The
  spreadsheet never subtracted those grams from anything, so they are missing
  from that cycle's arithmetic too.
- **`2025-06` lists Ghost Train Haze at 0.01 g**, where the other two products
  are 10 g. Either a typo for 10 g or a deliberate "trace left" marker.
- **`cantourage-mac-1` and `cantourage-mac1` are the same product**, spelled
  "MAC 1+" in one header and "MAC1+" in another.

Imported history holds no `sesh` events, because the spreadsheet never recorded
consumption — only grinding. Nothing is invented to fill that gap, so the stash
balance of any product that recurs across cycles reads high until it is worked
down. Past sessions can be entered by hand with `wits sesh --date`.
