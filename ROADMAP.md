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

The scopes are **types** in the ledger — `Fill`, `Shelf`, `Jar` — so a figure
mixing them does not compile, and every deliberate crossing is a conversion.
Typing them found three mixes the eye had passed:

- A fill's row on the dashboard and in `wits status` stacked the fill's sealed
  grams with the product's whole stash and AVB, which hold grams of every fill
  of it: on the demo data a fill 100% sealed showed 12.35 g in its stash, and
  its bar ran full past its end. `State.FillGround` caps the stash and AVB to
  what the fill has had ground, which on a shelf of one-fill products is the
  jars' figures exactly.
- `Cycle.Ground` counted every grind during a fill's tenure, from whichever
  jar — the shelf's grinding over the period, not the fill's. It is `Shelf`.
- "Lasts ~19 days" divided the fill's remaining grams by the rate, as if they
  were next — but grams drain oldest first, and the shelf's projection under it
  counted the older jars too. The fill's figure is now what it is worth: "19
  days' worth" (`Stats.DaysWorth`; witsnap's `days_worth`).

### Entries carry two timestamps

**Occurred-at** and **recorded-at**, so a backdated entry is honest about being
late. The fold replays by occurred-at, keeping the recorded order among entries
at the same moment; the journal and its hash chain keep the recorded order, and
bundles are written from the journal, not the fold. Folding in recorded order
put a fill entered a month late into whichever cycle was open when it was typed
in. Recording refuses an entry whose grams were not in the jar at its moment,
on any date, not only today — a grind dated before its fill — and a correction
is never dated before what it corrects.

What is true **now** is a replay of the entries dated by now (`ledger.FoldAt`);
the rest are upcoming. A fill dated ahead used to count from the day it was
typed in: storage read 2 g more than the jar held, and weighing it honestly
recorded a 2 g loss, leaving the ledger 2 g short once the fill arrived. Every
entry is still in `State.Events`, so the journal lists the future ones —
marked upcoming — and they can still be undone; today's-balance checks judge
only entries dated by today, and `Shortfalls` judges every entry on its date. Grams are flower, not money: an account
is never below zero, not even in the past, so there is no overdraft to record —
`ledger.Shortfalls` names an entry that takes more than was there, following
each jar down to empty and no further. Both are kept to second precision: nanoseconds say nothing about when
something was ground, and would have to be carried through every export to keep
hashes reproducible.

An entry has **no identifier of its own** — its hash names it, the way a commit
hash names a commit. That is what lets a bundle restore into a journal that
verifies against the one it came from.

---

## ✅ Built

### The repository — `wits init`

`.wits` is created the way `git init` creates `.git`, and every command finds it
by walking up from the working directory. Configuration lives in `config.yml` —
only the format version and the theme; `log_file` named a `wits.log` that every
launch opened and nothing wrote to, and is gone;
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

Amounts are **centigrams** wherever they are added up (`journal.Centigrams`):
balances, the per-fill lots, each cycle's share and daily totals are integer
sums, so a total is exact whatever order it is added in, not exact because
every path remembered to round. The journal still stores grams as a decimal —
the encoding every hash is over — and the journal refuses an amount finer than
a centigram: 0.333 g used to be stored as typed, shown as 0.33 g everywhere,
and bundled as 0.33 g, so its restore was not the original. Every amount typed
into a command or a form is read by one parser, `ledger.ParseGrams`, where
there were seven copies; `wits fsck` reports any finer amount an older wits
recorded.

Every gram in storage stands on the account of the cycle that dispensed it,
in per-fill lots drained oldest-first. Grinding down last month's remainder
draws on last month's cycle, not this month's fill; a product's "left" cannot
read over 100%, and a cycle closes itself when its own lots are empty. The
shelf a fill arrived to is still noted (`Carried`, `Opening`), for the
record.

### The commands

`init`, `buy`, `grind`, `sesh`, `rate`, `status`, `log`, `show`, `revert`, `reconcile`,
`device add` and `device list`, `temps`, `import`, `export`, `bundle`, `restore`, `fsck`,
and `completion` for bash, zsh and fish — which offers product handles with what each holds, and
only the ones the command can act on. Only the parts of git's vocabulary with
a real referent were borrowed; branching and merging mean nothing for a
prescription and are absent.

