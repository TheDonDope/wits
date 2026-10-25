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

## Getting started

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

Tab completion offers the handles, with the full name and how much is left
beside each — and only the ones the command can act on, so `sesh` offers what is
in a stash rather than everything ever dispensed:

```console
$ wits grind <TAB>
lcook-281   17.10 g · Cannamedical 28/1 Lemon Cookie
mac1-251    15.00 g · Cantourage 25/1 MAC1+
wcake-221   16.90 g · Enua 22/1 Wedding Cake
```

Completion is a script the shell sources, written by `wits completion bash`
(or `zsh`, `fish`). Bash only runs it through the `bash-completion` package,
which is not always preinstalled — without it the script loads silently and
does nothing. Install that first, give it the directory it reads user
completions from, and write the script there:

```sh
sudo pacman -S bash-completion   # or apt/dnf install bash-completion,
                                 # brew install bash-completion@2
mkdir -p ~/.local/share/bash-completion/completions
wits completion bash > ~/.local/share/bash-completion/completions/wits
```

A new shell picks it up from there; nothing needs sourcing in `.bashrc`, and
no `sudo` was involved. To try it in the current shell without writing
anything: `source <(wits completion bash)`. Zsh wants the script on its
`fpath` as `_wits`, fish in its own completions directory:

```sh
wits completion zsh > "${fpath[1]}/_wits"                    # zsh
wits completion fish > ~/.config/fish/completions/wits.fish  # fish
```

Anything can be backdated with `--date 2026-07-29`, which is what makes a
forgotten evening loggable the next morning without pretending it was entered
then. Without it the entry is dated now. A fill can be dated however far back,
and lands where its date puts it — a fill entered a month late opens its own
cycle a month back. Nothing is dated ahead: the journal records what happened,
so an entry is written once it has. Grinds and sessions are held to what was
there on their date: a grind dated before the fill it draws on is refused.

```console
$ wits status
On cycle 30, opened 2026-08-29 (day 57)

PRODUCT                       STORAGE  STASH  AVB    LEFT
nice-morning-sun-321          11.37g   8.63g  0.00g  57%
all-nations-mac-doughnut-291  12.42g   7.58g  0.00g  62%
sumo-papaya-bomb-251          12.94g   7.06g  0.00g  65%

total                         36.73g                 61%

36.73g of 60.00g left over 55 days, 23 of them with an entry
8.07g more in 2 older jars, 1 earlier cycle still open
1.93g per active day, 1.69g median, 0.81g per elapsed day
About 19 days' worth at that rate
Order the next prescription by Sat 14 November, with 3 days for the pharmacy.
```

Every gram in storage stands on the account of the cycle that dispensed it,
so the total is the fill's own: 36.73 g of the 60 g this prescription
brought. What older cycles still hold gets its own line — those cycles are
simply still open. A jar refilled before it was empty holds two cycles'
grams, and grinding draws the oldest first, the way inventory leaves a
shelf.

The last line is the refill planner. The shelf, 44.80 g in every jar, runs
out on 17 November at the fill's 1.93 g a day, and the pharmacy needs its lead
time before that. `wits refill` says it at length, `--lead 5` changes the lead
time and keeps it, and `--ics refill.ics` writes a calendar reminder for the
morning of the day to order by. The dashboards say it under their projection
of the supply.

```console
$ wits refill --ics refill.ics
44.80 g on the shelf, at 1.93 g a day with grinding: empty on Tue 17 November.
Order the next prescription by Sat 14 November, with 3 days for the pharmacy.
Wrote a reminder for 2026-11-14 to refill.ics
```

`wits` on its own opens the interface: a dashboard of cards — storage, stash,
sessions, devices, two rhythm calendars and a projection of the supply's
decline to its empty day — then the journal, an analysis view scoping from the
current cycle out to the whole history, the storage, the stash, the sessions,
and the devices. `tab` moves between them. Entries can be recorded
there too — `b` for a fill, `g` to grind, `s` for a session, `r` to weigh, `$`
to price the purchases still without one — and `?` lists every key the current screen honours. `q` quits; `esc` only ever
closes the form in front.

