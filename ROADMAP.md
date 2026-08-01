# 🥦 Wits Roadmap 🏗️

Features, bugs and refactorings are tracked here rather than in GitHub Issues, so
that the plan and the code stay in one place. A detailed changelog is in
[CHANGELOG.md](./CHANGELOG.md).

Everything below is checked against the code rather than remembered.

---

## 🧭 The model

Wits is a **ledger**. Nothing it shows is stored; it is all derived by replaying
an append-only journal. History is never rewritten, because the data is a
multi-year medical record: a mistake is corrected by appending a correction, the
way `git revert` adds a commit.

### The four accounts

Grams move between accounts, and every entry is a transfer, so mass is conserved
from the pharmacy through to the ash.

| Entry | From | To | Carries |
| --- | --- | --- | --- |
| `purchase` | — | storage | product, grams |
| `grind` | storage | stash | product, grams |
| `sesh` | stash | consumed | product, grams, device, temperature |
| `avb-collect` | consumed | avb | product, grams **as weighed** |
| `avb-use` | avb | — | grams, purpose |
| `adjust` | any | any | grams, reason — corrections are these |

**Three tins, not one.** The stash is per product, so every gram stays
attributable to a single product end to end. No blends, no proportional guessing.

### Cycles are derived

A cycle runs from one prescription fill to the next. Ending it when storage
reaches zero sounds tidier but does not survive real data: 13 of 47 historical
cycles ended with a remainder between 0.01 g and 4.84 g, and consecutive cycles
overlap because a new sheet was started before the old one finished. See
`ledger.CycleGap`.

### Entries carry two timestamps

**Occurred-at** and **recorded-at**, so a backdated entry is honest about being
late. Both are kept to second precision: nanoseconds say nothing about when
something was ground, and would have to be carried through every export to keep
hashes reproducible.

An entry has **no identifier of its own** — its hash names it, the way a commit
hash names a commit. That is what lets a bundle restore into a journal that
verifies against the one it came from.

---

## ✅ Built

### The repository — `wits init`

`.wits` is created the way `git init` creates `.git`, and every command finds it
by walking up from the working directory. Configuration lives in `config.yml`;
there is no `.env` and no environment to set. The directory is `0700` and its
files `0600`, because it is health data.

### The journal

Newline-delimited JSON, opened `O_APPEND`, never rewritten — so a failed write
can add a bad line but cannot destroy an existing one. Each entry is chained to
the previous by SHA-256, so an edit made outside Wits is detectable.

Appending takes an **advisory file lock** as well as a mutex. It is a
read-then-write — the tip is read to chain onto — and the mutex only holds within
one process. Two writers reading the same tip would append entries claiming the
same predecessor and fork the chain. That matters as soon as anything other than
a single command talks to a repository.

### The fold — `pkg/ledger`

Balances per account and product, cycles, and the figures the spreadsheet used to
maintain by hand: average and median per day, and how long the remainder will
last.

Days-with-an-entry and days-elapsed are reported **separately and labelled**. The
spreadsheet conflated them, because its date column was pre-filled with zero
rows, so its average depended on how far ahead it had been filled in.

### The commands

`init`, `buy`, `grind`, `sesh`, `status`, `log`, `revert`, `device`, `temps`,
`import`, `export`, `bundle`, `restore`. Only the parts of git's vocabulary with
a real referent were borrowed; branching and merging mean nothing for a
prescription and are absent.

Grinding or seshing more than an account holds is refused. The journal would
record it happily, but a negative balance means the log has stopped describing
the tin on the table.

### Bundles — `wits bundle` / `wits restore`

The whole repository as one plain-text file, restoring to a journal identical
**byte for byte**, hash chain included. 1369 entries: 500,729 bytes as a journal,
27,510 as a bundle, 6,299 gzipped.

Plain text and line-oriented on purpose: the record may outlive this program, and
it diffs cleanly in git. Base 36 was tried for the integers and reverted — it
saved a few percent and cost the legibility that justifies a text format at all.

### The spreadsheet importer — `wits import`

Turns each worksheet into a fill and the grinds that followed it. Products are
resolved **by position**, through the bindings in the running-balance formulas,
not by the label in the strain column — those labels are dropdown values that
were not always renamed as products changed.

Nothing is written without `--commit`, and a repository that already holds entries
is refused: a second import would double every gram, and it cannot be told from a
genuine second helping of the same product on the same day.

### The interface