References resolve by prefix, except for `buy`: a purchase that matches nothing
adds a product, so it takes only a handle or a full name. Part of a name is
refused with the products it is part of — "MAC1" used to land on the MAC1+ jar
unannounced, and "Can", part of two names, made a third product. `--slug` says
a new product is meant.

`show` is one entry in full, the way `git show` is a commit: its place in the
chain, when it happened and how long after it was written down, what it moved
and what the jar held before and after — labelled the jar, since `status` speaks
for the fill — the fills it fell in, what undoes it or what it undoes, and a
session's standing rating. With no entry named, the one written last.

Grinding or seshing more than an account holds is refused. The journal would
record it happily, but grams that are not in the jar cannot come out of it, and
a log that says they did has stopped describing the stash on the table.

### Bundles — `wits bundle` / `wits restore`

The whole repository as one plain-text file, restoring to a journal identical
**byte for byte**, hash chain included. 1429 entries: 528,321 bytes as a journal,
29,417 as a bundle, 6,693 with `--gzip`.

Plain text and line-oriented on purpose: the record may outlive this program, and
it diffs cleanly in git. Base 36 was tried for the integers and reverted — it
saved a few percent and cost the legibility that justifies a text format at all.

Format 2 writes an entry's accounts (`f=stash>storage`) when its type does not
imply them — an adjustment's never do. Format 1 took every entry's accounts to
follow from its type, so every undo and weighing came back from a restore as
external to external, moving nothing: the wrong balances, and a journal that
was not the original, though the restore reported success. The workbook has no
adjustments, so no test and no recording showed it. A format 1 bundle is still
read: an undo's accounts are the entry it names, reversed — the chain is
rebuilt while reading to find it — and a weighing's or a clean-up's are read
from the note wits wrote for it. An adjustment whose note does not say them
refuses the restore, naming it, rather than restoring it wrong.

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

`wits fsck` reports the chain, entries that took grams that were not in the
jar at the time (`Shortfalls`, said as what the jar held — "grind 5.00 g of
wcake-221 took more than storage held — 1.00 g"; the recorder refuses these,
but an import, a restore or a hand edit does not go through it), entries naming a product or device the catalogs lack (one line per name),
corrections of entries that do not exist, and the mass balance added up
straight from the entries — what came in, what went out, what is in the jars
and what has gone through a vaporizer (it called all of that "held"). It
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

The dry run's report includes any entry that takes grams no row before it put
in the jar, across the whole workbook — what one sheet at a time cannot see. A
restore gives back what the bundle holds, and warns if it holds such an entry.

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

The journal draws only the rows in its window: styling all 1429 of the
workbook's entries took 34 ms a frame — every second, and every key held down —
and now takes 2.3 ms. The window follows the selected row's line rather than its
index; a blank line before each day had put the selection further below the
window the further down it went, off screen after twenty rows. The storage and
stash screens share one list (`markList`) for moving, ticking and playback,
where their Update was the same 41 lines twice.

The snapshot follows the repository. Each second's clock tick stats the journal
and the catalogs, and reloads when another process has written to them or the
day has turned; until then the screen showed the repository as it was when it
opened, with "today" frozen at launch. The cycle in progress is the one at the
frame's moment (`ledger.State.CycleAt`): a fill dated ahead is not in progress
until it arrives, where it used to put "day -1529" on the dashboard. `esc` no
longer quits — it closes forms, and one press too many closed the interface.

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
upward, since the pharmacy it draws on is not a jar.

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
imported years recorded no sessions, so it grows from here. The newest eight
sessions that stand are listed with their ratings, and `e` rates the selected
one in a form drawn a line a dimension — huh's three lines a field would push
the note and the enter off a 30-row terminal. A re-rating opens on the standing
one. Each product's mean overall rating is drawn under the list, with its count.

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

### In a browser — `wits-server` and `wits-ui`

