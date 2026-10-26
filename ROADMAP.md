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

The effect log and the price log were still put back a record at a time, each
append reading the whole log and flushing it: the workbook's bundle with 1500
ratings and 200 prices took 21.2 s to restore. Both logs are one chained log
now (`pkg/chain`, generic over the record; a rating and a price say only how to
reach their chain fields), and each goes back as one batch (`AppendAll`): 0.10
s, the files byte for byte what the old restore wrote. The ratings and prices
are chained in memory before anything is written, so a bundle whose logs do
not hold together is refused before it leaves a journal behind.

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

An undone entry and its undo count in no total. The fold settles which entries
stand once, over the whole journal (`ledger.Undone`, `State.Standing`,
`Cycle.Standing`), since an undo is dated when it was made and can fall outside
the fill or the range the entry it undoes is in. Only the export and the report
used to leave them out: the jar was right, but the rate, the shelf, the refill
date, the calendars, the analysis and the jars' lifetimes counted the undone
grams, in the terminal, the commands and the browser alike. Buying 20 g,
grinding 2 g on two days, and undoing a 5 g grind read 3.00 g a day and "11.00
g on the shelf … order by Mon 26 October"; it reads 2.00 g a day, 16.00 g, and
Fri 30 October. The shelf's projection is one function now (`ledger.Project`),
which the terminal and the browser both draw and whose empty day is the refill
planner's — there were three, and they rounded the day differently.

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
it stands, read again whenever one of its files changed (below), and every write — fill, grind, sesh, weigh, undo — goes through the recorder under
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

### A phone's navigation — the dock

At 375 px, an iPhone 11 Pro upright and the narrowest the interface is laid
out for, the top tabs showed one icon at a time between paging arrows, Record
floated over whatever was under it, and Analysis and Costs were wider than the
screen. Under 640 px the tabs now give way to a dock: Dashboard, Journal and
Jars, Record raised between them, and More, a sheet with the rest (Analysis,
Effects, History, Costs, and Sync on the phone build). The top bar names the
page in place of the tabs, the help line and the floating button go, and both
bars take the safe-area insets (`viewport-fit=cover`). Every page's grid caps
its column at the screen (`minmax(0, 1fr)`): a grid column grows to its widest
child, and Analysis's five scopes had pushed it to 503 px; they scroll on their
own now. Measured with the scroll width of each page at 375 px: none wider.

The page names its build: `git describe --tags --always --dirty`, stamped by
`make ui` and `make pages` through `ng build --define` (`data/version.ts`;
"unstamped" from `ng serve` and the tests), at the end of the help line and,
since a phone has no help line, at the foot of More. The phone's build is
cached by its service worker, so a page can be older than the one deployed,
and nothing on it said which it was. Pages checks out the whole history so
the tags are there to describe.

Two pages were then fitted to it. Analysis's scopes go by short names on a
phone (Fill, 30 d, 90 d, 12 mo, All), five equal segments the width of the
card, no check mark beside the chosen one, so all five show in every theme,
the monospace ones included. The Jars page was 12,900 px tall at 243 px a
jar: a phone gets a compact card, the jar small beside the name, the chips
with the day it was last used, and the four figures in one line, and the
buttons as a grid of two under the shelf/finished switch. 6,800 px for the
53 jars of the workbook.

### Which repository, and importing — the Repository page

`wits-server` started on one repository and kept it. The interface can open
another now: the Repository page browses this computer's folders
(`GET /api/dirs`: a folder's folders, hidden ones left out, each saying whether
it holds `.wits` of its own), opens the one picked (`POST /api/repo`), or makes
one in a folder, after asking. The server swaps the repository under the lock
that guards its snapshot, so a request already under way finishes on the one it
began on, and the client reads every view again. It must be the folder's own
repository: a folder inside one is refused, though a command run there would
find it. Recent ones are remembered in the browser.

A workbook can be imported from the browser (`POST /api/import`): sent as the
file itself, in the spreadsheet's own type, which a page elsewhere cannot send
without a preflight the server never grants, as with JSON for every other
write. It is a dry run unless `?commit=1`, reporting what `wits import` reports
— worksheets, products, entries, grams dispensed, ground and unaccounted, the
merged spellings, the anomalies — and commits through `importer.Commit`, so
into an empty journal only (409 otherwise). The importer reads a stream as well
as a path now (`importer.ReadFrom`). Tested against the committed workbook,
through a repository made by the endpoint: 53 products and 1,429 entries, the
dry run writing nothing and a second import refused.

The page is a tab of its own on a computer, the eighth, and under More on a
phone.
It was first a folder chip in the header instead, to spare the tabs a width,
and was not found there; the name of the repository served is in the
verified dot's tooltip now. Eight labelled tabs take 963 px, so the clock goes
below 1500 px and the labels below 1260: measured every 10 px from 641 to
1920, the bar pages at none, 48 px spare at the tightest.

The phone has it too. At first it was left out (`desk.go` was `!js`), the
phone holding one repository and the spreadsheet reader weighing on its
engine: 9.7 MB, and 11.9 MB with it, 450 KB more gzipped. But that left a
phone unable to start from the workbook without a computer, and with a
single journal to try anything in. Its home folder is `/` on its own file
system, the first repository `/wits`; the stand-in folder and the temporary
one are hidden (`/.remote`, `/.tmp`), so the page lists only what was made
there, and every other file is kept in IndexedDB. Making a repository in a
new folder is one step now on both (`POST /api/repo` makes the folder when
the one above it is there), since a phone has no folders until it makes
them. The engine sends a workbook across as bytes in its own type, syncs and
shows the key of the repository open, and reopens it next time (`/.opened`).
`make wasm-check` makes `/book` in the engine, imports the workbook there and
opens `/wits` again: 1,429 entries, as `wits import` writes. With Sync, the
phone's build has nine tabs, so there the clock goes below 1600 px and the
labels below 1380: measured every 10 px from 641 to 1920, the bar pages at
none.