The interface keeps up with the repository: an entry made with a command in
another terminal shows within a second, and at midnight "today" moves on.

`t` picks a theme: wits' own, which follows the terminal's light or dark
background, or one of twenty editor themes — Dark+ and Light+, One Dark Pro,
Dracula, Monokai, Darcula, Solarized Dark and Light, Nord, Gruvbox Dark, Tokyo
Night, Catppuccin Mocha and Latte, GitHub Dark and Light, Night Owl, Material
Palenight, Ayu Dark, Cobalt2 and Rosé Pine. The screen follows the cursor as it
moves; `enter` keeps the theme and saves it to `config.yml`, `esc` puts the old
one back. An editor theme repaints the terminal's background the way the
editor would, and the terminal gets its own colours back when wits exits.

The analysis view draws the daily amounts as a braille area chart with a
seven-day average riding over it, and the longer scopes as a calendar heatmap —
one cell per day, colour carrying the amount — so a year of habit reads the way
a contribution graph does: the heavy weeks, the pauses, whether weekends
differ. `s` changes the scope.

Because everything is replayed from the log, the analysis, storage and stash
screens can start from empty and grow the way the record grew: `p` plays,
`←/→` steps an entry at a time, and `+/-` retunes the speed.

## Commands

| | |
| --- | --- |
| `wits init [dir]` | Create a repository |
| `wits buy <product> <amount>` | Record a prescription fill, `--slug` to name it, `--price` for what it cost |
| `wits price <purchase> <euros>` | Write down what a purchase cost; `wits reconcile purchases` walks every one without |
| `wits costs` | What the prescriptions cost: per gram, by fill, by month, and per session that helped |
| `wits describe <product>` | What is known about a product; `--terpenes` writes down its profile, dominant first |
| `wits like <product>` | The products most like one, by name, cannabinoids and terpenes |
| `wits grind <product> <amount>` | Move product from storage into its stash |
| `wits sesh <product> <amount>` | Record a session, drawing on the stash; `--device`, `--temp`, `--note` |
| `wits avb collect <grams>` / `use <grams>` | AVB weighed out of a device, or used up; `wits avb` for the jar and every yield |
| `wits rate <session\|last> <dimension=1–5>…` | Rate what a session did; `--note` |
| `wits status` | What is left, and how long it will last |
| `wits refill` | When to order the next prescription; `--lead` for the pharmacy's days, `--ics` for a reminder |
| `wits log` | The journal, newest first; `--oneline`, `-n`, `--product`, `--current` |
| `wits show [entry]` | One entry in full: what it moved, the jar before and after, what corrects it |
| `wits revert <entry>` | Undo an entry by recording a correction |
| `wits reconcile [account] [product] [weight]` | Make an account agree with the scale; interactive with no arguments |
| `wits device add <name>` / `list` | Register a vaporizer and its temperature range; list them |
| `wits temps <celsius>` | What a temperature is hot enough to release |
| `wits import <file.xlsx>` | Import a tracking spreadsheet; a dry run without `--commit` |
| `wits export` | The current cycle as Markdown; `--all` for every cycle, `--from`/`--to` for a range of days, `--site <dir>` for GitHub Pages |
| `wits report` | One page for the prescriber, as a PDF; `--from`/`--to`, the last 90 days by default |
| `wits bundle` | The whole repository as one compact file; `--gzip` to compress it |
| `wits restore <file>` | Rebuild a repository from a bundle |
| `wits remote add <folder>` | Share the repository through a folder; `--key` on every device after the first |
| `wits push` / `wits pull` | Write the repository to the folder; bring in what other devices wrote there |
| `wits fsck` | Check the journal and the catalogs for damage |
| `wits seal` | Sign the journal's head; `--timestamp` anchors it with OpenTimestamps, `--upgrade` fetches the proof |
| `wits verify <file.seal>` | Check a seal, or a statement from `wits carry`: its signature, the journal it seals, its timestamp |
| `wits carry` | What is held, against the fills that dispensed it, signed; `--out` a QR code (.svg, .png) or the statement (.txt) |
| `wits completion <shell>` | The tab-completion script for bash, zsh or fish |