Built on Bubble Tea v2. Four screens — dashboard, journal, analysis, devices —
reading from the ledger and nothing else, so the figures on screen and the
figures in `wits status` are the same figures by construction.

Entries are recorded with huh v2 forms embedded as models. Nothing calls
`form.Run()`, which is what the previous interface did inside `Update` and why it
blocked the event loop.

The charts are drawn in-tree. The terminal charting libraries still target Bubble
Tea v1, and mixing the majors puts two renderers and two colour-profile detectors
in one binary, which shows on screen as inconsistent colour.

### Correcting entries

`wits revert`, and `e` / `d` in the journal view. An entry is undone by moving the
same grams back the way they came and recording that alongside the original.
Undoing is refused if the grams have since moved on, and the confirmation defaults
to keeping the entry.

The log shows what currently stands; `v` reveals the corrections behind it. They
are hidden rather than removed — that is the difference between a record that can
be audited and one that cannot.

### Temperatures and devices

Every cannabinoid and terpene with its boiling point, so a setting on a dial reads
as what it releases, and warns at the 205 °C where benzene starts. A device's
range is checked when it is typed rather than later when a session is refused.

---

## 📌 Planned

### 🔹 AVB tracking

- **Status**: Planned
- The event types and the account exist and the fold handles them; there are no
  commands yet. Wanted: `avb collect` recording what is actually weighed out of a
  device, `avb use` drawing it down, and the observed yield ratio over time.
- Imported history holds no sessions at all, because the spreadsheet only ever
  recorded grinding. Nothing was invented to fill that gap, so the stash balance
  of a product recurring across cycles reads high until it is worked down.

### 🔹 A products screen

- **Status**: Planned
- Devices have one; products do not. Wanted: browsing the catalog, correcting a
  parsed name, and merging two products the slug split or joined.

### 🔹 Splitting products the slug merged

- **Status**: Planned
- `Slugify` drops the THC ratio, so the same cultivar from one manufacturer at two
  potencies becomes one product. The importer reports these rather than doing it
  quietly, but there is no way to split them afterwards.

### 🔹 `wits show` and `wits fsck`

- **Status**: Planned
- `show` for one entry in full. `fsck` to expose `journal.Verify` and check mass
  conservation on the command line — the chain is already verified on restore.

### 🔹 A cached fold

- **Status**: Planned
- `wits init` creates `.wits/index/` and **nothing writes to it**. Replaying the
  journal on every open is milliseconds at 1369 entries, so it does not matter
  yet; a long-lived server changes that. `wits reindex` should rebuild it, and it
  must stay disposable.

### 🔹 `log_level` is dead configuration

- **Status**: Planned
- `config.yml` carries `log_level` and nothing reads it — the same dead setting
  the old `.env` had, moved to a new home. Wire it up or drop it.

### 🔹 Markdown export for publishing

- **Status**: In Progress
- `wits export` writes one Markdown file. For GitHub Pages it needs site
  structure: a page per cycle, an index, and something to link them.

### 🔹 A date range for export

- **Status**: Planned
- Export takes whole cycles or everything. A range would be more use for an
  appointment.

### 🔹 `wits-server` and `wits-ui`

- **Status**: Planned
- A REST API and an Angular interface. `pkg/workspace` is what a request handler
  would open a repository with, and cross-process appends are already safe, so the
  seam exists. The layout has room for both: `cmd/wits-server/` and `wits-ui/`.
- One module rather than one per component, because the domain is shared and a
  boundary between a server and the ledger it serves would mean versioning the
  ledger against itself.

---

## 🩺 Findings in the imported records

The tracking spreadsheet was imported once. These describe the records rather
than the tool, and are worth correcting at the source:

- **`2025-03` is dated a year early.** All 32 of its entries fall in March 2024.
- **`2025-10` has 2.40 g with no product selected**, on 24 and 26 November. The
  spreadsheet never subtracted those grams from anything either, so they are
  missing from that cycle's arithmetic too.
- **`2025-06` lists Ghost Train Haze at 0.01 g** where the other two products are
  10 g. Either a typo or a deliberate "trace left" marker.
- **Two products were named more than one way** and became one, because the slug
  drops the THC ratio: `420 Evolution 22/1 CA MAC: MAC1` with its 25/1 spelling,
  and `All Nations 21/1 Lemon Tartz` with its 25/1.

---

## 📜 Notes

- Status is **Planned**, **In Progress**, or listed under Built.
- Anything under Built is in `v2` and can be read in the code; the sections above
  say why it is the way it is, which the code cannot.