### Four tabs, and what was forgotten

The browser had grown to eight tabs, nine on the phone, the dock leaving five
of them under More. There are four now (`data/pages.ts`, read by the routes,
the tab bar, the dock, the keys and the phone's top bar):

- **Dashboard.**
- **I forgot to** (`Forgot`), what the ledger can show was left undone,
  counted on the tab (`Todo`).
- **Shelf**: Jars, Journal and Devices.
- **Insights**: Analysis, Effects, History and Costs.

A tab that holds several pages is one component (`Section`) with its pages as
child routes and a row of its own to pick among them. Repository, and Sync on
the phone, moved to the header's menu (⋮), with the build and the theme's name
at its foot for the phone, which has no help line. The help line names the
theme beside the build. `1`–`4` go to the tabs, and `h`/`l` step through all
eight pages in turn. The old addresses redirect.

**I forgot to** gathers what was spread over three pages and a dialog, each
section shown only while there is something in it:

- **Sessions to rate**, from the last 7 days (`RATE_DAYS`): a rating is asked
  for while the session is remembered. The queue (`RateQueue`) moved from
  Effects, which now links to it.
- **Pickups to price** (`PriceWalk`, moved from Costs): counted by the fills
  still in storage with a purchase unpriced, shown while any purchase lacks a
  price.
- **Stashes left behind** (`StashClear`): a jar with nothing sealed left whose
  stash still holds grams, the sessions never logged. All are ticked, then
  cleared together through `/api/flush`, as the Jars page clears them.
- **AVB waiting** in a device, with **Collect** opening Record's AVB tab on
  that device (`openRecord`).
- **The jars still sealed**, with **Weigh** opening Record's Weigh tab on that
  jar. These are offered, never counted: a weighing that agrees with the
  ledger writes nothing (`record.ErrNothingToReconcile`), so wits cannot tell
  a jar was checked. Counting them would be a reminder that never clears.
- **Terpene profiles** missing (`TerpeneQueue`): every jar dispensed with no
  terpenes written down, those on the shelf first, each looked up on its own
  or all in turn (**Research all**, a pause between so the searches are not
  asked too fast), and kept one at a time or together. The first ten are
  listed and the rest folded under **Show all**, so 53 jars are not a page
  of their own. Offered, never
  counted, for the same reason: a profile no page names would hold the count
  up for good.

On the imported workbook it counts 50: fills 29 and 30 to price, and 48 stashes
left behind holding 1,154.00 g, since the workbook logs no sessions. Clearing
them wrote 48 adjustments, the count went to 2, and `wits fsck` still accounts
for every gram. Every page measures 375 px on the phone. On a computer, the
clock goes below 1100 px and the tab labels below 780; measured every 20 px
from 641 to 1920, no tab bar pages and no page scrolls sideways.

### A strain rated as a whole — the Shelf, I forgot to, `wits describe --rating`

A session's rating says what that session did; nothing said what a strain is
thought of as a whole, the thing a next prescription is chosen by. A strain
now has a rating of its own (`catalog.StrainRating`): overall, taste and aroma,
1 to 5 each or left out, whether you would have it again, a line, and when. It
lives with the product in the catalog, as its terpenes do, not in the effect
log, which names a session by its hash, and it is written under the
catalog's lock through `Recorder.RateStrain`, which checks it (a score past 5,
or a rating that says nothing, is refused). Rated again, the newer stands.

- **Bundles**, format 9: `sr=overall/taste/aroma/again/when` and `sn=` for
  the line. A restore gives it back, as checked on the demo.
- **Sync**: a pull took the remote's copy of any product that differed, which
  would have dropped a rating made here and not yet pushed. Of two ratings
  the one given last now stays (`newerRating`). `sameProduct` printed the
  product to compare it, and a rating would have printed as a pointer, so it
  compares ratings by value. A rating taken away on one device and still
  there on the other comes back on pull; there is no tombstone for it.
- **The command line**: `wits describe <product> --rating "overall=4 taste=5"
  --again yes --say "…"`, `--unrate`, and a `Rated:` line in what describe
  shows.
- **The browser** (`PUT /api/jars/{slug}/rating`): **Rate** on a Shelf card
  opens the form over it, a jar's page has it inline, and **I forgot to**
  asks after every strain used in the last 90 days with no rating, counted on
  the tab: each clears when rated, or once it has gone three months unused.
  "Used" is a grind or a session (`last_used`): the stashes cleared in the
  demo wrote an entry against every jar the same day, and "last seen" said all
  53 were used that morning. On the demo it asks after six.

`readProduct` reached complexity 16 with the rating, so its four plain text
attributes are read from a table.

### What a strain is like, by its terpenes — the map, the jar, `wits like`

With terpenes written down for 50 of the demo's 53 products, the map of the
strains and "most like it" were rebuilt around them (`pkg/report`). Alike was
the cultivar's name, the cannabinoids and a weighted overlap of the terpenes,
and the map put THC across. It is now mostly the profile:

- **The terpenes**, as vectors: each weighs by its place, 1/√(rank), and by
  how rare it is among the profiles (inverse document frequency). Limonene is
  in 45 of 50 profiles and Terpinolene in 9, so sharing Terpinolene tells
  more. Compared by cosine.
- **The taste**, family by family (`cannabis.Aroma`): each terpene's flavours
  fall in eight families — citrus, fruity, floral, sweet, herbal, woody,
  earthy, spicy — so two citrus-led profiles are near even through different
  terpenes. The profile is 70 % terpenes, 30 % taste.
- **The rest**, a little: 85 % profile, 10 % the cultivar's name, 5 % THC and
  CBD. A product without terpenes is compared by name and cannabinoids alone,
  at half weight, so it ranks below any known profile.

Each match carries its score and the terpenes the two share. The map is the
profiles laid out in two dimensions by classical multidimensional scaling
(the second axis taken after shifting by the first eigenvalue, so a negative
one cannot be mistaken for it). Each axis is spread half by rank, since two
outliers pressed fifty strains into a corner. A product with no profile has
no place, and the map says how many.