Every command takes `--help`, and `buy`, `grind` and `sesh` take `--date`.

## Bringing a spreadsheet across

`wits import` reads a tracking workbook and turns each worksheet into a
prescription fill and the grinds that followed it. The default is a dry run: it
reports what it would record, and anything about the spreadsheet that does not
add up, so years of history can be checked before any of it is written.

![Importing four years of a spreadsheet](./assets/wits-import.gif)

Products are resolved **by position**, through the bindings in the running
balance formulas, rather than by the label in the strain column — those labels
are dropdown values that were not always renamed as products changed. On the
records this was written for, every one of the 1209.20 g ground is placed
through a binding, never through a label.

An import only goes into an empty repository: a second one would double every
gram, and could not be told apart from a second helping of the same product on
the same day. It says what it is doing as it goes, timed, on stderr — reading
the workbook, adding the products, writing the entries — and writes every entry
in one go: an entry that does not validate refuses the import before anything
is written.

## Correcting a mistake

Nothing is edited in place. An entry is undone by recording a correction that
moves the same grams back the way they came, so both stay in the log:

```sh
wits log --oneline -n 1
wits revert 8297238 --reason "misread the scale"
```

In the journal, `e` amends an amount and `d` undoes an entry. The log shows
what currently stands; `v` reveals the corrections behind it, `f` filters it,
and `←/→` browses the entries as cards.

The storage screen is two tables: what still holds something, and the history
of every jar weighed down to zero, newest first. The stash screen drills into
the ground product the same way — the stashes holding something above, and
under them every stash worked down to nothing, grouped under the day it was
consumed. The sessions screen tells the other half of the story: how much came
out of the stash, when, through which device and how hot, drawn with the same
charts the analysis view uses. Its newest sessions are listed with what each
was rated; `e` rates the one under the cursor — a line a dimension, `←`/`→` or
`1`–`5` to score, `0` to leave one out — and under the list each product's
overall rating, with how many ratings it stands on. Space ticks jars in either table and `r` weighs
the ticked ones together; `e` corrects a name; `c` records stale stash
remainders from earlier cycles as consumed, which is what four imported years of
grind-only records leave behind. Product names are never abbreviated — the name
column takes what the longest name needs.

## When the ledger and the scale disagree

Ledgers drift. A little is spilled, a session goes unlogged, a scale is read
wrong. The past is not edited to hide it, because nobody knows which entry was
wrong — instead the difference is recorded, and the account agrees with the jar
again:

```console
$ wits reconcile storage wcake-221 16.5 --dry-run
storage holds 16.90g by the ledger and 16.50g on the scale: -0.40g

Dry run. Nothing was written. Re-run without --dry-run to record it.

$ wits reconcile storage wcake-221 16.5 --reason "spilled on the desk"
[4b36264] adjusted 0.40g out of storage of wcake-221, now 16.50g
```

![Reconciling against the scale](./assets/wits-reconcile.gif)

For weighing day, `wits reconcile` on its own is interactive. Pick the account
on the scale (storage, the stash, or the AVB once it holds anything) and tick
the jars to weigh, all of them by default. Each is asked for in turn, with the
ledger's figure beside the prompt, and a blank reading skips a jar; `wits reconcile stash` skips the first question. Nothing is
written until every question is answered. In the interface this is `r`, on any
screen — it weighs the jars ticked on the storage screen, the one under the
cursor, or otherwise the fullest one, since that is the one worth checking.

## The repository

```text
.wits/
  config.yml      # settings — the interface theme
  products.yml    # the catalog
  devices.yml     # vaporizers and their temperature ranges
  journal.ndjson  # append-only, one entry per line; only a pull rewrites it
  journal.ndjson.orig  # the journal as it was before a pull chained entries again
  effects.ndjson  # the effect log: ratings of sessions, append-only too
  prices.ndjson   # the price log: what each purchase cost, append-only too
  remote.key      # the age key the remote is encrypted to; never in the remote
  seal.key        # the key seals are signed with; yours, and in no bundle or remote
  seals/          # the seals of the journal's head, and their timestamp proofs
  *.lock          # empty; what two writers of one file take turns on
```

