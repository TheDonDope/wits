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

**A stash per product, not one pool.** The stash is per product, so every gram stays
attributable to a single product end to end. No blends, no proportional guessing.

### Cycles are derived, and they overlap

A cycle opens with a prescription fill — purchases within three days count as
one pickup, see `ledger.CycleGap` — and closes when **its own jars are
empty**, not when the next fill arrives. A remainder belongs to the cycle
that dispensed it: that cycle simply stays open beside its successors until the
last of its grams is ground, reconciled, or cleaned away. In the imported
records, cycles 25, 26 and 27 were all open at once in March 2026.

Every gram in storage stands on the account of one cycle. A jar refilled
before it was empty holds two cycles' grams, and grinds draw the oldest lot
first — no scale can say whose grams leave a mixed jar, so the ledger says
the oldest do, the way inventory leaves a shelf. That is what keeps a
product's "left" at or under 100% with no carry-over arithmetic at all.

### The three scopes

Every figure on screen speaks exactly one of these, and says which:

| Scope | What it counts | Where |
| --- | --- | --- |
| **The fill** | one cycle's own grams: dispensed, remaining, per product | dashboard storage card, `wits status`, `witsnap json`, export |
| **The shelf** | every jar standing today, whoever dispensed it | supply projection, storage screen balances, the "N older jars" lines |
| **The jar** | one product across its lifetime — every fill, every gram | storage screen detail ("of 65 g ever dispensed, over 4 fills"), history table |

The dashboard's "+ 15.82 g in 7 older jars from 6 earlier cycles still open"
is the seam between the first two: the shelf minus the fill, attributed to
the cycles it still belongs to.

### Entries carry two timestamps

**Occurred-at** and **recorded-at**, so a backdated entry is honest about being
late. The fold replays by occurred-at, keeping the recorded order among entries
at the same moment; the journal and its hash chain keep the recorded order, and
bundles are written from the journal, not the fold. Folding in recorded order
put a fill entered a month late into whichever cycle was open when it was typed
in. Recording refuses an entry that would overdraw an account on any date, not
only today — a grind dated before its fill — and a correction is never dated
before what it corrects. Both are kept to second precision: nanoseconds say nothing about when
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

The files rewritten whole — `products.yml`, `devices.yml`, `config.yml` — are
written to a temporary file and renamed into place (`journal.WriteFile`); they
were truncated and then written, so a crash between the two left them empty.
Every change to one goes through `repo.UpdateProducts`, `repo.UpdateDevices`
or `SetTheme`: under the file's lock, the file is read as it stands, changed,
and written back. Changed from a copy read earlier, a rename in the TUI wrote
back a catalog without the product `wits buy` had added in another terminal.

### The journal

Newline-delimited JSON, opened `O_APPEND`, never rewritten — so a failed write
can add a bad line but cannot destroy an existing one. That line, a write cut
off before its newline, is left out when reading and truncated, under the lock,
before the next append; one used to make every command refuse the repository.
A bad line that does end in a newline is still an error. Each entry is chained to
the previous by SHA-256, so an edit made outside Wits is detectable.

Appending takes an **advisory file lock** as well as a mutex. It is a
read-then-write — the tip is read to chain onto — and the mutex only holds within
one process. Two writers reading the same tip would append entries claiming the
same predecessor and fork the chain. That matters as soon as anything other than
a single command talks to a repository.

The guard rails hold under the same lock. `AppendWith` hands a builder the
journal as it stands, read fresh inside the lock, and writes what it returns in
a single write; every recorder entry is built that way, so its checks — enough
in the jar, not already undone, the difference a weighing makes — are checks of
the history, not of the snapshot a screen was drawn from. Checked against the
snapshot, a grind in one terminal and the same grind in the TUI both passed and
left storage at -17.10 g. An amendment's undo and its re-recorded entry are one
write, so nothing can land between them.