`pkg/cannabis` keys terpenes for both catalog and research now
(`cannabis.Key`, its longest word less isomer prefixes, so "Alpha-Pinen" and
"Trans-Caryophyllen" key right), tastes the minor terpenes the lookups find —
Ocimene, Farnesene, Guaiol and others, which have no boiling point — and puts
a profile's taste in words, its three strongest flavours from three families
("Citrus, flowers and pine"). `/api/terpenes` lists every terpene the shelf
names, its taste, how many products name and lead with it, and their sessions'
mean overall rating, weighted by rated sessions and rank.

In the browser the map marks each strain by the group its strongest taste
falls in — five, each a colour and a shape. Eight colours could not be kept
apart: run through a palette validator, Aurora's green and rose are ΔE 3.2
apart to a red-green colour-blind eye, so shape carries identity as well.
Picked, a strain shows its taste as a wheel (spokes on a square-root scale, so
the second and third notes show), its terpenes and its five nearest, and
pointing at one draws its taste over the picked one's. The names placed on
the map try four spots each and take the first that clears the rest. Under the
map, the terpenes on the shelf; on a jar's page, its taste, each terpene's
flavours, and its matches with their scores. On the demo, Farnesene's products
were rated highest (4.5 over 191 sessions), and Morning Sun, a Terpinolene
profile, finds the Terpinolene hazes: Ghost Train Haze, Jack Haze, Frozen
Lemon Mints.

The demo also showed the Effects page 587 px wide on a phone: the
product-by-dimension matrix, now full, put seven columns beside the names. On a
phone each name now takes its own line above its cells, and the temperature
bars stay inside their card. Every Insights page measures 375 px again.

### A window of its own — the AppImage, in Electron

The AppImage opened the interface in the browser, a tab among others, with a
terminal window to close to stop it. It opens a window of its own now: the
same server, the same page, shown by Electron (`desktop/main.js`, pinned to
44.7.0, the newest release then two weeks old). AppRun still starts the server
and makes the repository; it then runs the window in the foreground, and
closing the window stops the server. Launched again, the window comes to the
front (Electron's single-instance lock). `WITS_BROWSER` keeps the old way,
`WITS_NO_WINDOW` (or `WITS_NO_BROWSER`) the server alone, and `cli` and the
`wits` link are as they were. The page gets nothing of Node; links off the
server open in the system's browser, started with the system's own library
path rather than the image's.

Electron, not a system web view: Wails would have made the image about
25 MB, but needs WebKitGTK on the machine, and this Steam Deck has none, nor a
Chromium to borrow for an app window. Electron carries its own: the image went
from 22 MB to 130 MB, most of it the 219 MB Chromium binary compressed. zstd
at level 19 saved under a megabyte, and the AppImage runtime reads nothing
else. Only English of Chromium's locales is kept.

Chromium links against NSS, NSPR and ALSA, which SteamOS lacks, so the image
carries them from the build machine (`./doctor.sh env` here, the Ubuntu
packages in CI) and puts them first for the window alone. npm no longer runs
install scripts unasked, so the build runs Electron's to fetch its binary.
Chromium sandboxes its pages in user namespaces; where AppArmor withholds them
(Ubuntu 24.04 and later), AppRun runs it with `--no-sandbox`. `appimage.sh
check` now also opens the window on the server it started and waits for the
page to load (`WITS_SMOKE`), on the display there is or one `xvfb-run` makes —
headless, Chromium crashed before loading — and passes with the system's
library path removed, on the carried libraries alone. A pull request that
touches how the image is made builds and checks it too (`appimage.yml`).

### A demo — Seed with demo data, on the menu

`assets/demo-seed.wits` is a bundle made to show every page with something on
it, and **Seed with demo data** on the header's menu opens it
(`POST /api/demo`, `pkg/api/demo.go`): restored into `wits-demo` in the home
folder, `/wits-demo` on the phone, and served; asked for again, the repository
there opens as it was left, and a folder by that name holding anything else is
left alone. The interface ships the bundle (`wits-ui/public/demo-seed.wits`, a
link to the one in assets, which the AppImage copies as the file); the phone
restores it on its own storage like the computer. `wits restore`'s code moved
to `bundle.Restore` so the server can run it.

`./demo-bundle.sh` (`make demo-seed`) makes it: the workbook imported into a
new repository, then `wits-ui/scripts/demo-seed.ts`, bundled by esbuild, run
against it through wits-server's API, every entry through the recorder:

- the 37 invoices read by the browser's own reader (`readInvoice`, pdf.js
  under Node) and matched: 32 fills priced, 5 matching no fill in the
  workbook. The pharmacy is kept; the invoice's number, which the batch
  writes into the price's note, is not.
- the 15 device presets, as Add a device makes them;
- a year of sessions from a seeded generator: after every grind of the last
  twelve months, a few sessions of it over the next days, mostly in the
  evening, on the Venty, the Mighty+ and the Frolic; 897 recorded, 718 rated
  — overall from the product, the heat and the hour, sleep at night, focus in
  the morning — and 76 AVB collections. One session the recorder refused.
- every jar's terpenes looked up: 50 of 53;
- the 48 stashes left behind cleared.

Sessions come before the clearing, not after: a session dated in the past
draws on that day's stash, and the recorder checks every date, so one recorded
after a stash was cleared would take grams no longer there. The bundle is
2617 events, 165 KB. Checked by clicking it: on the computer the dashboard
opens on the demo (AVB 102.93 g, the last emptying 54 %), and the Pages build
restores it on the phone in /wits-demo, 2617 entries.

### The AppImage — every release, one file

