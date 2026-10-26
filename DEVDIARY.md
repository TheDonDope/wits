# Development Diary

## 2026-10-26

### Every jar opened, a year to look around in, and a window of its own

The dashboard shows every fill not yet emptied now, not only the newest: on the workbook, fill 29's three jars stand under fill 30's, holding the 8.07 g the "older jars" line had always named. A jar on the dashboard or the shelf opens in full, with its fills and pickups and what they cost, its months, its sessions by device and rating, and its terpenes. When none are written down, wits looks them up: a web search for the product as the pharmacy sells it, and Leafly for the strain. On the first four tried, each came back with a profile from two to seven pages. On the phone the lookup failed with a bare 504, because a page may read no other site; it asks through Jina's Reader now, and found Lemon Tartz on six pages in 1.7 s in the Pages build. Seed with demo data, on the menu, opens a repository with something on every page: the workbook, 32 fills priced from the invoices, 15 devices, 897 sessions over a year with 718 rated, terpenes for 50 of 53 jars, and the 48 stashes left behind cleared, 2617 events restored beside your own. Last, the AppImage stopped handing the interface to a browser tab and opens it in an Electron window of its own, 130 MB where it was 22, checked by opening it. Shipped as v0.93.0 through v0.96.0, and the README's browser section was rewritten around new screenshots on the way.

- `7df80bd` feat(wits-ui): every open fill on the dashboard, every jar opens in full, and its terpenes looked up
- `d3c6112` fix(wits-ui): splitTerpenes after the component, where Codacy cannot count the template as its body
- `125474f` docs: update changelog for v0.93.0
- `47cefe0` fix(research): the phone looks terpenes up through a reader, since its page may read no other site
- `0424571` test(research): the stand-in reader answers from the stand-in sites, asking nothing itself
- `e20c882` docs: update changelog for v0.94.0
- `a5963f2` feat(demo): seed with demo data — a year of it, priced, rated and looked up, on the menu
- `e6c0f36` refactor(demo): the generator's choices in small named steps, which Codacy can count
- `b77be31` docs: update changelog for v0.95.0
- `f476184` feat(appimage): wits in a window of its own, closed to quit
- `f753c1e` refactor(desktop): the window's shell as an ES module, its browser's environment filtered
- `5facd06` docs: update changelog for v0.96.0

### Notes for 2026-10-26

Brave and DuckDuckGo both turn a client away now and then: DuckDuckGo with a page asking to prove a person is searching, which has no results and passes to the next search, and after a day of testing, Brave with 429 and DuckDuckGo with 403 for a while. The lookup retries in the demo's generator, not in the app, where a failed jar just says so. Three products found no profile anywhere: Amsterdam Amnesia, Super Citra G. and CA GG. On the phone every lookup goes through Jina's Reader, a third party that sees the product's and strain's names; its free tier takes twenty requests a minute, which is why Research all there waits nine seconds a jar. The demo bundle is public and carries the three pharmacies' names and every fill's price, taken from invoices that themselves stay out of git; the invoices' numbers were kept out of it. Its sessions come from a seeded generator, but the generator was refactored after the bundle was made, so `make demo-seed` now draws different sessions, and the terpenes are the web's on the day; the committed bundle is not what a rerun gives. Electron is pinned to 44.7.0 rather than 45.0.0, which was five days old. npm 12 runs no install scripts unasked, so the build runs Electron's itself. Chromium here needs NSS, NSPR and ALSA, which SteamOS lacks, so the AppImage carries them, and `make appimage` on this machine wants `eval "$(./doctor.sh env)"` first for the build to find them. Chromium headless segfaulted before loading, so the window check uses the display or `xvfb-run`. Codacy misread short TypeScript functions three times — a helper before a component's decorator, nested ternaries, an object of ternaries — and each was rewritten plainly rather than argued with. A `pkill` matching its own command line killed the shell twice more; a server is stopped by its PID now. The merge of #160 was refused by the auto-mode permission check, and the maintainer merged it by hand. The entry for 2026-10-25 stops at v0.90.0: v0.91.0 (a price's pharmacy in the bundle) and v0.92.0 (the devices) came from another session and are not written up there.

## 2026-10-25

### What a gram cost, and one file to run it

With every fill priced from its invoices, the costs could finally say something. Insights → Costs now shows every purchase's price a gram over time, a line through each fill's, each strain in a table that opens to its every purchase, the pharmacies side by side, strength against price, and the years. A strain is the cultivar under every name it was sold by (`prices.StrainOf`). MAC1 comes under five names from two producers, and MAC 1 stays apart from MAC 3. A price now remembers the pharmacy it was paid to (`Price.Seller`), read off the invoice. On the workbook it comes to 10,156.01 € for 1,244 g, and the years add up to it. A gram went from 10.73 € in 2023 to 6.25 € in 2026, cost 10.38 € at one pharmacy against 6.74 € and 6.52 € at the other two, and stronger flower cost no more. Then wits became one file. The AppImage holds the server, the command line and the interface. It serves `~/.local/share/wits`, made on the first launch, opens the browser, and stops when its terminal window closes. A release workflow builds it for every published release, checks it by running it, and attaches it with its `SHA256SUMS`. It worked on its first run, for v0.90.0. Shipped as v0.89.0 and v0.90.0.

