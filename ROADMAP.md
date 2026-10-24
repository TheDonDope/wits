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

Nothing is dated ahead. Entries dated in the future were once allowed, a fill
entered before its pickup held off the balances until its day; it took a
second fold, an "upcoming" list in status, the journal, the API and the
browser, and a special case in every check, for something that is a plan and
not a record. The recorder refuses an entry dated after the moment it is
written ("an entry is recorded once it has happened") — before `buy` touches
the catalog — and `fsck` names any a journal written before then still holds.
`Shortfalls` judges every entry on its date. Grams are flower, not money: an account
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

Newline-delimited JSON, opened `O_APPEND`, never rewritten but by a pull (below) — so a failed write
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
`remote`, `push`, `pull`, `refill`, `avb collect` and `avb use`, `price`, `costs`, `report`, `seal`, `verify`, `carry`, `describe`, `like`,
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

`export` writes Markdown that reads without Wits: the fill in progress, or
every fill with `--all`. `--from` and `--to` make one document for a range of
days, for an appointment — the range's own totals, labelled so, what each
product was dispensed, ground and vaporized, the sessions' ratings with the
count each mean stands on, and every entry, an undone one marked. An undone
entry and its correction count in no total. `--site` writes a folder GitHub
Pages serves as it is: an index of the fills, newest first, and a page for each,
linked to the index and to its neighbours. A fill's grinding is labelled "from
any jar", since jars drain oldest first and a fill's tenure grinds the shelf.

`avb collect` and `avb use` put grams into the AVB jar and take them out
(`Recorder.CollectAVB`, `UseAVB`). A collection is from one device and from the
sessions put through it since it was last emptied (`ledger.Uncollected`). It is
shared between their products in proportion to what each put in, in whole
centigrams that add up exactly (`ledger.Split`, largest remainder), as an entry
a product written together, and more cannot come out than went in. A use
draws the jar down as a whole, in proportion to what each product holds. What
stays in **consumed** is what the vapour took. `wits avb` and the API's
`/api/avb` give the jar, what each device has waiting, and every collection's
yield, put back together from its entries by moment and device
(`ledger.Yields`). The browser records both on Record's **AVB** tab (`v`),
picking the device when only one has sessions waiting, and its AVB tile shows
the last yield. The bundle already carried both types, so nothing in the
format changed. The terminal interface shows AVB but cannot record it yet.

Imported history holds no sessions, because the spreadsheet only recorded
grinding, and nothing was invented to fill the gap. So the stash of a product
recurring across cycles reads high until it is worked down.

On a phone the record dialog's two-field rows did not fit: Material sizes a
field's input at 180 px, and the form was sized to the dialog without its
border. Fields in it now shrink to the form, and the form to the dialog.

`refill` is when to order the next prescription (`ledger.PlanRefill`): the
supply projection's empty day, the shelf held when the fill arrived less every
grind since, run down at the fill's grams per day with grinding, less the
pharmacy's lead time (`lead_days` in the config, 3 until `--lead` sets it).
The projection's own arithmetic, so the dashboards' empty day and the date to
order by cannot disagree: on the workbook 44.80 g at 1.93 g a day is empty on
17 November, order by the 14th. `--ics` writes an all-day iCalendar event with
an alarm at nine, its UID naming the day; `wits status`, both dashboards and the
API's `fill.refill` say the same, and say "order now" once the day has passed.

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

### Sync — `wits remote` / `wits push` / `wits pull`

One repository on several devices through a folder: Syncthing, a USB stick,
anything that carries files. The folder holds one file, `wits.age`, a gzipped
bundle encrypted with age (X25519) to the key in `.wits/remote.key`, which
never goes in the folder. The workbook's 1429 entries are 6,911 bytes there.
age authenticates what it decrypts, so a file that was changed or cut short
fails to open rather than restoring as something else. A bundle carries no
hashes, so a fetched one is chained the way appending it would chain it
(`journal.Chain`, `effects.Chain`) and then checked like any other copy.

A pull is a rebase (`Journal.Rebase`): shared entries kept, the remote's
taken as they are, this device's own chained again on top with new hashes,
undos pointed at them, and the effect log following the sessions that moved
while the journal's lock is still held. It is the one rewrite the journal
has, of entries no other device has seen yet; the journal as it was is kept as
`journal.ndjson.orig`. A push is refused unless this device holds everything on
the remote — entries, ratings, products and devices — so two devices cannot
keep rewriting each other's entries. On a pull the remote's catalog entries win.