The catalogs and the config are rewritten whole, so they are written to a
temporary file and renamed into place — a crash leaves the old file or the new
one, never an empty one — and every change to one is made under its lock, onto
the file as it stands: a product added in one terminal survives a rename made
in the TUI from a copy read before it.

The journal is only ever appended to, and each entry is chained to the one before
it with a SHA-256 hash, so an edit made outside Wits is detectable. Appending
takes an advisory file lock as well as a mutex, because it is a read-then-write
and two processes reading the same tip would fork the chain. The checks an entry
must pass — enough in the jar, not already undone — run inside that lock against
the journal as it stands, so the TUI and a command in another terminal cannot
both take the same grams. A write cut off before its newline — a crash, a full
disk — is left out when reading, and cut away before the next entry is appended.

The chain is checked every time a repository opens. If the journal was changed
outside Wits, `wits status` and the interface say so, and `wits fsck` says
where — along with any entry that took grams that were not in the jar at the
time, any entry naming a product or device the catalogs lack, and whether every
gram that came in is in the jars, through a vaporizer, or went out:

```console
$ wits fsck
1429 entries, 53 products, 1 device
1254.00 g came in, 0.00 g went out; 1254.00 g in the jars — every gram accounted for
✓ the chain verifies, and nothing is out of place
```

The directory is created `0700` and its files `0600`. Nothing is transmitted
anywhere unless you ask. `wits push` writes to a folder you name, encrypted, and
what carries the folder further is yours to choose. `wits seal --timestamp`
is the one network call wits makes, and only when asked. It sends one SHA-256,
the seal's hash and nothing else, to the public OpenTimestamps calendars.

## Bundles

`wits bundle` writes the catalogs and every entry to a single file that
`wits restore` reads back, reproducing the journal **exactly, hash chain
included**. That is what makes it worth trusting as a backup. A restore writes
every entry in one go and checks the chain it made; a bundle that does not
restore whole restores nothing. Bundles written before v0.41.0 left out the
accounts of undos and weighings; they are recovered when such a bundle is
restored, and one that cannot be recovered is refused rather than restored
wrong. The effect log, the price log and the seals with their timestamps
travel in the bundle too, after the entries, and each product's terpenes with it
(format 6; an older build refuses one, rather than dropping what it cannot
read).

It is plain text, so the record stays legible with nothing but a text editor and
diffs cleanly in git. Small, too, because most of what the journal stores is
derivable and is left out: sequence numbers, account pairs and the whole hash
chain are recomputed on restore.

Three years of real history, 1429 entries across 53 products:

| | bytes | |
| --- | ---: | --- |
| journal | 528,321 | |
| **bundle** | **29,417** | **18×** |
| bundle, `--gzip` | 6,693 | 79× |

![Bundling and restoring](./assets/wits-bundle.gif)

## More than one device