- `cc0b554` feat(wits-ui): what a gram cost, by strain, pharmacy, strength and year
- `0abca28` docs(roadmap): one blank line between paragraphs
- `09eabd2` docs: update changelog for v0.89.0
- `2569703` feat(release): wits as an AppImage, built and attached to every release
- `926bc53` docs: update changelog for v0.90.0

### Thirty-seven receipts, and four doors

Late in the evening, the maintainer turned up every invoice still to be found, and they became the test. Of the first 21, from two pharmacies, the reader took one whole. The one that ships prints a product over three rows under its PZN. The other changed its header layout twice. And invoices rarely name a strain the way the jar does: breeder for distributor, cut short, 25:01 for 25/1. All of it reads now. A vape cartridge or the shipping is left out and named, so every invoice adds up to its total, and a line is matched to the fill's own products by the words, strength and grams they share. Then the tabs, eight of them, became four. Dashboard. I forgot to, which gathers the sessions to rate, pickups to price, stashes left behind and AVB to collect, and counts them. Shelf. Insights. On the workbook it counted 50, since the workbook logs no sessions and 48 jars sat empty with 1,154 g in their stashes. Pricing then walked newest first, took invoices by the folderful, and learned to stop asking about a pickup with no price to be had (`Price.Unknown`, written so the records before it hash as they did). Sixteen more invoices from a third pharmacy priced the workbook's first fills. One, ordered 2 July 2024, was fill 10 to the gram but picked up the 15th, so the window went from 7 days to 14. All 37 on a fresh import: read in 3.3 seconds, all 30 fills priced, 10,156.01 € for 1,244 g, the same to the cent as their lines added up apart. The journal's checksum didn't change. Shipped as v0.86.0 through v0.88.0.

- `2d2413a` feat(invoice): both pharmacies' invoices read, and each finds its fill by what it shares
- `823140e` docs: update changelog for v0.86.0
- `7b56915` feat(wits-ui): four tabs, and one of them for everything forgotten
- `90bd72b` docs: a list stands off the paragraph before it
- `29554d1` docs: update changelog for v0.87.0
- `dc1983f` feat(wits-ui): every invoice at once, and a pickup with no price asked about no more
- `4d99070` feat(invoice): a third pharmacy's invoices, and a fill two weeks after its order
- `e0a6b48` docs: update changelog for v0.88.0
- `6ad12cc` test(tui): a reload is read at once in the tests, so a slow runner cannot drop it

### Forty-one coats, and three strains to a receipt

The evening finished what the afternoon left half done, then went back to the paperwork. A second review cleared what the first missed. The fill form had worked a total in euros while the server worked cents, so in 33 of 21,015 cases the price shown was not the one written: 30 g at 2.05 € with VAT showed 73.18 € and wrote 73.19 €. Both sides now work it the same way, with a test on each. A refused fill no longer leaves a product behind in the catalog. Repository became a tab, since nobody found it as a chip in the header. The page now names its build, so a phone's service worker serving an old copy shows it. The phone got a Repository tab of its own and its own repositories, importing the workbook as 1,429 entries, the same as `wits import`; the spreadsheet reader costs its engine 450 KB gzipped. Then twenty more themes, forty-one in all, several drawing their own paper in CSS: Blueprint's grid in three line weights, greenbar ledger paper with tractor holes, a gold sunburst, light through venetian blinds, and a Bauhaus Dark that casts red shadows. Every one was photographed at 1440 px and at 375 px, where none is wider than the screen. Last, Costs prices a prescription as it was picked up: 88 unpriced purchases in 30 fills, a price a gram per strain with 19% VAT on top, or the pharmacy's invoice scanned on the device with its first page shown beside the prices it filled. An invoice is matched to the pickup within a week that holds its strains, so it is never recorded twice. The real one is dated 28 August, fill 30 was picked up on the 29th, and it asks before pricing it at 351.80 €, the invoice's own total to the cent. Shipped as v0.81.0 through v0.85.0.

- `56a7952` fix(record): a refused fill leaves no product behind, and a purchase is a fill of one
- `9de3871` chore: what the second review found, cleared
- `c67d317` fix(wits-ui): the Repository page is a tab, where it can be found
- `be43c5f` docs: update changelog for v0.81.0
- `40e367f` docs(roadmap): eleven things the record could still say
- `eb94b35` feat(wits-ui): the page says which build it is
- `b8752f4` docs: update changelog for v0.82.0
- `4dcf20f` feat(wits-ui): the phone has a Repository tab, and its own repositories
- `66065ba` docs: update changelog for v0.83.0
- `641bd8e` feat(wits-ui): twenty more themes, some of them drawn on paper
- `c745512` docs: update changelog for v0.84.0
- `329d767` feat(wits-ui): a fill is priced as it was picked up, from its invoice if you have it
- `4d623bd` docs: update changelog for v0.85.0

### Twenty-one looks, a thumb's reach, and the paperwork