Two devices can each grind a jar's last 2 g. Merged, the later of the two took
grams that were not there. Both are kept, and `pull`, `status` and `fsck` name
it. The recorder refuses anything more on that jar but an undo (`ErrClash`),
and other jars carry on. Undoing either clears it: `Shortfalls` passes over an
undone entry and its undo, which together moved nothing, and the fold puts back
only what an entry took from its fill's lots. Before that, the undo of a grind
that found its lots empty credited the fill 1.5 g its jar did not hold. Two
devices recording an identical entry in the same second write one entry, since
the hash is the same.

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
frame's moment (`ledger.State.CycleAt`), so a replay's frame is on the fill of
its day. `esc` no
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
`pkg/workspace`, nothing more. Every request answers from the repository as
it stands, read again whenever one of its files changed (below), and every write — buy, grind, sesh, weigh, undo — goes through the recorder under
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

### On the phone — `wits-wasm`

The phone keeps its own copy of the journal, so the server keeps answering
only its own machine. `cmd/wits-wasm` is `pkg/api`'s handler compiled to
WebAssembly (`GOOS=js`) and run in the page. The client's interceptor
(`data/engine.ts`) answers `/api/` from it in the `pages` build, so every page
reads and records as it does against the server, through the same recorder. Go's
js port calls Node's `fs`; `data/memfs.ts` is the twenty calls it makes, in
memory, with "no error" passed as `null` (Go reads anything else as an error)
and only error codes Go has an errno for (any other panics it). Every file the
engine changes under `/wits` is written to IndexedDB after each call, and the
page asks for persistent storage. The lock is a no-op there (`lock_js.go`): one
page, one thread.

Sync is `remote.Pull` and `remote.Push` against a stand-in folder: the
`wits.age` the phone opens is put there, and what the push writes there is
shared or downloaded to save back, since a browser cannot write into a folder
by itself. With nothing yet to save, there is no file. The key is pasted once
and kept in the phone's `.wits/remote.key`. A phone can also be the first device,
making the key.

It is served from GitHub Pages (`make pages`, `.github/workflows/pages.yml`):
installable, with a manifest and Angular's service worker caching everything
the engine included, and shortcuts to `?record=grind` and `?record=session`. On
a touchscreen the key hints are hidden. The engine is 8.9 MB, 2.4 MB gzipped;
`httptest` was swapped for a ten-line recorder, which kept the testing flags out
rather than saving much — the size is the API and `net/http`. Before this,
the 5.2 MB of a first build was a stub whose unused imports the linker had
dropped.

Checked: `make wasm-check` runs the engine on MemFS under Node against the CLI
through a folder, both ways, and fails unless both end holding the same
entries. In headless Chromium at phone size, the `pages` build pulled the
workbook's 1429 entries in 10.5 s, registered its service worker, kept the
repository in IndexedDB, and drew fill 30 as `wits status` does: day 57, 36.73
g of 60, 19 days' worth, 8.07 g in 2 older jars.

### A chain others can check — `wits seal`, `wits verify`

A seal (`pkg/seal`) is the journal's head signed with an Ed25519 key of one's
own, made in `.wits/seal.key` the first time and kept out of every bundle and
remote. It is six lines of text: `head`, `entries`, `at`, `key`, and a `sig`
over the lines before it. The head's hash covers every entry before it, so a
seal fixes the journal up to there. `wits verify` checks the signature and
whether the journal here holds that head at that entry; a journal grown since
still does. Outside a repository it checks the seal alone. The fingerprint,
`SHA256:` and base64 the way ssh writes one, is what to hand out (`--key`).

`--timestamp` is OpenTimestamps, written in-tree: the seal's SHA-256 goes to
the four public calendars, the only network call wits makes, opt-in and said
plainly when it happens. Their answers merge into one standard `.ots` proof,
pending until a calendar's commitment is in a block. `--upgrade` asks the
calendars again and puts the Bitcoin attestation in place of the promise. The
four live calendars answered a test digest, the proof parsed, and it wrote
back byte for byte. The JavaScript reference library would have been the
independent check, and installing it here was refused. `ots verify` on the
user's side is the check that needs neither wits nor that library.

`wits report --seal` seals before writing, and the footer names the seal and
its key. Bundle format 5 carries the seals and their proofs, which travel in
push and pull as well. A proof is only completed, never undone, so a push is
refused if it would take back a proof the remote finished, and a pull keeps
the one that got further. fsck checks every seal: its signature, its head, and
that its proof is of it. On the workbook: 1429 entries sealed and timestamped,
verified in place, outside the repository and after a restore, and the sealed
report's footer read.

### Read once, while nothing changes — the server's snapshot

A cached fold was planned: the replayed state kept in `.wits/index/` so an open
would not replay the journal. Measured on the workbook, it would have saved
almost nothing. Opening takes 12.2 ms: reading and decoding the journal 4.3,
verifying its chain 5.2, the catalogs and the logs about 2, and the fold
0.63. What a long-lived server spent its time on was reading and verifying,
on every request, a journal that had not changed since the last. On the
phone's engine the dashboard's nine requests took about 450 ms and every
four-second poll 49.