The tip is cached between appends, pinned by the file size: a size that no
longer matches means another process appended and the tip is re-read. That is
what keeps a run of plain appends linear instead of re-reading the whole journal
per line.

### The fold — `pkg/ledger`

Balances per account and product, cycles, and the figures the spreadsheet used to
maintain by hand: average and median per day, and how long the remainder will
last.

Days-with-an-entry and days-elapsed are reported **separately and labelled**. The
spreadsheet conflated them, because its date column was pre-filled with zero
rows, so its average depended on how far ahead it had been filled in.

Every gram in storage stands on the account of the cycle that dispensed it,
in per-fill lots drained oldest-first. Grinding down last month's remainder
draws on last month's cycle, not this month's fill; a product's "left" cannot
read over 100%, and a cycle closes itself when its own lots are empty. The
shelf a fill arrived to is still noted (`Carried`, `Opening`), for the
record.

### The commands

`init`, `buy`, `grind`, `sesh`, `status`, `log`, `revert`, `reconcile`, `device add`
and `device list`, `temps`, `import`, `export`, `bundle`, `restore`, `fsck`, and `completion`
for bash, zsh and fish — which offers product handles with what each holds, and
only the ones the command can act on. Only the parts of git's vocabulary with
a real referent were borrowed; branching and merging mean nothing for a
prescription and are absent.

References resolve by prefix, except for `buy`: a purchase that matches nothing
adds a product, so it takes only a handle or a full name. Part of a name is
refused with the products it is part of — "MAC1" used to land on the MAC1+ jar
unannounced, and "Can", part of two names, made a third product. `--slug` says
a new product is meant.

Grinding or seshing more than an account holds is refused. The journal would
record it happily, but a negative balance means the log has stopped describing
the stash on the table.

### Bundles — `wits bundle` / `wits restore`

The whole repository as one plain-text file, restoring to a journal identical
**byte for byte**, hash chain included. 1429 entries: 528,321 bytes as a journal,
29,417 as a bundle, 7,009 with `--gzip`.

Plain text and line-oriented on purpose: the record may outlive this program, and
it diffs cleanly in git. Base 36 was tried for the integers and reverted — it
saved a few percent and cost the legibility that justifies a text format at all.

A restore is one batch through `AppendWith` — one write, all or nothing, the
emptiness check under the lock — and verifies what it wrote. An entry at a time
took 13.04 s on disk for the workbook's bundle; the batch takes 0.07 s.

### Checking — `wits fsck`

The chain is verified **on every open** (`Workspace.Chain`); a broken one does
not stop the repository opening, but `wits status` and the interface say so.
`Verify` checks each hash against the line **as stored**, blanking the hash
field to recover the bytes that were hashed, rather than re-encoding the
decoded event: a field added to `Event` would otherwise have invalidated every
entry ever made. A test pins one entry's encoding and hash, so a change to how
entries encode cannot pass unnoticed.

`wits fsck` reports the chain, every account the replay ever drew below zero,
entries naming a product or device the catalogs lack (one line per name),
corrections of entries that do not exist, and the mass balance added up
straight from the entries — what came in, what went out, what is held. It
changes nothing, and fails when it finds anything.

### The spreadsheet importer — `wits import`

Turns each worksheet into a fill and the grinds that followed it. Products are
resolved **by position**, through the bindings in the running-balance formulas,
not by the label in the strain column — those labels are dropdown values that
were not always renamed as products changed.

Nothing is written without `--commit`, and a repository that already holds entries
is refused: a second import would double every gram, and it cannot be told from a
genuine second helping of the same product on the same day.

Each stage is printed to stderr as it begins, with the time since the start, so
a wait is never silent. The entries are one batch through `AppendWith` — one
write, one flush, checked for emptiness again under the lock. An entry at a
time flushed 1429 times and took 13.2 s on disk in silence; the batch takes
0.25 s, and one bad entry refuses the batch instead of stopping it halfway.