The afternoon was the browser's. It got twenty-one themes, Aurora as it was and twenty more, differing in corners, glass, backdrop, shadow and type as much as colour. That meant routing some seventy colours written into the components through theme variables first; the film's canvas reads the theme off the page, and Aurora was checked against main's screenshot. Then a review round on an iPhone 11 Pro held upright, 375 px, now the narrowest the interface is laid out for. The tabs there showed one icon at a time between paging arrows, so phones got a dock with Record raised in it and the rest under More. Analysis measured 503 px and Costs 404 at that width, and every page measures 375 now. Analysis's five scopes go by short names, the Jars page went from 12,900 px to 6,800 as a compact list, and the Record dialog's five tabs now fit side by side.

Then two things the terminal could do and the browser could not. The Repository page opens another repository or makes one, after asking, and imports a workbook uploaded from the browser: a dry run first, then into an empty journal only. Through the page, the committed workbook came in as 53 products and 1,429 entries, and a second import was refused. None of it is built for the phone, whose engine stays clear of the spreadsheet reader. Last, a fill holds several strains now, each with grams and a price a gram, recorded as one batch. A 19% VAT switch, on by default, adds Germany's tax to prices typed in. Pharmacy invoices are read as PDFs on the device by pdf.js, and the example's three strains matched their jars past "GM", "MOS" and "PAB". It came to 351.80 €, the invoice's total with VAT to the cent. Shipped as v0.75.0 through v0.80.0.

- `f6c8742` feat(wits-ui): twenty-one ways to look at the same grams
- `a105608` fix(wits-ui): the film's jar takes its box as one
- `376a395` fix(wits-ui): the film's jar takes its box by name
- `72e96f7` docs: update changelog for v0.75.0
- `c5d5c57` feat(wits-ui): a phone navigates by thumb, and nothing scrolls sideways
- `236c34d` fix(wits-ui): the page's path is split without a regex
- `3d3859b` docs: update changelog for v0.76.0
- `18447c0` feat(wits-ui): five scopes in a phone's width, and the shelf as a list
- `eb52512` docs: update changelog for v0.77.0
- `ffe6273` feat(api): the browser picks the repository, and brings the workbook
- `00744ea` fix(api): a folder is made absolute, not cleaned
- `9108326` docs: update changelog for v0.78.0
- `9d73183` feat(record): a fill of many strains, priced, and read off the invoices
- `5592065` docs: update changelog for v0.79.0
- `7a6b3aa` fix(wits-ui): the record dialog's five tabs, all in a phone's width
- `b5b844b` docs: update changelog for v0.80.0

### The label, the undo, and a house in order

Next came one of the roadmap's later delights: a photograph of the pharmacy label fills the fill. Tesseract reads it in the page, compiled to WebAssembly, its engine and its German served by wits itself, so a photograph of a prescription goes nowhere, and `catalog.ReadLabel` makes a product line and grams of the text, passing over the expiry date, the batch number and the pharmacy's house number. Every one of the workbook's 53 names, copied off labels to begin with, reads back to its own product, as written and in capitals. A rendered label read in 1.0 s in headless Chromium, every request, the worker's included, to the machine serving the page. Then a review of the 9,600 lines landed since the last one, every finding reproduced before anything was fixed. The worst: an undone grind still counted in the rate, the shelf and the refill date, everywhere but the export and the report. Buying 20 g, grinding 2 g on two days and undoing a 5 g grind said order by 26 October; it says 30 October now, settled once in the fold over the whole journal. The seal key could be made twice by two first uses at once, which in 7 of 20 rounds left a carry statement signed by a key that no longer existed, and restoring 1500 ratings took 21.2 s, a record and a flush at a time; it takes 0.10 s now that the effect log and the price log are one chained log, put back in one write. Last, the files: every command in the one named for it and the report's four concerns apart, moved by a tool rather than retyped, 221 of 225 declarations byte for byte. Shipped as v0.69.0 through v0.74.0.

- `4f18a95` test(tui): today's fixture is dated no later than now
- `d455bf9` feat(label): photograph the pharmacy label, and the fill fills itself
- `a2bfd5d` docs: update changelog for v0.69.0
- `0943992` fix(ledger): an undone entry counts in no total, wherever it is read
- `d34110d` docs: update changelog for v0.70.0
- `ee56c86` fix(seal): one key, however many ask for it first
- `ba296e9` docs: update changelog for v0.71.0
- `e511ec4` refactor(chain): one chained log for ratings and prices, put back in one write
- `21c168f` test(chain): date the record a test compares, so the zone cannot differ
- `66a5050` docs: update changelog for v0.72.0
- `e35be8d` docs: a new file needs its own package comment, not to be avoided
- `002aed8` refactor: a file for each command, and the report's four concerns apart
- `1df94cc` refactor(rate): the last session is the fold's, not a fourth undo map
- `672c346` fix(commands): put each new file's package comment where revive reads it
- `d30d40d` build(preflight): ask revive about a new file's package comment
- `d408fc3` docs: update changelog for v0.73.0
- `311e936` chore: what the review left behind, cleared
- `79c9c2c` chore(wits-ui): its go.mod names no Go version
- `636570e` docs: update changelog for v0.74.0

### Trust, a map, a film, and a cache that was not