So the server keeps the repository it read last (`api.Server.snapshot`) and
reads it again only when a file has changed: `workspace.Stamp`, the size and
time of the journal, the effect and price logs, the catalogs, the config and
the seals' folder, seven stats instead of a read. A snapshot is only read
from, and measures against now. Writes read the journal under its lock as
they always did, and a command in another terminal is seen on the next
request. The terminal interface's stamp had left out the price log, the
config and the seals, so a price written elsewhere did not refresh it; it
uses the same stamp now. On the phone's engine the dashboard takes 21 ms once
warm, 92 cold, and a poll 0.4. Every view read in parallel from one snapshot
passes the race detector. Nothing goes on disk, so there is nothing to
rebuild: `wits init` no longer makes the `index/` nothing ever wrote to, and
`wits reindex` has nothing to do.

### The replay as a film — Film, on the Jars page

Three years of the shelf in thirty seconds, as a video the browser records
itself. A capture of the page frame by frame was not to be had, since a
browser cannot draw its own HTML into a video. So the frames are drawn on a
canvas from the same replay the Jars page plays (`data/film.ts`, `viz/film.ts`):
900 moments spaced evenly from the first entry to the last, so a quiet month
lasts as long as a busy one. Each shows the jars that hold sealed grams, sealed
blue under ground green, filled against the most each ever held, and keeps them
in the order they arrived. A stash the record never drew down stays off it: the
imported history has no sessions, so its stashes only grow.

`MediaRecorder` turns the canvas into WebM in VP8, or MP4 on Safari. Two
choices were tried and dropped here. MP4 first: Chromium without its
proprietary codecs said it could record one and failed part way. Then VP9:
recording in real time, its software encoder fell behind on this machine and
gave up twenty seconds in. A recorded WebM has no duration, so players showed
0:00 and could not seek; the film writes it into the header's Info element.
On the workbook: 2.5 MB, 1280×720, 30.00 s by ffmpeg, which decodes it
cleanly, and its frames read, November 2023 to the shelf of September 2026.
The README's GIF is made from it.

### A map of the strains — `wits like`, the Effects page

Every product dispensed, placed by its profile (`report.Strains`). Across is
its THC, readable off the map; up and down is the rest of the profile — the
words of the cultivar's name without their digits, so "MAC1+", "MAC 1" and
"MAC3" are one family, and the terpenes where both products have them, dominant
first, as a weighted Jaccard — laid on one line by classical multidimensional
scaling and spread by rank. Two axes of scaling alone were tried first and
crowded the products the workbook knows only by THC into a corner. What a
product is most like is the whole distance, cannabinoids included, and a
terpene profile not written down counts as halfway alike, not as alike: at
first it made two unknown profiles nearer than two matching ones. Each point
is coloured by the effect log's overall mean and sized by the grams dispensed.

Terpenes are written down with `wits describe --terpenes` or on the second
page of the terminal's product form; that form is two pages so it does not
outgrow the terminal. "Myrcen" and "β-Myrcene" are one terpene
(`catalog.TerpeneKey`). Bundles had never carried a product's terpenes, so a
restore dropped them; format 6 carries them. The workbook lists none, so it
places by name and THC for now: the two Wedding Cakes find each other first,
then the other Cakes, and the MAC1s find their family. A tested profile shows
the terpenes pulling two products together past one nearer in THC.

### Proof of what is carried — `wits carry`

What is held, against the fills that dispensed it, as a signed statement
(`seal.Carry`) and a QR code. Each product gets the jar's own balances, sealed,
ground and AVB, which are exact, and the fills its sealed grams stand on, or
its last fill once they are all ground. The stash cannot be split by fill,
only storage is kept in lots, so nothing pretends to split it. The default is
the products on the shelf; `--all` adds every jar holding anything, since the
imported history's stashes, never worked down, read 1209 g. On the workbook
each of the five jars on the shelf holds exactly its fill's 20 g, 100 g in all.

The code carries `verify.html#z…`: the statement deflated and in base64,
after the `#` that a browser never sends, so the host never sees it. Deflate
and the lowest error correction took the code from 117 modules across to 89.
`verify.html` is plain HTML in the phone's build, cached by its service worker.
It checks the Ed25519 signature with WebCrypto, or with its own
implementation of RFC 8032 where WebCrypto lacks it (Chrome before 137: the
headless Chromium here is 128). Both accepted the statement and refused it
with one gram changed. `wits verify` takes the statement or its address and
checks it against the journal too; `/api/carry` and the Jars page's **What I
carry** show the code. The QR code is drawn with `rsc.io/qr`. It has not yet
been read by a phone's camera here; that is the check left to do. A phone
signs with its own key until the computer's is brought to it.

### Tolerance — the analysis tab