Two spellings of one product at one strength still merge, and that is reported
rather than done quietly. Two strengths of one product never do.

It is tested against the real workbook, committed at
`assets/Tracking.2022.cleaned.xlsx`, rather than only against fixtures — a
fixture agrees with whatever the code believes, and this one does not. Every
figure asserted was reconciled against the workbook read independently.

### The interface

Built on Bubble Tea v2. Seven screens — dashboard, journal, analysis, storage,
stash, sessions, devices — reading from the ledger and nothing else, so
the figures on screen and the figures in `wits status` are the same figures by
construction.

Because everything is derived from an append-only log, any screen can start
from empty and grow the way the record grew: the analysis, storage and stash
views share one replay transport (`p` plays, `←/→` steps, `+/-` retunes;
adjustments ride along silently).

The Séance, a tab that staged the replay as playing cards on a table, was
removed in v0.30.0.

Entries are recorded with huh v2 forms embedded as models. Nothing calls
`form.Run()`, which is what the previous interface did inside `Update` and why it
blocked the event loop.

The charts are drawn in-tree. The terminal charting libraries still target Bubble
Tea v1, and mixing the majors puts two renderers and two colour-profile detectors
in one binary, which shows on screen as inconsistent colour.

Daily amounts are drawn as a braille area chart — two days per cell, four times
the vertical resolution of a block — with a seven-day average over it. The
longer analysis scopes add a calendar heatmap, one cell per day in GitHub's
contribution greens. The dashboard's cycle bar shades from green into the
colour the remaining fraction has earned, and its thirty-day columns tint
heavier days hotter.

Themes are a palette each — twenty editor themes beside wits' own, picked with
`t` and saved as `theme:` in `config.yml`. Every style derives from the
palette, and the accounts borrow each theme's own syntax colours (storage its
blue, AVB its orange, sessions its purple or pink) so a colour keeps its meaning
in every theme. An editor theme sets the terminal's background through the view,
which Bubble Tea restores on exit; the heat ramp climbs from that background to
the theme's green. The forms are dressed in the theme too, through a huh theme
that reads the current one at every frame, since huh themes a field only once
and the picker previews on a form that is already open.

### Correcting entries

`wits revert`, and `e` / `d` in the journal view. An entry is undone by moving the
same grams back the way they came and recording that alongside the original.
Undoing is refused if the grams have since moved on, and the confirmation defaults
to keeping the entry. Sessions can be undone and amended — their grams wait in
`consumed`, which the check once could not see — and a purchase can be amended
upward, since the pharmacy it draws on cannot be overdrawn.

The log shows what currently stands; `v` reveals the corrections behind it. They
are hidden rather than removed — that is the difference between a record that can
be audited and one that cannot.

### Products and reconciliation

A storage screen in two tables: what still holds something — storage, stash,
AVB and grams ground, with the potency and how much of the fill is still held —
and under it the history of every jar weighed down to zero, newest first,
because a catalog remembering four years of prescriptions is a history and a
shelf, and the two read better apart. Names are never abbreviated: the name
column takes what the longest name needs, and the numbers are narrow instead.

Jars can be ticked in either table with space and weighed together on `r`, the
way `wits reconcile` walks an account. `c` cleans the history: a stash whose
storage is long gone and whose sessions were never logged — the shape the
imported years left — is reconciled to zero, recorded as consumed at some
point, which is the only honest reading of an empty jar.

The stash screen drills in the same way: the stashes holding something above —
with everything that ever passed through each, and how many sessions that took
— and the finished ones below, grouped under the day each was consumed. A
refilled stash returns to the active table; the old ending no longer ends the
story. The sessions screen reads the other direction: sessions, grams drawn,
per-session and average-temperature figures, the braille per-day chart, grams
by device with session counts and temperatures, and the rhythm calendar. The
imported years recorded no sessions, so it grows from here.