`cmd/wits-server` serves `pkg/api`: a JSON API another caller of
`pkg/workspace`, nothing more. Every request opens the repository fresh, and
every write — buy, grind, sesh, weigh, undo — goes through the recorder under
the journal's lock, so the browser, the terminal and the commands refuse the
same things in the same words. The figures keep their scopes in their names:
the fill's `dispensed` and `remaining`, the shelf's projection
(`shelf_projection`), the jar's lifetime. It answers this machine only: a
non-loopback listen address is refused, a request naming another host gets 403
(DNS rebinding), and a write that is not JSON gets 415 (a cross-origin form).

`wits-ui` is Angular 22 with Angular Material, zoneless and signal-driven, its
charts drawn as SVG with d3's geometry: the fill as a ring with the older jars
as a halo, each product as a jar of glass with a moving surface, the shelf's
supply as a curve to its empty day, the whole history as a calendar a year a
row, every fill as a lane on one timeline, and a virtual-scrolled journal. It
polls for entries made elsewhere. Its fonts and icons ship with it — the
scaffolding fetched them from a font service, which would have been the only
network call wits made. Reduced motion is honoured throughout, the route
cross-fade included.

The analysis tab and the replay came across from the terminal: the same five
scopes and the same transport (`Replay`, the terminal's `player` — adjustments
ride along silently, stepping passes over them, the end settles to live), with a
scrubber and `?at=` for a day added. Replayed figures are folded in the browser
(`data/derive.ts`, in centigrams, mirroring `ledger.Summarise` and the fold's
balances) so a frame never waits on the server. The jars are ticked and their
stashes cleared together through `Recorder.Flush` — one batch, all or none.
Every key is declared once in `Keys`, page layers over the shell's, and the
footer is drawn from the same list, so it can only show keys that answer.

### The effect log — `wits rate`

What a session did, rated afterwards from 1 to 5 on any of overall, pain,
sleep, calm, mood, focus and appetite. It is kept beside the journal in
`effects.ndjson`, not in it: the journal's every entry is a transfer and a
rating moves no grams. It keeps the journal's other disciplines — append-only,
written under a lock, hash-chained and verified over the stored bytes, a line
cut short left out. A rating names its session by hash; rating it again
supersedes the earlier rating, and the log keeps both. Only a session that
stands can be rated. `fsck` checks the log's chain and that every rating rates
a session the journal holds; bundle format 3 carries the log after the events,
behind `==`, and a restore gives it back byte for byte.

`effects.Analyse` groups each session's standing rating by product, by
ten-degree temperature band and by part of the day, leaving undone sessions
out; every mean carries its count. In the browser, `/effects` rates the waiting
sessions with a tap a dimension and draws the three groupings: a matrix, bars,
and a wheel of the day. The terminal interface rates from its sessions screen.
The interface's change stamp includes `effects.ndjson`, so a rating made
elsewhere shows within a tick.

---

## 📌 Planned

In this order: each phase stands on the ones before it. AVB tracking and prices
both change the journal and the bundle, so they land together, in one format.
The report waits for weeks of real ratings, which is also what the tolerance
curve needs. The phone comes after sync, because the server answers only this
machine: a phone keeps its own copy of the journal and syncs it, rather than
the server opening up. Timestamping a chain and syncing over WebDAV are the
first things wits would send anywhere, so both are opt-in and said plainly.

### 1 · What is open, and what is quick

#### 🔹 Markdown export for publishing

- **Status**: In Progress
- `wits export` writes one Markdown file. For GitHub Pages it needs site
  structure: a page per cycle, an index, and something to link them.

#### 🔹 A date range for export

- **Status**: Planned
- Export takes whole cycles or everything. A range would be more use for an
  appointment.

#### 🔹 A refill planner

- **Status**: Planned
- The shelf's projection, less the pharmacy's lead time: "order the next
  prescription by 9 November", with an `.ics` reminder to put in a calendar.

### 2 · The physical record, complete

#### 🔹 AVB tracking

- **Status**: Planned
- The event types and the account exist and the fold handles them, and the AVB
  can be weighed (`wits reconcile --avb`, or `r` once it holds anything) — but
  nothing puts grams into it yet. Wanted: `avb collect` recording what is
  actually weighed out of a device, `avb use` drawing it down, and the observed
  yield ratio over time.