Phase 4 came first: making the record checkable by someone who trusts neither the patient nor wits. `wits seal` signs the journal's head with an Ed25519 key of one's own. The head's hash covers every entry before it, so six lines of text fix the whole journal. `--timestamp` sends the seal's SHA-256, and nothing else, to the four public OpenTimestamps calendars, the one network call wits makes and only when asked; the proof format is written in-tree, and a real proof writes back byte for byte. Codacy refused the first push for a SHA-1 operation in it, and the calendars only use SHA-256, so it went. `wits carry` then signs what is held against the fills that dispensed it, as a QR code that opens a plain verify page checking the signature on the checker's own phone, the statement riding after the `#` where no server sees it. The headless Chromium here is 128, which has no Ed25519 in WebCrypto, so the page carries its own RFC 8032 as well; both paths accepted the workbook's statement and refused it with one gram changed. Deflating the link took the code from 117 modules to 89.

From "Later" came the map of the strains and the replay as a film. The workbook lists no terpenes, so the map places products by THC across and by their cultivar's words up and down. Building it found that bundles had never carried a product's terpenes; format 6 does. A profile not written down counts as halfway alike, or two unknown ones came out nearer than two that match. The film draws the replay on a canvas and lets the browser record it. MP4 failed part way in Chromium without its codecs, and VP9 fell behind and gave up twenty seconds in, so it is VP8, with the duration written into the header so players can seek. Last came the cached fold, measured before it was built. The fold is 0.63 ms of a 12.2 ms open, so what got cached is the read itself, in memory, while nothing changes. On the phone's engine the dashboard went from about 450 ms to 21. Shipped as v0.64.0 through v0.68.0.

- `dafb981` feat(seal): a chain others can check, signed and timestamped
- `cdfa1bd` fix(seal): refuse SHA-1 in a proof, and test the timestamp against a calendar
- `940ffb9` docs: update changelog for v0.64.0
- `632a294` feat(carry): what is held, against what was dispensed, as a signed QR code
- `e4bd38f` docs: update changelog for v0.65.0
- `35120e8` feat(strains): a map of the strains, and what each is like
- `44b760f` docs: update changelog for v0.66.0
- `1852a1f` feat(film): the replay as a film, three years of the shelf in thirty seconds
- `1970bb2` docs: update changelog for v0.67.0
- `d36e023` perf(server): read the repository once, while nothing changes
- `5d7a4b1` docs: update changelog for v0.68.0

### Notes for 2026-10-25

Grouping strains took three passes on the real catalog. The first lumped MAC 1 with MAC 3. The second missed three products whose names end in their strength, where the producer field had swallowed the strain ("All Nations Mac Doughnut GM"). The third fixed that by keying them by name, less the words of producers the catalog knows. Pharmacies on a chart are told apart by shape as well as colour, since no one palette can be checked across 41 themes; in Aurora, one pharmacy's violet was too close to another's until it took the theme's amber. The AppImage pins `appimagetool` 1.9.1 and the type-2 runtime 20251108 to their SHA-256s. That `appimagetool` calls itself a "continuous build" when it runs, but its checksum is the 1.9.1 release's. In extract-and-run mode the AppImage runtime passes no signal on, so `./appimage.sh check` starts the image in a session of its own and stops the whole group. A first version of the check left a server running, which made the next run fail. The published AppImage was downloaded again, matched its `SHA256SUMS`, and passed the check here. Left open: the costs leave out shipping and narcotics fees, `wits costs` on the command line doesn't show the breakdowns, and the AppImage is x86_64 only.

The invoices (`assets/invoices/`, 37 PDFs) name their patient, so the folder went into `.git/info/exclude` before anything else. The tests use their rows with made-up people. Every check ran on throwaway copies of the workbook import; the maintainer's own repository has no prices written and its 48 stashes are not cleared. DevTools can't fill a folder input the way a folder pick does, so **Pick a folder** has only been tried through the reader it shares with **Add invoices**. The CLI in `bin/` was older than the server once and read a "no price" record as 0 €. Any build from before v0.88.0, such as a phone's cached page, will do the same, so reload it. Weighing is offered on I forgot to but not counted: a weighing that agrees writes nothing, and a reminder that can't be cleared would only nag. A "weighed, it agreed" entry would fix that, and is left open. So are the two December 2024 invoices that look like one order issued twice, and the 10 g Ghost Train Haze in fill 18 that no invoice lists. Codacy refused one thing all evening, a Markdown list with no blank line above it. This entry's own PR failed CI in `TestRateFromTheSessionsScreen`, which nothing tonight touched. The test helpers give up on a command after 50 ms, to skip timers that block on purpose, and a reload under the race detector on a busy runner took longer. Slowing every reload to 80 ms reproduced it. The tests now read a reload at once (`reloadAtOnce`), the program still reloads in the background, and the suite got faster, 9.3 s to 7.3 s. Test-only, so it merged without a release.