`appimage.sh` builds wits as an AppImage, `dist/wits-<version>-x86_64.AppImage`
(21 MB). Inside are `wits` and `wits-server`, static (`CGO_ENABLED=0`) and
stamped with the version, and the interface the server serves, without the
phone's engine. The launcher (`packaging/appimage/AppRun`) serves the
repository in `~/.local/share/wits`, made on the first launch, or the one it
is given, on 127.0.0.1:7341, and opens the browser. Launched again while it
runs, it only opens the browser. `cli …` runs the command line. It runs in a
terminal window (`Terminal=true`), and closing the window stops the server
with it. `appimagetool` 1.9.1 and the type-2 runtime 20251108 are pinned to
their SHA-256s, and the runtime is handed in, so the build fetches nothing
unpinned.

`./appimage.sh check` (and `make appimage` after building) runs the image in
a throwaway home, in a session of its own, since the runtime passes no signal
on. It checks four things: a first launch makes its repository and serves the
API and the interface, a second finds it running, and `cli --version` names
the version. The release workflow (`.github/workflows/release.yml`) does the
same for every release when it is published. It builds the tag, checks the
image, and attaches it with its `SHA256SUMS`, and it can be run again for a
tag.

On this machine, the first build took 16 seconds. Served by the AppImage, the
imported workbook shows its dashboard (fill 30, day 58, 36.73 g left) and its
costs (10,156 € priced from the invoices). The footer names `wits-ui v0.89.0`.

### Devices — Shelf → Devices

A device was a vaporizer with a temperature range. It is now any of the
things a session is taken with (`catalog.Device`):

- a **category**: vaporizer, induction heater, torch, grinder or glass;
- what it is: maker, model and the maker's country;
- electric or manual;
- its way of heating: convection, conduction or hybrid;
- how it's used: session, on demand or both;
- for glass, whether it holds water;
- its **modes**, how long it stays on unused (`AutoOff`), and what it cost,
  where and when it was bought, a link, notes, and whether it is retired.

A mode (`catalog.Mode`) has a name and may heat its own way, have a usual
temperature, and run a session timer. The Limelight Herb Frolic's On demand is
convection at 165 °C, and its Session hybrid at 185 °C for 420 s. An induction
heater's modes are its settings. Every field is left out when empty, so a
catalog written before reads and writes as it did, and a device without a
category is a vaporizer.

A session records more (`record.Sesh`, `SessionWith`): the device's mode, the
device that heated it, that heater's setting, and its timer, all `omitempty`
on the entry, so the entries before them hash as they did. The recorder
refuses:

- a device a session is not had with (a grinder, a heater);
- a mode or a setting the device or the heater does not list, when it lists
  any;
- a heater that is not an induction heater or a torch;
- a heater, or a setting, with no device.

A mode's usual temperature and timer fill in what a session leaves out, before
the device's default temperature. `wits sesh` takes `--mode`, `--timer`,
`--heater` and `--setting`. The terminal's session form offers only the
vaporizers and glass in use.

Bundle format 8 carries all of it:

- a device line's new attributes, the modes as
  `name@signature@temp@timer` joined by `|`;
- a session's `m=`, `h=` (the heater by its place in the header, which now
  lists heaters too), `s=` and `tm=`.

A remote compares devices by value (`sameDevice`), since a list of modes
cannot be compared with `!=`.

`/api/devices` gives each device whole, with what it has done: its sessions
and grams, the sessions it heated, and when it was last used. `POST
/api/devices` adds one and `PUT /api/devices/{slug}` changes one, through
`repo.UpdateDevices`, checked by `Device.Check`. **Shelf → Devices** groups
them by category, each card with what the device is, cost and has done,
including its price per session so far, and opens the form to change it.
**Add a device** starts from a preset (`data/devices.ts`) researched for the
maintainer's devices. Each preset fills in only what could be found, and the
rest stays empty: nothing was found on the Mad Heaters Tempest, the Simrell
Hydro or Old Mate Designs' country. The Record dialog's session asks for the
mode, then the timer when the mode runs one, then for a manual vaporizer or
glass what heated it and at which setting. It shows a temperature only for a
device with a range.

In the browser, on the imported workbook, the Frolic was added from its preset
through the form. A session in its Session mode came out at 185 °C with a 420 s
timer, and a Tornado session heated with the YLL 3.0 at 45 W. The journal
holds both, `wits fsck` verifies, and the Journal reads "Vestratto Tornado ·
heated with YLL 3.0 at 45 W". Every page measures 375 px on the phone.

### Themes in the browser — the palette, `t`

Forty-one themes for `wits-ui`, chosen from the header or stepped with `t`
and `T`, as the terminal's `t` picks among its own. A theme is data in
`src/themes.scss`: a background, text, a primary and an accent, the four
account colours in its own shades, and a style — corners (square to pill, down
to Material's own corner scale), glass (opacity and blur), a backdrop (aurora,
vignette, grid, scanlines, a synthwave horizon, a blueprint grid, Bauhaus shapes, greenbar paper, halftone, a sunburst, contours, blinds, stars, warning tape, a desktop, or none), a shadow (soft, glow,
a hard offset in any colour, a 1995 bevel, none) and typefaces (Inter, Space Grotesk, JetBrains Mono, and
Fraunces and Nunito added for the serif and rounded ones, bundled like the
rest). Each sets custom properties under `[data-theme]`, and Material is built
for both schemes (`theme-type: color-scheme`) with the theme's colours laid
over it, so the light themes are light down to the dialogs and fields.

Every colour the components wrote out — about seventy, `#fff` and white at
five per cent the most of them — now reads a theme variable or mixes one with
`color-mix`. The effect scores run from the theme's warning colour through its
AVB amber to its `good` colour, mixed by the browser. The film's canvas cannot
read a CSS variable, so it reads the theme off the page when it records, and
is painted in the theme that was showing. The picker's swatches wear their own
themes, so the menu shows each one's colours without a second copy of them.
The choice is per browser, in local storage — a look, not a fact about the
ledger, so it travels with no repository — and goes on before Angular starts,
so the page never flashes the default. `?theme=` overrides it without saving,
for a link or a screenshot. Aurora is the interface as it was.