`wits push` and `wits pull` keep one repository on several devices through a
folder they can all reach: a Syncthing share, a USB stick, anything that
carries files. The folder holds one file, `wits.age`: the repository as a
compressed bundle, encrypted with [age](https://age-encryption.org) to a key
in `.wits/remote.key`. The key never goes in the folder. Bring it to each other
device yourself, and pass it to `wits remote add --key` there. The whole
workbook, 1429 entries, is 6,911 bytes in the folder.

```console
$ wits remote add ~/Sync/wits
Remote set to ~/Sync/wits. Its key is ~/wits/.wits/remote.key: bring it to every other device, and nowhere else.
$ wits push
Pushed 1429 entries and 0 ratings.
$ wits pull   # on the phone
Pulled 1429 entries and 0 ratings.
Took the remote's 53 products and 0 devices.
$ wits push   # on the deck, after both recorded a grind
🚨 the remote holds entries this device does not: wits pull first
$ wits pull
Pulled 1 entry and 0 ratings.
Chained 1 entry of this device's on top, with new hashes.
$ wits push
Pushed 1 entry and 0 ratings.
```

A pull works the way `git rebase` does. The entries both devices share are
kept, the remote's are taken as they are, and this device's own are chained
again on top of them. Those get new hashes, since a hash names a place in the
chain, and undos and ratings are pointed at the new ones. The journal as it
was is kept as `.wits/journal.ndjson.orig`. A push is refused until the
device holds everything on the remote, so nothing another device wrote is
written over. Where the remote holds a product or a device differently, the
remote's version stands.

Two devices can each record the same last grams of a jar. Both entries are
kept as written, and `wits pull`, `wits status` and `wits fsck` name the one
that took grams that weren't there. The jar takes nothing more except an undo
until one of the two is reverted with `wits revert`. Every other jar records
as usual.

## For the prescriber

`wits report` writes one page for an appointment, as a PDF. It covers the days
asked for (the last 90 unless `--from` and `--to` say otherwise): grams a day,
with grinding and per calendar day, the weeks as bars, which strains, what the
sessions did by the effect log, each mean with the count it stands on, and
today's fill and shelf with the date the next prescription is due. The footer
names the head of the journal's hash chain. The browser's History page
downloads the same file, and a phone hands it over to share.

```console
$ wits report --from 2026-08-01 --to 2026-10-24
Wrote a report for 2026-08-01 to 2026-10-24 to wits-report-2026-10-24.pdf
```

![The report, from the workbook](./assets/wits-report.png)

Its figures are the range's own, the same `wits export --from --to` gives:
67.56 g ground on 33 of the 85 days. The PDF is written by wits itself, in the
Helvetica every reader has, and this one is 5,004 bytes.

## A chain others can check

The journal is hash-chained, so its last hash fixes everything before it.
`wits seal` signs that hash with a key of your own (Ed25519, made the first
time in `.wits/seal.key`). `--timestamp` has the OpenTimestamps calendars
commit the seal's hash into Bitcoin, which proves it existed by a time nobody
can move. A doctor or an authority can then check a report against the journal
without trusting the patient or wits: the report's footer names the seal and
the key's fingerprint, `wits verify` checks the seal, and `ots verify` or
opentimestamps.org check the `.ots` proof with no wits at all.

```console
$ wits seal --timestamp
Made a sealing key in .wits/seal.key; its fingerprint is SHA256:t3GaiTSwisPSicN8ccsjAcVflyo7R9oxCglLYSluQQM
Sealed the journal's 1429 entries, head 1639d82, as 2026-10-24-1639d829b7e8.seal
Sent the seal's SHA-256, and nothing else, to 4 calendars; the proof is 2026-10-24-1639d829b7e8.ots, pending until it is in a block.
$ wits verify .wits/seals/2026-10-24-1639d829b7e8.seal
✓ signed by SHA256:t3GaiTSwisPSicN8ccsjAcVflyo7R9oxCglLYSluQQM on 24 October 2026, 22:13
✓ the journal here holds what was sealed: 1429 entries, up to 1639d82
· timestamp pending at 4 calendars; `wits seal --upgrade` asks for it again
```

A seal is six lines of text: the head, how many entries it closes, when, the
public key, and the signature over the lines before it. Hand out the key's
fingerprint (`wits seal --key`) once, to the person who checks, and never the
key file. A proof is pending for a few hours until a calendar's commitment is
in a block; `wits seal --upgrade` then fetches it. `wits report --seal` seals
the head before writing the page, so the page names the seal it can be
checked against. Seals travel in bundles and through push and pull, and fsck
checks every one: its signature, its head in the journal, and its proof.

## What I carry

In Germany a patient must be able to show that what they carry is what was
prescribed. `wits carry` lists what is held, each product sealed, ground and
already vaped, against the fills that dispensed it. It signs that with the
sealing key, as a statement that names the journal's head, and writes it as a
QR code:

```console
$ wits carry --out carry.svg
Held on 25 October 2026, 00:07, by the journal's 1429 entries up to 1639d82:

PRODUCT                            SEALED  GROUND  AVB    DISPENSED
Sumo 25/1 Papaya Bomb              12.94g  7.06g   0.00g  20.00g on 2026-08-29
All Nations 29/1 Mac Doughnut      12.42g  7.58g   0.00g  20.00g on 2026-08-29
NICE 32/1 Morning Sun              11.37g  8.63g   0.00g  20.00g on 2026-08-29
420 Evolution 27/1 Ice Cream Cake  5.67g   14.33g  0.00g  20.00g on 2026-07-09
Enua 36/1 Citrus Slap              2.40g   17.60g  0.00g  20.00g on 2026-07-09

100.00g in all. Signed by SHA256:t3GaiTSwisPSicN8ccsjAcVflyo7R9oxCglLYSluQQM; the QR code opens it at https://thedondope.github.io/wits/verify.html
```

Scanned with any phone, the code opens a page that checks the signature in
that browser and lists the holdings. The statement rides after the `#`, which
a browser never sends to the server, so GitHub never sees it, and the page
stores and sends nothing. It is plain HTML, uses the browser's own Ed25519
where it has one and its own where not, and works offline once the app is
installed. The browser shows the same code under **What I carry** on the Jars
page. Only the products on the shelf are listed unless `--all` says every jar
holding anything: a stash the record never worked down (the imported history
has no sessions) is the ledger's, not what is in a pocket.

The signature says who signed it. Whose key that is, the person checking
learns from the fingerprint the patient gave their doctor, or the one on a
sealed report. A phone makes a key of its own the first time, so its
fingerprint differs from the computer's until the computer's
`.wits/seal.key` is brought to it.

## What it costs

A price goes beside the journal, in `.wits/prices.ndjson`, because a purchase
is never edited and a price moves no grams. Give it when buying, afterwards, or
in one sitting for every purchase still in storage without one:

```console
$ wits price 52b8 "180,00 €" --note "the label said 180"
[ea22d7e] 180.00 € for 20.00g of sumo-papaya-bomb-251 on 2026-08-29, 9.00 € a gram
$ wits reconcile purchases          # one question a purchase; --all for every one
$ wits costs
520.00 € for 60.00g with a price, 8.67 € a gram; 85 purchases without one

FILL  DISPENSED   GRAMS   PAID      A GRAM
30    2026-08-29  60.00g  520.00 €  8.67 €
29    2026-07-09  60.00g  -         -       3 purchases without a price
…
1 session that helped (4 or more overall), 3.40 € each on average.
```

These figures are the ones from pricing the workbook's fill 30 by hand at 170 €
and then 180 € a jar; the workbook itself has no prices. A purchase priced
again costs what it was priced at last, and the log keeps both. Only purchases
with a price count. How many have none is said, not guessed at. A session that
helped is one rated 4 or more overall, at its product's price per gram. The
same walk is `$` in the terminal interface and the **Costs** page in the
browser.

## AVB

Already vaped bud is what is left of a session once it has been through a
device. When a device is emptied, weigh what comes out and record it:

```console
$ wits avb collect 0.21
[d3dd09d] collected 0.21g of AVB from 0.75g in 3 sessions: 28% back
$ wits avb
0.21g in the AVB jar

COLLECTED   DEVICE               AVB    FROM                 YIELD
2026-10-24  storz-bickel-mighty  0.21g  0.75g in 3 sessions  28%
$ wits avb use 0.1 --note tincture
[885418c] used 0.10g of AVB, 0.11g left in the jar
```

What came out came from the sessions put through that device since it was last
emptied, and it is shared between their products in proportion to what each
put in: here, 0.08 g to the Morning Sun's 0.30 g and 0.13 g to the Papaya
Bomb's 0.45 g. More cannot come out than went in, and a device cannot be
emptied twice without a session between. `--device` can be left out when only
one device has sessions waiting. `avb use` draws the jar down as a whole, each
product's share in proportion to what it holds. What stays behind in
**consumed**, here 0.54 g, is what the vapour took. The browser records both
on the **AVB** tab of Record (`v`), and its AVB tile shows the last yield.

## Temperatures

Wits knows the boiling point of every cannabinoid and terpene, so a number on a
dial reads as what it actually does — including the point at which it starts
producing benzene.

```console
$ wits temps 210
COMPOUND         BOILS AT  EFFECTS
THCA             120°C     anti-inflammatory, anti-epileptic, anti-proliferic
Nerolidol        122°C     sedative, anti-fungal, anti-malarial, anti-parasitic, skin penetration enhancer
CBDA             130°C     anti-inflammatory, anti-proliferic
β-Caryophyllene  130°C     anti-inflammatory, anti-malarial, cytoprotective, analgesic
…
⚠️  210°C is at or above the 205°C boiling point of Benzene.
```

The devices screen shows the same for whichever device is selected, at its
default setting, and `a` adds a device there.

## In a browser

`wits-server` serves the repository it is started in, and the interface built
from `wits-ui/` beside it:

```sh
make serve          # builds both, then serves the repository here
open http://127.0.0.1:7341
```

The dashboard draws the fill as a ring — sealed, ground, AVB — with the older
jars around it as a halo; each product of the fill as a jar of glass, filled to
what it holds; and the shelf's supply as a curve to the day it runs out. The
history is three years of grinding as a calendar, and every fill as a lane on
one timeline, overlapping the way fills do. The journal scrolls all of its
entries and undoes one with a correction, the jars are a gallery, and **Record**
grinds, records a session, a fill or a weighing — through the same recorder as
the commands, with the same refusals in the same words. The interface follows
the repository: an entry made with the command line shows within a few seconds.

A fill can be read off the pharmacy label: **Read the label** on the Fill tab
takes a photograph and fills in the product and the grams, matched to a jar you
already have when it is one. The text is read in the browser, by Tesseract
compiled to WebAssembly and served by wits itself, so the photograph goes
nowhere — check what it read, then record.

On a phone the tabs give way to a dock under the thumb: Dashboard, Journal, a
raised **Record**, Jars, and **More** for the other pages. The top bar names
the page you are on, and the bars keep clear of the notch and the home
indicator. It is laid out for an iPhone 11 Pro held upright (375 px) and
nothing narrower; no page scrolls sideways. There, Analysis's scopes go by
short names (Fill, 30 d, 90 d, 12 mo, All) and the jars are a compact list.

The palette in the header picks one of 21 themes, and `t` steps through them
(`T` back): wits' own Aurora, then Nord, Dracula, Gruvbox, Solarized, Tokyo
Night, Mocha, Rosé Pine, Phosphor, Synthwave, Void, Forest, Ember and Abyss in
the dark, and Paper, Solarized Light, Latte, Sakura, Bauhaus, Daylight and Dune
in the light. They differ in more than colour — Paper is cream and serif with
square corners, Phosphor a green CRT in monospace with scanlines, Bauhaus hard
black edges and offset shadows on a grid, Mocha and Latte rounded and pastel.
Each keeps the account colours' meaning — storage blue, stash green, AVB amber —
in its own shades. The choice is this browser's, kept in its local storage;
`?theme=paper` opens the page in one without keeping it.

![The dashboard](./assets/wits-ui-dashboard.png)

![Every day, and every fill](./assets/wits-ui-history.png)

**Analysis** is the terminal's long view — this fill, the last 30 or 90 days,
12 months, all time — and the record played back through it. Press play and
everything replays from empty, an entry a moment: the daily chart fills in, the
products race each other up the ranking, the fills grow on their timeline; drag
the scrubber to any point, step with `←`/`→`. The jars replay the same way,
filling as fills arrive and draining as they are ground. `?at=2025-02-14`
opens either as the record stood that day.

![The record played back to February 2025](./assets/wits-ui-analysis.png)

The analysis tab also draws **tolerance**: grams a day against how the sessions
went, both over the same 14 days rolling. Tolerance creeps when the grams climb
and the rating stays flat. Breaks, a week or more with nothing ground and no
session, are shaded behind the curve, and the replay draws it day by day, so you
can see whether it comes back down after one. A week, not three days: the
workbook grinds every two or three days, and at three days its last 90 held
eleven "breaks" that were only the habit. At a week it holds one, 12 to 18
September, and ten over the whole record, the longest 99 days.

**Film** on the Jars page records the replay as a video: three years of the
shelf in thirty seconds, from the same replay the page plays, each jar sealed
blue under ground green, paced evenly through time rather than entry by entry.
It is WebM, MP4 on Safari, recorded by the browser itself, and a phone hands it
over to share.

![The shelf, three years in thirty seconds](./assets/wits-film.gif)

On the jars, tick the cards — or **Select all** — and **Clear stash** records
their stashes as consumed, one adjustment each in a single write: the
terminal's history clean-up, for the stashes you choose.

**Effects** is the effect log. Each session waits to be rated — 1 to 5, tapped,
on how it went overall and on pain, sleep, calm, mood, focus or appetite, any of
them — and what the ratings add up to is drawn three ways: every product against
every dimension, each dimension by temperature band, and by time of day as a
wheel. Every mean says how many ratings it stands on, and one that stands on a
single rating is hatched: a rating, not a finding. `wits rate last sleep=5`
does the same from the command line, and `e` on the terminal interface's
sessions screen.

![What works, by product, temperature and time of day](./assets/wits-ui-effects.png)

Under it is **a map of the strains**: every product dispensed, across by THC,
up and down by its name and terpenes so that alike sits at one height, sized by
what was dispensed and coloured by how its sessions went. Pick one for what is
most like it; `wits like` says the same on the command line:

```console
$ wits like cantourage-mac1-251
Like Cantourage 25/1 MAC1+:
  420 Evolution 25/1 CA MAC: MAC1
  Cantourage 25/1 MAC 1+
  Barongo 27/1 MAC3
```

The workbook names its cultivars but lists no terpenes, so for now the map
knows the Wedding Cakes and the MAC1s by name and the rest by THC. Write a
profile down as the label gives it, `wits describe wedding --terpenes "Myrcen,
Limonen, Caryophyllen"` or the terminal's product form, and products alike in
terpenes find each other whatever they are called.

Every key the terminal has is here too, and the footer shows the ones the page
answers: `g` `s` `b` `r` `v` to record, `1`–`7` or `h`/`l` between pages, `p` to
replay, `j`/`k` to move, `space` to tick, and `?` for all of them.

It answers this machine only. The server listens on a loopback address and
refuses one that is not; a request naming any other host is refused, which keeps
a web page elsewhere from reaching it; and every write must be JSON, which a page
on another origin cannot send. Every font and icon ships with the interface, so
it makes no network call either. `?record=grind` (or `session`, `fill`, `weigh`)
opens the form on that tab — a link to bookmark.

## On the phone

The same interface runs on a phone, with wits itself inside it: the
repository, the recorder and the API that `wits-server` runs, compiled to
WebAssembly. The phone keeps its own copy of the journal in the browser and
refuses the same things in the same words as the terminal. It works offline
once it has loaded, and a home-screen shortcut opens the grind form directly.

![The phone, after its first sync](./assets/wits-phone-dashboard.png)

It's served from GitHub Pages at <https://thedondope.github.io/wits/>, and
holds no data there. Install it from the browser's menu (**Add to home
screen**). The phone syncs through the same folder as `wits push` and
`wits pull`, by hand:

1. On the device that made the folder, show the key: `cat .wits/remote.key`.
2. On the phone, open **Sync**, open `wits.age` from the shared folder (on
   Android, the folder Syncthing keeps), and paste the key the first time.
3. **Sync** pulls the folder's entries in, chains the phone's own on top, and
   pushes. **Save wits.age** then hands the result to the Files or Syncthing
   app to save over the one in the folder.

![Sync, on the phone](./assets/wits-phone-sync.png)

A browser on a phone can read a file you pick, but it can't write into a
folder by itself. That's why the result is shared or downloaded, and why you
save it over the folder's copy yourself. The key and the journal never leave
the phone except inside that encrypted file. `make pages` builds what Pages
serves, and `make wasm-check` syncs the engine with the command line both ways.

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
    api/                  the repository over HTTP, for wits-server
    tui/                  the screens
    version/              the build stamp every binary reports from
  wits-ui/                the web interface: Angular and Angular Material
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