The invoice tests in the browser ran on the real invoice, which names its patient. It is still kept out of git by `.git/info/exclude`, and the screenshots showing it stayed in the session's scratch folder. Driving the pages took a small Node script talking to Chromium's DevTools directly: `DOM.setFileInputFiles` puts a PDF in a file input, and `Emulation.setDeviceMetricsOverride` gives a 375 px phone. It lives outside the repository. A `pkill -f` matching its own command line killed the shell a third time; stopping a background server by its task works. Codacy would have flagged `matchInvoice` (complexity 19) and `priceFill` (54 lines), so each got a named helper (`fillMatch`, `fillPrices`) before the push. Left open: the week an invoice may lie from its pickup is a guess, set from one invoice a day off. The command line and the terminal still reconcile purchases one at a time, so the browser's walk by fill is no longer the same walk in all three. The theme menu now scrolls through forty-one, which works but is long.

The example invoice names its patient, so it stays out of the repository, kept off commits by `.git/info/exclude` on this machine; the tests use its rows without the person. Its line prices include VAT: they add up to its gross total, not its net. A strain read off an invoice is never taxed again, but whether the prices typed in by hand are quoted before VAT, as the switch assumes, is the maintainer's to confirm. pdf.js 6 needs `Promise.try`, which the Chromium here lacks and Safari has only from iOS 18.2, so it runs as its legacy build. A headless profile reused between runs kept the app's service worker, which served the old chunks and hid the fix for a while; the profile is wiped before each run now. Codacy flagged two things that were not what it took them for: lizard read the regex literal `/[?#]/` as the start of a 62-line function, and a security rule took `filepath.Clean` on a browsed path for a traversal guard. That guard is absent by design: the folder browser may look anywhere its user could, on a server that answers this machine only. Left open: the phone layouts were measured in Chromium's emulation, never on the iPhone itself, so the notch and home-bar insets are untested. The dashboard is 345 px wide at 320 px, below the width the layout is held to.

The label reader has only read labels rendered here, never a photograph of a real one; the next fill is its first test. The TUI's fixture tests failed every morning before eleven, on main too, because they dated an entry ten o'clock UTC today, which the recorder refuses until then. The rule that a new Go file gets flagged by Codacy turned out wrong: a file with a package comment of its own passes. Fourteen files then failed on the form, a comment a blank line above the package clause being no package comment at all, so preflight now asks revive itself, which `./doctor.sh install` fetches. Codacy read `go 1.26` in `wits-ui/go.mod` as a standard library in use and listed 47 advisories, so that file names no Go version. A force-push to fix a test on a branch was refused by the permission check, which is why `test(chain)` is a commit of its own. Left open: the Record dialog's tab body is 4 px wider than the dialog, on main before any of this, and `journal.Marshal` stays exported because other packages' tests use it.

Installing the reference OpenTimestamps JavaScript library to parse a proof independently was refused by the permission check, so the outside check left is `ots verify` on a proof once it is in a block (`wits seal --upgrade` fetches it, a few hours on). The QR code has not been read by a real phone's camera yet. A phone makes a sealing key of its own, so its fingerprint differs from the computer's until `.wits/seal.key` is brought over, and there is no way in the app to do that yet. Twice, a `pkill -f` matching its own command line killed the shell running it; a `timeout` around the server is the way here. A bulk find-and-replace in the API also rewrote the read inside the new snapshot function into a call to itself, which held its own lock and hung the race tests until it was stopped.

## 2026-10-24

### The records add up

The committed workbook caught up with the jars: it now runs to 2026-10-23, one more fill and four more months, with every closed jar cleaned to exactly zero. Every last thing the importer had flagged was then fixed in the spreadsheet itself instead of being explained away: 2025-03 moved to the year it is named for, the 0.01 g Ghost Train Haze in 2025-06 became the 10 g fill it was, and the 2.40 g logged against no product in 2025-10 is gone. `wits import` now reports nothing, and the test checks that nothing is reported. Every figure the tests check was re-read from the raw cells, separately from the importer: 53 products, 1429 entries, 1254.00 g dispensed, 1209.20 g ground, no balance negative and no cycle over 100%. A later re-save of the workbook was checked by importing both versions; they replay to the same ledger. The bundle figures were re-measured, and the bundle gif was re-recorded so the picture says what the README says. Shipped as v0.26.0 and v0.27.0.

The afternoon turned to everything around the records. All fifteen Dependabot alerts were in excelize — malformed workbooks that panic, hang or exhaust memory — and no tagged release carried the fixes, so excelize now comes from master as of 2026-10-20; the real workbook still imports to the same 53 products and 1429 entries, and GitHub closed all fifteen within minutes of v0.28.0. Then the living docs were checked claim by claim against the code: real `status`, `temps` and `reconcile` output back in the README, the commands and keys it never mentioned, a "13 of 47 cycles" from before the cycle rebuild replaced by the records' own overlapping cycles 25 to 27, and a gzipped bundle figure no single command reproduced replaced by what `--gzip` writes, 7,009 bytes. The loop got a doctor, `./doctor.sh`, which checks and installs everything development needs beyond Go, and project skills for the work that keeps coming back: shipping, the workbook, the tapes, and this diary. The workbook skill reads the cells without the importer, and matched every pinned figure on its first run. Shipped as v0.28.0 and v0.29.0.