### On the phone — `wits-wasm`

The phone keeps its own copy of the journal, so the server keeps answering
only its own machine. `cmd/wits-wasm` is `pkg/api`'s handler compiled to
WebAssembly (`GOOS=js`) and run in the page. The client's interceptor
(`data/engine.ts`) answers `/api/` from it in the `pages` build, so every page
reads and records as it does against the server, through the same recorder. Go's
js port calls Node's `fs`; `data/memfs.ts` is the twenty calls it makes, in
memory, with "no error" passed as `null` (Go reads anything else as an error)
and only error codes Go has an errno for (any other panics it). Every file the
engine changes, but for its scratch folders, is written to IndexedDB after each call, and the
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
The key is read or made under its own lock (`seal.Key`): two first uses at
once — the browser's carry page and `wits seal` — each made a key, the second
written replaced the first, and what the first had signed could never be
verified. Six `wits carry` started together on a repository without a key
signed with a key that was then lost in 7 of 20 rounds; under the lock, in
none.

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

### A fill of several strains, and invoices — Record's Fill tab

A fill was one product. It is any number now, recorded as one batch
(`Recorder.Fill`, `POST /api/fill`): a purchase for each, all or none, checked
line by line before the catalog is touched so a refused fill leaves no new
product behind, and then each one's price to the price log. A strain's price
is a price a gram, or a line's total; 19% VAT — Germany's, which medical
cannabis pays in full — is added to prices a gram when the fill says so, and
the form's switch for it is on by default.

Invoices are read too: one or more of the pharmacy's PDFs, on the device, by
pdf.js (its legacy build, as the modern one needs `Promise.try`, which Safari has
only from iOS 18.2). Only the text is sent, a row of the invoice a line, and
`pkg/invoice` reads each product with its grams, its price a gram and its
total, the date ordered and the order's number, and the totals with and
without VAT; `POST /api/invoice` names each product as the jar it already is.
An invoice's line prices include VAT — its lines add up to its gross total, to
the cent on the one example — so a strain from one keeps the invoice's total
and VAT is never added to it, even with its grams changed; the price is noted
with the invoice's number. On the example, against the imported workbook: three
strains, each matched to its jar past the invoice's extra words (GM, MOS, PAB)
and past a jar of the same name at another strength, 351.80 € with VAT. The
example stays out of the repository, as it names its patient; the tests use its
rows without them.

An invoice is matched against the fills already recorded (`matchInvoice`,
`pkg/api/match.go`): of the pickups within two weeks of its date, the one holding
most of its strains, with how many days apart and which purchase each line is.
An invoice is dated the day it was ordered and a fill the day it was picked
up, so they are often a day apart: the example is dated 28 August and the
workbook's fill 30 was picked up on the 29th, and it matches with all three
lines. Matched on the same day, the Fill tab does not add the strains again but
offers to price that fill from the invoice's totals; a few days off, it asks
first; matched to nothing, the strains go on a new fill as before.

Two pharmacies' invoices are read (`pkg/invoice`). The first pharmacy has
used three header layouts: the date and number under their headings, or
beside their labels on one row (`Reservierungs ID: … Reservierungsdatum: …`,
`Rechnung Nr. … Rechnungsdatum …`). The second ships, and spreads a product
over rows under its PZN (`block.go`): the name, a row of grams and prices, and
a description giving the strength (THC and CBD) and the strain, which fill in
what the name lacks. Its prices use a decimal point and its dates
`dd/mm/yyyy`. A priced row in the table that is not flower by the gram (a vape
cartridge by the piece, shipping) is `Other`, left out and named, so the
strains and what was left out add up to the gross total. A NUL byte in one
invoice's text is passed over.