Every product gets a short handle when it is first bought — three to five
characters from the cultivar, then the THC/CBD ratio: `wcake-221`. Handles are
never one keystroke from another, since references resolve by prefix, and a
prefix is enough while it is unambiguous. `--slug` chooses one by hand and is
taken as written. Imported products keep a handle spelled out from their full
name (`nice-morning-sun-321`), because the imported years refer to them that
way.

**The ratio is part of the slug** because one cultivar from one manufacturer at
two strengths is two prescriptions. Without it the two collapsed into one
product and their grams were added together: these records held two such pairs,
a MAC1 at 22/1 and 25/1, and a Lemon Tartz at 21/1 and 25/1.

A handle is fixed once and never changes: it is the name every entry refers to,
so `e` on the storage screen corrects what a product is *called*, not which
product it *is*.

`wits reconcile`, and `r` in the interface, record the difference between what
the ledger believes an account holds and what it actually weighs. Nothing in the
past is edited: the difference becomes an adjustment, which is a transfer like
any other, so the accounts still balance afterwards.

Run bare, reconcile is interactive: it asks which account is on the scale,
offers the jars as a ticked checklist, and asks for each reading in turn with
the ledger's figure beside the prompt — a blank reading skips a jar. Naming the
account skips the first question, and `wits reconcile stash wcake-221 1.75` is
the whole thing as one line. Nothing is written until every question is
answered, so abandoning the forms halfway records nothing.

### witsnap — the camera and the tap

`witsnap screens` renders any screen as plain text, with scripted key presses
(`--press p,tick,tick` photographs a replay mid-run); `witsnap json` writes the
whole derived state. It drives the same `Snapshot` the tests do, draining Bubble
Tea commands the way the runtime does, so a form renders as it would on screen.
It is how a screen is verified without a terminal, and the seam a new client
starts from.

### Temperatures and devices

Every cannabinoid and terpene with its boiling point, so a setting on a dial reads
as what it releases, and warns at the 205 °C where benzene starts. A device's
range is checked when it is typed rather than later when a session is refused.

---

## 📌 Planned

### 🔹 AVB tracking

- **Status**: Planned
- The event types and the account exist and the fold handles them, and the AVB
  can be weighed (`wits reconcile --avb`, or `r` once it holds anything) — but
  nothing puts grams into it yet. Wanted: `avb collect` recording what is
  actually weighed out of a device, `avb use` drawing it down, and the observed
  yield ratio over time.
- Imported history holds no sessions at all, because the spreadsheet only ever
  recorded grinding. Nothing was invented to fill that gap, so the stash balance
  of a product recurring across cycles reads high until it is worked down.

### 🔹 `wits show`

- **Status**: Planned
- `show` for one entry in full. (`fsck` is built — see Checking above.)

### 🔹 A cached fold

- **Status**: Planned
- `wits init` creates `.wits/index/` and **nothing writes to it**. Replaying the
  journal on every open is milliseconds at 1429 entries, so it does not matter
  yet; a long-lived server changes that. `wits reindex` should rebuild it, and it
  must stay disposable.

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

These describe the records rather than the tool. Everything the importer
flagged — a sheet dated a year early, a 0.01 g fill, grams logged against no
product, leftovers that never reached zero — has since been corrected at the
source, and `wits import` now finds nothing that does not add up.

- **Two cultivars were dispensed at two strengths each** — `420 Evolution CA MAC:
  MAC1` at 22/1 and 25/1, and `All Nations Lemon Tartz` at 21/1 and 25/1. These
  used to become one product apiece, because the slug dropped the ratio. The
  ratio is now the end of every slug and they stand apart, which is why the
  import reported 50 products where it once reported 48.

---

## 📜 Notes

- Status is **Planned**, **In Progress**, or listed under Built.
- Anything under Built is on `main` and can be read in the code; the sections
  above say why it is the way it is, which the code cannot.