- `89dfad6` chore(deps): bump codecov/codecov-action in the actions group
- `c34de80` chore(deps): bump the gomod-minor group across 1 directory with 4 updates
- `d379433` test(importer): reconcile against the workbook through october 2026
- `157efcf` test(importer): the records add up, so say so
- `dbde7cc` chore: update seed data
- `a3adfc8` docs: update changelog for v0.26.0
- `32140e9` chore(vhs): re-record the bundle gif against the october workbook
- `33c3827` docs: update changelog for v0.27.0
- `d6ddb95` docs(diary): the records add up
- `512baee` fix(deps): take excelize from master, where its fifteen advisories are fixed
- `3284274` docs: update changelog for v0.28.0
- `b2a5d7c` build: a doctor for the loop, and skills for the work that recurs
- `fc2baf7` docs: rewrite the guides against what the code says today
- `4969826` docs: the workbook reader's docstring, the way pydocstyle reads it
- `3009cc1` docs: keep the workbook reader's docstring to one line
- `88b2490` docs: update changelog for v0.29.0

### Notes for 2026-10-24

Recording on SteamOS needs help: the Chromium that vhs downloads lacks `libnss3`, `libnspr4` and `libasound`, which SteamOS does not ship. `./doctor.sh install` now unpacks Arch's `nss`, `nspr` and `alsa-lib` into `~/.cache/wits-dev/chromelibs`, and `render-vhs-tapes.sh` points vhs at them, with nothing installed system-wide. Preflight had been skipping lizard and shellcheck all along on this machine; it runs both now. Codacy runs pydocstyle with D212 and D213 both on, which contradict each other for any multi-line docstring, so Python here keeps its docstrings to one line. excelize is pinned to an untagged master commit and should move to v2.11.1 once that is tagged.

### Two reviews, and a window

The rest of the day went from v0.30.0 to v0.50.0. The Séance left the table, twenty editor themes arrived on `t`, and a notice that had sat in the footer forever learned to leave at the next key. Then an architecture review, its findings reproduced before anything was fixed: sessions could not be undone, `buy "Can"` made a third product out of two partial matches, one cut-off line locked the whole repository, a fill dated a month back joined the wrong cycle, and the TUI and a command in another terminal could both take the same jar, leaving storage at -17.10 g. Each was fixed where it lived — the checks now run under the journal's lock against the journal as it stands, the fold replays by date, the catalogs are written atomically under their own locks. A slow import turned out to be 1429 flushes to disk: 13.24 s became 0.25 s, and restore went from 13.04 s to 0.07 s the same way. `wits fsck` was built, the chain is checked on every open, and the ledger now adds in centigrams and refuses what a scale cannot read.

Taking the bundle reader apart for its complexity found a bug in its format: bundles never carried an adjustment's accounts, so every restored undo and weighing moved nothing — wcake came back at 16.90 g instead of 17.95 g, and restore reported success. Format 2 writes them; format 1 bundles are repaired on reading. Typing the three scopes — `Fill`, `Shelf`, `Jar` — made mixing them a compile error and found three mixes the eye had passed. A second review found a fill dated ahead counting as on the shelf, which would have recorded a false 2 g loss at the next weighing, and a journal screen taking 34 ms a frame to draw entries nobody could see, its selection off screen after twenty rows; it draws in 2.3 ms now. The overdraft model went too: grams are flower, not money, and nothing is ever below zero. Every figure was checked against the workbook throughout, and `wits status` read the same at every step that was not meant to change it.

The evening built what the roadmap had planned: `wits-server`, a JSON API that answers this machine only, and `wits-ui`, an Angular client with Angular Material. It draws the fill as a ring, each jar of glass filled to what it holds, the supply as a curve to its empty day, three years as a calendar and every fill as a lane on one timeline — and it reads 36.73 g of 60, day 57, 19 days' worth, what `wits status` says. It got the analysis tab and the replay back, with a scrubber, ticked jars with their stashes cleared in one batch, and every key the terminal has, in a footer drawn from the same list the keys are answered from.