A third pharmacy (Grünhorn's, April 2023 to August 2024) names a product on
one row and prices it on its dosage row below: `Dosierung: … 9,99 € 20 199,80 €`,
price before quantity, no unit. A fee, delivery or the narcotics charge, has
the same shape with no dosage, or only a count and a total
(`Dosierung: 1 9,99 €`). Its date is `Ausstellungsdatum`, not the
`Fälligkeitsdatum` above it. Its gross is `Rechnungsbetrag`, and its VAT
`MwSt. (19%)`.

The match compares each line with the products of every fill within 14
calendar days (`likeness`). It was seven, until an invoice of 2 July 2024
turned out to be fill 10's, with all three strains at their grams, picked up
on 15 July. The same jar is certain. Otherwise the strain
words they share count, singular and lowercased, more when the strength (25:01
is 25/1) and the grams agree, and a strength both print differently rules the
pair out. Lines are paired best first, one purchase each. Run against all 21
invoices kept (6 shipped, December 2024 to February 2025; 15 picked up, May
2025 to September 2026) on a fresh import of the workbook:

- **20 matched fills 12 to 30.** The invoices are 0 to 7 days before the
  pickup, and every line went to the purchase with its grams, past breeder
  against distributor names ("Peace Naturals … Sonic Lemon Fuel" is
  Cansativa's), names cut short ("… DK 28/1 Ghost") and 25:01.
- **Two invoices for one fill:** an order of 4 December 2024 re-issued on the
  6th both match fill 13. The first one's 14/1 White Widow matches nothing
  there, the fill holding the 16/1.
- **Priced at the invoices' totals**, they came to 5,028.50 € for 750 g. The
  journal's checksum is unchanged (1,429 entries, none new), and `wits fsck`
  verifies the chain.
- **One matched nothing:** the last invoice (Frozen Lemon Haze and a vape
  cartridge, 29 September 2026) is for a pickup the workbook does not hold.
- **Still without a price:** fills 1 to 11, with no invoice kept, and fill
  18's Ghost Train Haze, which its invoice does not list.

The invoices name their patient and stay out of the repository; the tests use
their rows without the person.

### Reading the pharmacy label — Record's Fill tab

**Read the label** on the Fill tab takes a photograph, the camera's on a phone,
and fills the product and the grams from it. The OCR runs in the page:
Tesseract 7 compiled to WebAssembly, its LSTM engine and its German served by
wits itself under `ocr/` (`data/label.ts`), so the photograph of a prescription
goes nowhere. The library is a lazy chunk of 17 kB; the worker (111 kB), one of
three engine builds (3.9 MB, picked for the browser's SIMD) and the German
(1.3 MB) are fetched the first time a label is read, and the phone's service
worker keeps them. The photograph is turned upright and read at 2400 px at
most; the engine is let go after each reading.

What it reads is text, and `POST /api/label` makes a fill of it
(`catalog.ReadLabel`), recording nothing. The product line is the first
carrying a THC/CBD ratio, without the words around it that say it is cannabis
or how much; a ratio that is part of a date (`03/2027`) or of the pharmacy's
house number (`Hauptstr. 12/1`) is passed over, and a line printed in capitals
is given the case names are kept in, codes like DNK left alone. The grams are
on the line naming the quantity (`Inhalt`, `Menge`, `Liefermenge (g)`), never
milligrams, never past 100 g. With no product line, the ratio is read off the
percentages and the name left to be typed. The catalog then says which product
it already is (`Catalog.Matching`): by name, by slug — the ratio anywhere, a
column heading in front — or, at the same ratio and from the same
manufacturer, by every word of its name but one. The form is filled with that
product's own name, so the fill adds none, and says whether the product is
known or new and what the photograph read, to check before recording.

Checked: every one of the workbook's 53 names was copied off a label, and each,
as written and in capitals, reads back to its own product
(`TestLabelsOfTheRealWorkbook`). In headless Chromium, photographs of two
labels read in 1.0 s: "PEDANIOS 22/1 GHOST TRAIN HAZE" filled the workbook's
Pedanios 22/1 DNK Ghost Train Haze and 10 g, past the address and the batch;
an Aurora 20/1 Pink Kush filled as new, 15 g. Every request, the worker's
included, went to the machine serving the page — the `pages` build too, under
`/wits/`, in 2.5 s.

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
places by name and THC until profiles are written down or looked up (the jar's
page, below): the two Wedding Cakes find each other first,
then the other Cakes, and the MAC1s find their family. A tested profile shows
the terpenes pulling two products together past one nearer in THC.

### One jar in full, and its terpenes looked up — Shelf → Jars → a jar

The dashboard shows every fill not yet emptied (`overview.fills`, newest
first), jar by jar, each in its own scope: its own sealed grams and rate. The
shelf's projection and the refill stay with the fill in progress. Before, an
older fill was only "+ 8.07 g in 2 older jars"; on the workbook it is now fill
29 under fill 30, its 8.07 g in its own three jars, which is the same figure.

A jar on the dashboard or a card on the Shelf opens the jar in full
(`/shelf/jars/<slug>`, `GET /api/jars/{slug}`): the catalog's facts and
terpenes; every fill of it, its own grams dispensed and still sealed, and its
pickups with the price paid and the seller; what the priced ones cost a gram;
ground and seshed month by month; its sessions by device, the temperatures
set, each dimension's mean rating and the latest eight; and the products most
like it. The jar's figures are its life, each fill's that fill's. Ticking a
card selects it for clearing as before; the card itself opens the jar (`o` on
the keys).

**Research terpene profile** (`GET /api/jars/{slug}/research`, `pkg/research`)
searches the web for the product as the pharmacy sells it — Brave Search, then
DuckDuckGo when Brave turns it away, since each answers a client it takes for
a bot with a page of no results — and asks Leafly for the strain
(`prices.StrainOf`, spaced as Leafly spells it: "MAC1" is found as "MAC 1",
and only a hit of that very name counts). Each result's words are read for
terpenes in any spelling (`catalog.TerpeneKey`), cannabinoids and flavonoids
left out; the profile is the terpenes most pages name, then those named
earlier, five at most. Leafly's is marked as the strain's, not the product's.
Nothing is written until kept (`PUT /api/jars/{slug}/terpenes`, through
`Recorder.Terpenes`). On the workbook's first four tried, Lemon Tartz, Wedding
Cake, Bucket List and MAC 1+ each came back with a profile from two to seven
pages. On a phone the dashboard's jars stand three to a row, so two fills are
not a screen each, and the line saying when to order the next prescription
wraps as a sentence; it had been laid out as four squeezed columns.

On the phone the lookup failed with a bare 504. The engine's requests go out
as the page's own `fetch`, and a browser lets a page read another site only
when that site allows it: neither search does, nor Leafly (whose API allows
leafly.com alone), and the service worker turns the refused request into a
504. The phone's engine now asks through Jina's Reader (`research.Phone`,
`Sites.Relay`), which allows any page to read what it fetches: the search as
HTML, Leafly's API as its text, so the same parsers read both. Brave's results
do not come through it, so the phone searches DuckDuckGo. The reader takes
twenty requests a minute and a jar is up to three, so **Research all** on the
phone waits nine seconds between jars and, on a 429, half a minute before
asking once more. Checked in the Pages build in Chromium: the workbook
imported on the phone's own storage, Lemon Tartz found on six pages in 1.7 s
and kept, and Research all found three jars in thirty seconds.

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
**I forgot to** (`PriceWalk`). A buy whose price fails still stands, and says how to price it.

The browser walks it a fill at a time (`PickupCard`): `/api/unpriced` names
each purchase's fill, and a fill is priced together (`POST /api/price/fill`,
`Recorder.PriceAll`). Every purchase is checked before any price is written, so
a bad line refuses the fill whole. Each strain takes a price a gram, with 19% VAT
on top when the switch is on, worked out by the fill form's `rowCents` on the
page and `lineCents` on the server. Or it takes its invoice's line total:
**Scan invoice** reads the PDF on the device and shows its first page beside
the figures. The invoice's match decides what happens: this fill on the same
day fills the prices; a few days off, it asks first; another fill's says whose
it is and hands it over. On the workbook, 88 purchases fall in 30 fills (28 of
three strains, 2 of two). The example invoice prices fill 30 at 351.80 €, its
own total to the cent; 20 g at 5,80 € with VAT is 138,04 € on both sides.

The walk shows the newest fill first and every purchase without a price by
default. **Add invoices** (several PDFs) and **Pick a folder** (`webkitdirectory`,
every PDF in it) read a batch on the device (`InvoiceBatch`), one after the
other, newest first. A row is ticked when it matched a fill with at least one
purchase not yet priced. The days apart are shown, and so are the invoices
that share a fill. Pricing goes oldest first, so a later invoice for one fill
stands over an earlier. All 37 kept invoices, from three pharmacies, read in
3.3 seconds in headless Chromium. 32 were ticked for all 30 fills, and five
matched none: four from April to September 2023, before the workbook's first
fill, and the September 2026 one. Pricing them left one pickup, fill 18's Ghost
Train Haze. Wits then reports 10,156.01 € for 1,244 g, the same as the matched
lines added up apart, with the journal's checksum unchanged. DevTools cannot fill a folder input the way a folder pick does,
so the run went through **Add invoices**. **Pick a folder** hands its files
to the same reader, and has not yet been tried with a real folder.

A price not to be had is written down (`Price.Unknown`, no amount,
`omitempty` so the records before it hash as they did). `Recorder.Unpriced`
asks about the purchase no more. `prices.Analyse` still counts it as without
a price, costing nothing, and a price given later supersedes it. A card's
**No price to be had — stop asking** marks every line left without one. On
fill 18 it wrote one such record. The costs stayed at 750.00 g with a price
and 34 purchases without, and the walk moved on to fill 11. A build from
before this reads such a record as a price of nothing, so an older page, such
as a phone's cached one, should be reloaded.

`wits costs` and `/api/costs` (`prices.Analyse`) give what was paid in all and
per gram, by fill and by month, and per session that helped: rated 4 or more
overall, at its product's price per gram. An undone purchase cost nothing, and
an unpriced one is counted, not guessed at. Bundle format 4 carries the log
after the ratings, restore writes it back byte for byte, fsck checks its chain
and names a price for no purchase, and push and pull carry and rebase it. A
price follows its purchase to the purchase's new hash, as a rating follows its
session. Format 7 carries a price's pharmacy (`s=`) and a price not to be
had (`u=1`). Format 6 dropped both, so from v0.88.0 to v0.90.0 a bundle, a
push or a pull lost the pharmacy. A price not to be had came back as a price
of nothing, so its restore was refused: "a price is more than nothing, got
0.00 €". Run on the workbook priced from every invoice, with one price not to
be had, v0.90.0 restored none of its 89 prices. Now all 89 restore, their
hashes identical, with the mark and all 88 pharmacies. The workbook has no prices. Priced by hand in a copy, fill 30 read
520.00 € for 60 g, 8.67 € a gram, the one session rated 4 cost 3.40 €, and the
bundle restored the log identical.

AVB tracking needed no format change, since the bundle already carried both
entry types, so the two did not land together as planned. They are two
releases: AVB in v0.60.0, prices with the one format change in v0.61.0.

The browser breaks the costs down (`prices.Insight`, `pkg/prices/insights.go`;
`/api/costs` gives them beside the totals):

- every priced purchase, oldest first;
- the purchases grouped by strain, producer, pharmacy and year, each group
  with what it cost, its price a gram over all its grams, and the cheapest,
  dearest, first and last purchase's.

A strain is the cultivar keyed alike under the names it is sold by
(`StrainOf`). The words are lowercased and made singular, and the producer's
words and the short codes pharmacies add (`CA`, `DNK`, `T01`) are left out,
unless the name is codes alone. A number after a short name stays with it,
so MAC 1 and MAC 3 stay apart, and a strength in a name is dropped. A product
with no cultivar is keyed by its name, less the words of every producer known
(`Producers`). On the workbook's 53 products that makes 40 strains. They
include MAC1 under five names from two producers, Hindu Kush from IMC (twice)
and Canopy, and the two Mac Doughnut jars, one of whose names ends in its
strength.

A price now says which pharmacy it was paid to (`Price.Seller`, `omitempty`
like `Unknown`). The invoice reader takes the pharmacy's name from the first
short cell with "Apotheke" in it that starts as a name does, its owner's
"e.K. …" cut off. All 37 kept invoices name theirs: 15, 6 and 16 for the
three pharmacies. The fill form, the pickup card and the batch pass it on, and
a price typed in has none.

**Insights → Costs** draws it. `PricePlot` (`viz/price-plot.ts`) is one dot
per purchase over time, or against THC, with a pharmacy by its theme colour
and by its shape (circle, square, diamond), a trend line, a tooltip, and a
click that opens the dot's strain. `CostStrains` is a sortable strain table
with sparklines, opening to a strain's every purchase. Beside them are the
pharmacies compared, strength bands, years with the change on the year before,
and fills that open to their purchases. The figures the page works out (a
gram lately, a month at this rate, the bands, the fill trend) are in
`data/costs.ts` with tests. Priced from all 37 invoices, the workbook comes to
10,156.01 € for 1,244 g, and the years add up to it: 10.73, 9.75, 6.96 and
6.25 € a gram from 2023 to 2026. Every page measures 375 px on the phone, and
the colours were checked in a dark theme and in Paper.
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

### The user guide — User guide, on the menu, and the docs split

**User guide**, first on the header's menu (⋮) and linked from the `?` sheet,
opens `/guide`: eighteen sections from how wits thinks to the phone, each with
what the page is for, the steps, pictures of it, a button that opens it (or
the Record tab it means), and **the same, in a terminal**: the commands with
what they printed on the demo, and the terminal interface's keys, with a
photograph of its screen where it has one. The content is data
(`data/guide.ts`), and `guide.spec.ts` holds it to the app: ids unique, every
`#link` in the prose a section, every page it opens a route, every picture in
`public/guide`, every section with a terminal way. The pictures are WebP,
1.3 MB in all, in a lazily cached service-worker group, so the phone does not
fetch them on install. The browser's were taken in headless Chromium on the
demo, the terminal's with vhs. A link inside the prose is followed by the page
itself, since `#weigh` would otherwise resolve against Pages' `/wits/` base.
Every section measures 375 px on the phone, and the page was looked at in
Aurora and in Paper.

The README was split the same way: it keeps what wits is, getting it
installed, and the layout and development; `docs/terminal.md` is the command
line and the terminal interface, `docs/browser.md` the server, the window and
the phone.

### The replay, as a timeline and a tape — Analysis, Jars

The replay's entry used to pop in under the play bar as a card of its own,
appearing when play was pressed and changing with every step, so the page
under it jumped. The transport (`viz/transport.ts`) is one panel of one
height now, live or playing. Beside the controls, the day being played and
how many entries in. Under them, the whole record in time
(`data/timeline.ts`): the grams ground in slices, a day each on a span
shorter than 120 days and 120 on a longer one, a tick at every pickup (a fill
of several strains is one), the years marked, or the months on a span under
fourteen months; the part played lit, the playhead glowing, and a pointer
anywhere on it scrubbing to that day (`Replay.scrubTo`). Under that, the tape:
the eight newest entries that tell the story, newest at the right, each one
moved along by a transition as long as one step, so it keeps pace from 1.2 to
20 a second. Live, it shows the last entries recorded. It was looked
at in Aurora and Paper, and fits 375 px on the phone.

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

### 6 · The effect log, put to work

The ratings are what the tolerance curve, the costs per session that helped
and the colours of the strain map wait on, and the workbook holds none. So
filling the log comes first, and reading it second.

#### 🔹 A nudge to rate

- **Status**: Planned
- The phone asks about a session once it has had time to work, ninety minutes
  after it by default, as a notification that opens `/effects` on that session.
  The service worker that already serves the phone's build can show it. A
  session rated already is never asked about, and the nudge is off until it is
  switched on, since it is the first thing wits would say unprompted.

#### 🔹 Did the break work?

- **Status**: Planned
- Breaks are already found (a week or more with nothing ground and no
  session). For each one, put the fortnight before it beside the fortnight
  after: grams a day with grinding, and the effect log's means, each with the
  count it stands on. Whether a week off resets anything is then a question
  the record answers, not a belief, and a break with too few ratings either
  side says so instead of drawing a line.

#### 🔹 What to ask for next

- **Status**: Planned
- Paste or photograph a pharmacy's list of what it has (the label reader's
  parsing, pointed at a list), and rank it by distance on the strain map to the
  products rated best. A cultivar with nothing rated near it is marked unknown,
  not placed by guesswork, and every rank names the products it stands on.

### 7 · The ledger's own tricks

Anything can be rebuilt from any prefix of the journal, and every entry carries
two timestamps. Three things only a ledger like this one can do.

#### 🔹 Then, and what was known then — `--as-of`, `--as-known`

- **Status**: Planned
- `wits status --as-of 2025-03-01` folds the entries that had occurred by that
  day: the shelf as it stood. `--as-known 2025-03-01` folds the entries that had
  been recorded by then: what the journal said that day, before the backdated
  entries arrived. The two differ by exactly what was written down late, which
  is a figure of its own, and the replay transport could scrub either axis.
  Every command that reads the fold could take them; none may write with them.

#### 🔹 Two days side by side — `wits diff`

- **Status**: Planned
- `wits diff <then> <now>` folds twice and names the difference: jars that
  appeared and emptied, what each account moved, the cycles that opened and
  closed between. `git diff` for the shelf, which is a part of git's vocabulary
  with a real referent. Each line speaks one scope and says which.

#### 🔹 A report with entries withheld

- **Status**: Planned
- A prescriber gets the report and the entries it rests on, with the ones not
  theirs to read withheld: each replaced by its hash, so the chain still
  verifies to the sealed head and nothing can have been removed or changed
  unnoticed. `wits verify` checks it, and says how many entries were withheld
  and where in the chain they stand.

### 8 · The physical record, measured

#### 🔹 The pharmacy's scale against mine

- **Status**: Planned
- Weigh a jar sealed at pickup, against the grams its label states (Record's
  Fill tab reads them already). Over the record, each pharmacy and each
  producer gets the mean and spread of what they dispensed against what they
  said, beside what a gram cost there. A weighing is a reading, not a transfer:
  it belongs beside the journal, like the price log, unless it finds grams to
  adjust.

#### 🔹 What the air took

- **Status**: Planned
- Flower dries, and a jar weighed later holds less than was put in. Today that
  shortfall is an `adjust` with a reason typed in. Naming it — drying, its own
  reason, and in the end a way out of the jar of its own, beside the vapour
  and the AVB — keeps the mass balance saying what happened, and gives each
  strain a drying rate per week sealed, and a jar a hint when it is drying
  faster than its kind.

#### 🔹 What the AVB still holds

- **Status**: Planned
- Every session records its device and temperature, and the vaporizer item
  below already reasons from boiling points. Together they put a range on what
  is left in the AVB jar — THC by each product's share of it, less what its
  sessions' temperatures will have released — so an `avb use` is offered an
  estimate in milligrams, as a range, with its assumptions on screen and never
  in a total.

### Later · hardware and delight

#### 🔹 The year, on one card

- **Status**: Planned
- Once a year, one page or a short film from the same replay as Film on the
  Jars page: the grams, the strains, the longest break, the temperature rated
  best, the dearest gram and the cheapest. Every figure is one the report or
  `wits costs` already computes, labelled with its scope.

#### 🔹 Packing for a trip — `wits pack`

- **Status**: Planned
- `wits pack --days 10` is the grams to take at the rate the refill planner
  runs on, with a margin, drawn from the jars oldest first, and a `wits carry`
  statement for exactly those grams rather than the whole shelf.

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