- Imported history holds no sessions at all, because the spreadsheet only ever
  recorded grinding. Nothing was invented to fill that gap, so the stash balance
  of a product recurring across cycles reads high until it is worked down.

#### 🔹 What it costs

- **Status**: Planned
- The price paid per fill: euros per gram, per month, and — with the effect log
  — per session that helped.
- A price given when buying (`wits buy … --price`), or afterwards. A purchase is
  never edited in place, so a price given later is recorded beside the journal
  the way a rating is — append-only, chained, naming the purchase by hash — not
  as a journal entry, which would be a transfer of no grams.
- **Reconcile purchases**: one function, in the TUI, the browser and the command
  line, that selects every purchase still standing in storage without a price
  and presents them one by one in an editor, to document after the fact what
  was paid.

### 3 · The report

#### 🔹 A report for the doctor

- **Status**: Planned
- One page for the prescriber: the months asked for, grams a day, which
  strains, what they did by the effect log, and what is left of the fill in
  progress. A PDF, built from `export` with the date range above.

#### 🔹 A tolerance curve

- **Status**: Planned
- Grams a day against the effect log's ratings, rolling: tolerance creep is the
  dose climbing while the effect stays flat. Breaks marked where the grinding
  stopped, and the replay showing the curve bend through them.

### 4 · Trust

#### 🔹 A chain others can check

- **Status**: Planned
- Sign the chain's head with a key of one's own and anchor it with
  OpenTimestamps, so a doctor or an authority can check a report against it
  without trusting the patient or wits.

#### 🔹 Proof of what is carried

- **Status**: Planned
- In Germany a patient must be able to show that what they carry matches what
  was prescribed. wits knows what is in every jar and which fill dispensed it: a
  view of the holdings against the prescriptions, as a signed QR code that
  verifies against the journal's chain.

### 5 · More than one device

#### 🔹 A cached fold

- **Status**: Planned
- `wits init` creates `.wits/index/` and **nothing writes to it**. Replaying the
  journal on every open is milliseconds at 1429 entries, so it does not matter
  yet; a long-lived server changes that. `wits reindex` should rebuild it, and it
  must stay disposable.

#### 🔹 Sync without a server

- **Status**: Planned
- The journal is append-only and hash-chained, the way git is: two devices'
  journals merge as branches do, entries interleaved by when they happened.
  `wits push` and `wits pull` over any dumb storage — Syncthing, WebDAV, a USB
  stick — end-to-end encrypted with age.

#### 🔹 wits on the phone

- **Status**: Planned
- wits-ui installable and offline-first, as a PWA: an entry made without the
  server waits and is appended when it is back, and a shortcut on the home
  screen records a grind in two taps (`?record=grind` already opens the form).

### Later · hardware and delight

#### 🔹 The scale over USB or Bluetooth

- **Status**: Planned
- Jewellers' scales report over serial or Bluetooth; WebSerial and Web Bluetooth
  read them from the browser. Weighing becomes putting the jar on the scale —
  the reading lands in the form, nothing typed.

#### 🔹 The vaporizer over Web Bluetooth

- **Status**: Planned
- Storz & Bickel's devices — the Volcano, the Mighty+ — speak Web Bluetooth;
  their own web app drives them that way. wits-ui could log a session as it
  happens, with the temperature curve it really ran, and say which terpenes and
  cannabinoids each step released from the boiling points wits already knows.

#### 🔹 Reading the pharmacy label

- **Status**: Planned
- Photograph the label and fill the buy form from it: name, ratio, grams. The
  OCR runs on the device (Tesseract compiled to WebAssembly) — a photograph of
  a prescription leaves the machine for nothing.

#### 🔹 A map of the strains

- **Status**: Planned
- Every product placed by its terpene and cannabinoid profile, coloured by what
  the effect log says it did. "Something like the Wedding Cake" becomes its
  nearest neighbours.

#### 🔹 The replay as a film

- **Status**: Planned
- Render the replay to MP4 or GIF: three years of the shelf filling and
  draining in thirty seconds, from the same frames the browser draws.


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