- `ab921ef` docs(diary): the records add up, and so does what surrounds them
- `aa612bf` feat(tui)!: the séance is over
- `eeaf3b6` build(preflight): fail on new complexity, as the gate does, not on old
- `a915cf7` test(witsnap): the camera and the tap, tested at last
- `14bd5ce` docs: update changelog for v0.30.0
- `5d626a5` docs(skills): ship from a clean main, and stop when you cannot reach one
- `ebc7082` feat(tui): twenty editor themes, and t to try them on
- `e77bf66` docs: update changelog for v0.31.0
- `2bc40ea` fix(tui): a notice lasts until the next key, not forever
- `e774aa4` docs: update changelog for v0.32.0
- `593f1e6` fix: sessions come back, buy stops guessing, and a cut-off line is only a cut-off line
- `042dc43` docs: update changelog for v0.33.0
- `b966069` feat(ledger): a fill lands where its date puts it
- `78bac09` docs: update changelog for v0.34.0
- `192ad6a` fix(record): the guard rails hold under the lock, against the journal as it stands
- `32d1919` test(record): compare the undo's moment, not its time zone
- `c72bf93` docs: update changelog for v0.35.0
- `64a6d87` feat(import): say what it is doing, and do it in one write
- `4032407` chore(vhs): re-record wits-import.gif because the import now names its stages
- `2b6698d` docs: update changelog for v0.36.0
- `fc829ff` fix(repo): the catalogs keep each other's changes, and survive a crash
- `f3f5618` docs: update changelog for v0.37.0
- `a8a94d5` feat: wits fsck, a chain checked on every open, and restore in one write
- `371c213` chore(vhs): re-record wits-bundle.gif because restore now names its stages
- `b916408` docs: update changelog for v0.38.0
- `2441ca6` feat(tui): the screen keeps up, a fill waits for its day, and esc stays in
- `7737a4a` docs: update changelog for v0.39.0
- `22592b8` refactor(tui): one slot for the form in front
- `54eec6a` docs: update changelog for v0.40.0
- `94b04ad` fix(bundle): undos and weighings come back from a restore
- `cca3442` docs: update changelog for v0.41.0
- `c9da754` refactor(ledger): add in centigrams, and refuse what a scale cannot read
- `995922a` docs: update changelog for v0.42.0
- `01fddff` refactor(ledger): the three scopes are types, and the mixes they found are gone
- `e6f4e7c` chore(vhs): re-record four recordings because fills now say what they are worth
- `258adac` docs(ledger): one comment per scope type, each beginning with its name
- `00bb26e` docs: update changelog for v0.43.0
- `b764b63` fix(ledger): flower, not money — no overdrafts, no negative grams
- `62778c6` docs: update changelog for v0.44.0
- `25b536e` fix(ledger): what has not arrived is not on the shelf
- `6f2e90b` docs: what has not arrived is not on the shelf
- `91320db` docs: update changelog for v0.45.0
- `4be8f35` feat(fsck): say when an entry took grams that were not in the jar
- `2a8c3d9` docs: update changelog for v0.46.0
- `68a411e` perf(tui): the journal draws what fits, and keeps the selection in it
- `db78d09` docs: update changelog for v0.47.0
- `588407a` chore: no wits.log, and figures named for what they count
- `a11c011` docs: update changelog for v0.48.0
- `95f4d53` feat(server): wits-server, a JSON API over the workspace
- `f2d1057` feat(server): what the client draws — the shelf's projection, every day, every fill
- `ddfb167` feat(ui): wits in a browser — Angular, Angular Material, and charts the terminal could not draw
- `0d86147` docs: wits in a browser — make serve, the screenshots, and how to work on it
- `3f5494f` ci(ui): lint the client with its types, where the types are
- `1c1ada1` style(ui): the stylesheets to the standard Codacy's Stylelint applies
- `e74cc71` docs: update changelog for v0.49.0
- `f087664` feat(ui): the analysis tab and the replay, ticked jars, and every key
- `31ec296` docs: update changelog for v0.50.0

### Notes for the rest of 2026-10-24

Codacy's ESLint runs without a project's dependencies, so in the Angular client every `@angular/*` and d3 type is unresolved and its type-aware rules flag every use — a hundred findings on the first pull request, ninety-nine about nothing. `.codacy.yml` excludes `wits-ui` from Codacy's ESLint only; the client is linted with its types by the `wits-ui` workflow and by `./preflight.sh` (`ng lint` with typescript-eslint's strict type-checked rules, and Stylelint). `ng add @angular/material` linked Roboto and the icons from a font service, which would have been the only network call wits made; they ship as npm packages instead. The client is verified by photographing it with the headless Chromium the tapes use, `--force-prefers-reduced-motion` so the count-ups have landed; `?record=`, `?at=` and `?scope=` open a form, a day and a scope for the camera. Node here is v26; Angular 22 built and tested on it without complaint. The `?` help sheet and a real stash-clearing click are covered by tests, not yet by a photograph. Left open from the reviews: AVB tracking, which stays on the roadmap.

### One ledger, two pockets

The night took wits off the desk, with the plan drawn up first. The roadmap gained thirteen ideas and then an order, in five phases, each standing on the ones before. The effect log came first, since several of the ideas stand on it. A session is rated afterwards, 1 to 5, overall and on pain, sleep, calm, mood, focus or appetite, from the command line, the browser, or `e` on the terminal's sessions screen. The ratings live beside the journal rather than in it, because a rating moves no grams, and they keep the journal's chain and lock. `wits show` prints one entry in full, the way `git show` prints a commit. And entries dated in the future went: an entry is recorded once it has happened, which removed a second fold and an "arriving" line from every view. Then two smaller things shipped. `wits export` learned `--from` and `--to` for an appointment and `--site` for GitHub Pages, and on the way the per-fill "Ground during" was renamed for the scope it is, since fill 27 had read 74.77 g ground of 30 g bought. Then the six vite advisories were closed with an override, because Angular's build pins vite exactly.

Then phase 5 of the roadmap. `wits push` and `wits pull` keep one repository on several devices through a folder. The folder holds one age-encrypted bundle, and the key never goes in it. A pull is a rebase: shared entries kept, the remote's taken as they are, this device's own chained again on top, with undos and ratings following them to their new hashes. Two devices grinding a jar's last grams is a clash. Both entries are kept and named, and the jar takes nothing but an undo until one is reverted. Testing that found an old bug: an undo put back grams a short grind had never taken, so the fill read 1.5 g its jar did not hold. Last, the phone. The server's own API, compiled to WebAssembly, runs in the page on a twenty-call in-memory file system kept in IndexedDB, and it's served from GitHub Pages. On the live site, at phone size, it pulled the workbook's 1429 entries in 11 s and read what `wits status` reads: fill 30, day 57, 36.73 g of 60, 8.07 g in 2 older jars. Shipped as v0.51.0 through v0.58.0.