Grams a day against the effect log's ratings, rolling (`data/derive.ts`:
`tolerance`, `breaks`; `viz/tolerance-curve.ts`). The browser derives it from
the entries, so the replay draws it frame by frame. Each day carries the grams
ground per calendar day over the trailing 14 days and the mean overall rating
of the sessions in the same window, by each session's latest rating. An undone
entry counts in neither. Breaks are runs of a week or more with nothing ground
and no session, shaded behind the curve, a break still going included. Three
days was tried first and marked the habit: the workbook grinds every two or
three days, and its last 90 days held eleven. At seven it holds one, 12 to 18
September, and ten over the whole record, the longest 99 days, the same as a
count from the raw journal. The curve's last point, 0.75 g a day, is the
journal's last 14 days, 10.48 g. The workbook has no ratings, so the rating line
waits for them and the legend says so. The terminal interface does not draw it
yet.

### The report — `wits report`

One page for the prescriber (`pkg/report`): the days asked for, the last 90 by
default, with grams a day with grinding and per calendar day, the weeks as
bars, the strains with their THC/CBD and what each was dispensed and ground,
what the sessions did by the effect log with each mean's count, and today:
the fill's sealed grams, the shelf, and the refill planner's dates. Every
figure in the range is the range's own (`ledger.Standing`, so an undone entry
counts in nothing), and "today" is labelled so. The footer names the
journal's head hash, for the chain others can check (phase 4).

The PDF is written in-tree, the way the charts are drawn: a single page of
text, lines and boxes in standard Helvetica with WinAnsi encoding, which has
the euro sign and the umlauts, and Adobe's widths for right-aligned numbers.
Text sets its own colour, since PDF fills text in whatever colour the last
bar left. Tests check that every cross-reference offset lands on its object.

`/api/report.pdf?from=&to=` serves the same bytes; the server's and the
command's PDFs for the same days are identical. The browser's History page
downloads it, and on a phone it is offered to share. In wits-wasm, anything
but JSON crosses into JavaScript as bytes, since a Go string read as UTF-8
would no longer be a PDF; `make wasm-check` checks the phone's report opens
with `%PDF-` and ends with `%%EOF`. On the workbook, 1 August to 24 October
reads 67.56 g on 33 of 85 days, 2.05 g a day with grinding, 0.79 g per
calendar day and 60.00 g in 3 purchases, the same figures as `export --from
--to`, rendered with pdf.js and looked at.

### What it costs — `wits price`, `wits costs`

A price goes in `.wits/prices.ndjson` (`pkg/prices`), the effect log's twin:
append-only, locked, hash-chained and verified over the stored bytes, naming its
purchase by hash, a re-pricing superseding (`prices.Latest`), the log keeping
both. Amounts are euro cents, parsed as typed: "120,50 €" and "€120.50" alike.
A price is given with `buy --price`, with `price <purchase> <euros>`, or in one
sitting by **reconcile purchases** (`Recorder.Unpriced`): every purchase still
in storage without one, or every purchase with `--all`, a question each, a blank
one skipped, nothing written until the end. It is the same walk in all three
interfaces: `wits reconcile purchases`, `$` in the terminal, and the browser's
**Costs** page. A buy whose price fails still stands, and says how to price it.

`wits costs` and `/api/costs` (`prices.Analyse`) give what was paid in all and
per gram, by fill and by month, and per session that helped: rated 4 or more
overall, at its product's price per gram. An undone purchase cost nothing, and
an unpriced one is counted, not guessed at. Bundle format 4 carries the log
after the ratings, restore writes it back byte for byte, fsck checks its chain
and names a price for no purchase, and push and pull carry and rebase it. A
price follows its purchase to the purchase's new hash, as a rating follows its
session. The workbook has no prices. Priced by hand in a copy, fill 30 read
520.00 € for 60 g, 8.67 € a gram, the one session rated 4 cost 3.40 €, and the
bundle restored the log identical.

AVB tracking needed no format change, since the bundle already carried both
entry types, so the two did not land together as planned. They are two
releases: AVB in v0.60.0, prices with the one format change in v0.61.0.

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

In this order: each phase stands on the ones before it. The first four —
what was open and quick, the physical record complete, the report, and trust
— are built, above; the report and the tolerance curve read the effect log, and say
so plainly while it holds no ratings. The phone came after sync, because the
server answers only this machine. Timestamping a chain and syncing over WebDAV
are the first things wits would send anywhere, so both are opt-in and said
plainly.

### 5 · More than one device

#### 🔹 WebDAV for the remote

- **Status**: Planned
- `wits push` and `wits pull` reach a folder; a WebDAV server is the first
  remote wits itself would send anything to, so it is opt-in and said plainly.
  A conditional write (`If-Match`) closes the gap a folder leaves: two pushes
  at the same moment, the second writing over the first. The device that loses
  still holds its entries and rebases them on its next pull, but the remote
  does not say it lost.


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