- `2205765` docs(diary): two reviews, and a window
- `2034d3e` docs(roadmap): what wits could become — thirteen ideas, planned
- `ce4d751` feat(effects): what the grams did, rated, and what that adds up to
- `ed0c5d3` docs: update changelog for v0.51.0
- `ef3006b` feat(tui): the sessions screen rates, a line a dimension
- `4b46bf3` docs: update changelog for v0.52.0
- `2115883` docs(roadmap): the plan has an order, and purchases get their prices
- `0514728` feat(show): one entry in full, the way git shows a commit
- `5168d8b` docs: update changelog for v0.53.0
- `104476d` fix(ledger)!: nothing is dated ahead, so there is one fold
- `5b9baf3` docs: update changelog for v0.54.0
- `142e3a2` feat(export): a range for the appointment, and a site for the pages
- `5ac8913` refactor(export): the index needs no session
- `2de1515` docs: update changelog for v0.55.0
- `fdc2112` fix(deps): take vite 8.3.5 under the angular build, where its six advisories are fixed
- `bc0727d` docs: update changelog for v0.56.0
- `5ecd8d1` feat(remote): push and pull through a folder, and a pull is a rebase
- `6818dae` docs: update changelog for v0.57.0
- `8de7a42` feat(phone): wits in the page, its own journal, synced by hand
- `199cec5` chore(journal): the browser's lock names its package
- `9ed06a5` docs: update changelog for v0.58.0

### Notes for the end of 2026-10-24

The six Dependabot alerts for vite were still open hours after v0.56.0, though main's lockfile holds only 8.3.5; GitHub had not re-scanned. The override in `wits-ui/package.json` comes out when Angular pins a fixed vite. Go's js port panics on any error code it has no errno for and reads anything but `null` as an error, so `data/memfs.ts` answers only with codes from `syscall/tables_js.go`. A first wasm build of 5.2 MB was a stub the linker had mostly dropped; the real engine is 8.9 MB, 2.4 MB gzipped, and swapping `net/http/httptest` out barely changed it. Node 26 runs the TypeScript MemFS directly for `wasm-check.sh`, so long as it uses no parameter properties. A plain folder still lets two simultaneous pushes overwrite each other; the losing device keeps its entries and rebases them on its next pull, and WebDAV with `If-Match` is planned to close that. The phone build has only been driven in headless Chromium, not yet installed on a real phone, and on Android "Save wits.age" shares or downloads rather than writing into the Syncthing folder.

### Phases 1 to 3, a release each

With the go-ahead to merge and release as each went green, the evening built the roadmap's first three phases. `wits refill` says when to order the next prescription: the supply projection's empty day less the pharmacy's lead time, in the command, the status line, the API and both dashboards, all from one function, so the dates cannot disagree. On the workbook that is empty on 17 November, order by the 14th. AVB tracking needed no format change after all, since the bundle already carried both entry types, so it shipped alone. A collection is shared between its sessions' products in exact centigrams, and its yield kept: 0.21 g from 0.75 g, 28%. Prices then made the one format change, a log beside the journal like the effect log, with "reconcile purchases" walking every unpriced fill in all three interfaces. The report for the doctor is a one-page PDF wits writes itself, no library, its figures the same as `wits export --from --to`: 67.56 g on 33 of 85 days. The tolerance curve rolls grams a day against the ratings on the Analysis tab. Its breaks were three days at first and marked eleven in 90 days, which was only the habit of grinding every two or three; at a week there is one. Shipped as v0.59.0 through v0.63.0.

- `690d5b7` feat(refill): order by the day the shelf runs out, less the pharmacy's days
- `55b075a` docs: update changelog for v0.59.0
- `ca8ac72` feat(avb): what comes out of the vaporizer, weighed, and what it gave back
- `4be6693` docs: update changelog for v0.60.0
- `0dd6670` feat(prices): what each fill cost, beside the journal, and reconcile purchases
- `4f9fbfc` docs: update changelog for v0.61.0
- `99f9ed8` feat(report): one page for the prescriber, as a PDF wits writes itself
- `6f9c87b` docs: update changelog for v0.62.0
- `6c7c303` feat(analysis): the tolerance curve, and the breaks it bends through
- `a6875a7` docs: update changelog for v0.63.0

### Notes for the last of 2026-10-24

Fixed along the way: on a phone the record dialog's side-by-side fields had never fit, because Material sizes a field's input at 180 px. The workbook has neither prices nor ratings, so the cost figures in the README come from pricing a copy by hand, and the curve's rating line waits for real ratings. No PDF renderer is installed here; pdf.js from npm, drawing into a canvas in the headless Chromium, was the way to look at the report. The terminal interface cannot record AVB or draw the tolerance curve yet.

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
