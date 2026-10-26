/**
 * The user guide: every page of the interface, what it is for, and the same
 * done in a terminal — the command line and the terminal interface `wits`
 * opens — so the browser is shown as one way into wits among three.
 *
 * Prose may carry <b>, <em>, <code> and <kbd>, rendered through Angular's
 * sanitiser. The terminal output quoted is what the demo (assets/demo-seed.wits)
 * printed when the pictures were taken, trimmed where marked with an ellipsis.
 */

/** A picture: a page in a browser, a phone, a dialog, or the terminal interface. */
export interface Shot {
  src: string;
  alt: string;
  caption: string;
  frame: 'browser' | 'phone' | 'dialog' | 'terminal';
  /** The address the browser frame shows. */
  url?: string;
  /** Its size in pixels, so the page keeps its place while it loads. */
  w: number;
  h: number;
}

/** One command, and what it printed. */
export interface Example {
  cmd: string;
  out?: string;
  /** A line under it, said in words. */
  note?: string;
}

/** A key of the terminal interface, and what it does there. */
export interface TuiKey {
  keys: string[];
  does: string;
}

export interface GuideSection {
  id: string;
  group: string;
  title: string;
  icon: string;
  /** The first paragraph, larger. */
  lead: string;
  body?: string[];
  steps?: string[];
  /** Paragraphs after the steps. */
  then?: string[];
  shots?: Shot[];
  /** Where the page is, for a button that opens it. */
  page?: { path: string; label: string };
  /** A tab of the Record dialog to open, in place of a page. */
  record?: { tab: number; label: string };
  /** A picture drawn by the guide itself. */
  diagram?: 'accounts' | 'scopes';
  cli?: Example[];
  tui?: TuiKey[];
  tuiShot?: Shot;
  tip?: string;
}

const web = (src: string, url: string, alt: string, caption: string): Shot => ({
  src: `guide/${src}.webp`, url, alt, caption, frame: 'browser', w: 1440, h: 900,
});
const term = (src: string, alt: string, caption: string, w = 1500, h = 900): Shot => ({
  src: `guide/${src}.webp`, alt, caption, frame: 'terminal', w, h,
});

export const GUIDE: readonly GuideSection[] = [
  {
    id: 'thinks',
    group: 'Start here',
    title: 'How wits thinks',
    icon: 'account_tree',
    lead: 'Wits is a <b>ledger</b>. Every gram a pharmacy hands you is followed from the jar to the vaporizer and out the other side, and nothing on any page is typed in: it is all worked out by replaying one append-only journal.',
    diagram: 'accounts',
    body: [
      'Grams move through four accounts — <b>storage</b> (sealed jars), the <b>stash</b> (ground, ready to use, one per product), <b>consumed</b>, and <b>AVB</b>, already vaped: what comes out of a vaporizer. The interface says “ground” and “already vaped”; the terminal keeps the short words. Every entry is a transfer between two of them, so the grams always add up: nothing is lost between the pharmacy and the ashes.',
      'Nothing is edited in place. A mistake is put right by recording a correction, the way <code>git revert</code> adds a commit rather than rewriting one, and the journal keeps both. Each entry is chained to the one before by a hash, so a change made behind wits’ back shows: the <b>verified</b> light in the header says the chain holds.',
      'A <b>prescription</b> — a <i>fill</i>, in the terminal — is one pickup. It opens with the purchase and closes when its own jars are empty, not when the next one arrives, so prescriptions overlap. Every figure says which of three things it counts:',
    ],
    steps: [
      '<span class="scope keep">the fill</span> one prescription’s own grams — “36.73 g left of this fill”.',
      '<span class="scope keep">the shelf</span> every jar standing today, whichever fill brought it.',
      '<span class="scope keep">the jar</span> one product over its whole life.',
    ],
    cli: [
      { cmd: 'wits fsck', out: `2617 entries, 53 products, 15 devices
1254.00 g came in, 977.35 g went out; 169.90 g in the jars, 106.75 g through a vaporizer — every gram accounted for
✓ the chain verifies, and nothing is out of place` },
    ],
    tip: 'The browser, the terminal interface and the command line all read and write the same repository through the same checks, so they refuse the same things in the same words. Use whichever is nearest.',
  },
  {
    id: 'start',
    group: 'Start here',
    title: 'A repository to work in',
    icon: 'folder_open',
    lead: 'Everything lives in a <code>.wits</code> folder, the way a git repository lives in <code>.git</code>. The <b>Repository</b> page, on the menu (⋮), opens one, makes one, or brings a tracking spreadsheet across.',
    steps: [
      'Open <b>Repository</b> from the menu (⋮) in the header.',
      'Browse to a folder. Those that hold a repository are marked <span class="scope keep">wits</span>; <b>Open</b> serves it from then on.',
      'Or make one, in an empty folder or a new one.',
      'Under <b>Import a workbook</b>, pick a tracking spreadsheet. It is read and reported first, and written only when you import — into an empty journal only, since a second import would double every gram.',
    ],
    then: [
      'Just looking? <b>Seed with demo data</b>, on the same menu, opens a repository with something on every page — three years of fills, a year of rated sessions, every device — beside yours, without touching it. <b>Repository</b> goes back.',
    ],
    shots: [web('repository', '/repository', 'The Repository page: the repository open, folders to browse, and a workbook to import',
      'The Repository page: the folders on this computer, and a workbook to import.')],
    page: { path: '/repository', label: 'Open Repository' },
    cli: [
      { cmd: 'mkdir -p ~/wits && cd ~/wits\nwits init .' },
      { cmd: 'wits import Tracking.xlsx', note: 'A dry run: what it would record, and anything in the sheet that does not add up.' },
      { cmd: 'wits import Tracking.xlsx --commit' },
      { cmd: 'wits restore demo-seed.wits', out: `Restored 53 products, 15 devices and 2617 events.` },
    ],
    tip: 'Every command finds its repository by walking up from the folder it runs in, as git does.',
  },
  {
    id: 'record',
    group: 'Every day',
    title: 'Recording',
    icon: 'add_circle',
    lead: '<b>Record</b>, at the bottom right (raised in the middle of the dock on a phone), is always in reach. Its five tabs are the five things that happen to grams.',
    steps: [
      '<b>Grind</b> <kbd>g</kbd> — out of a sealed jar, ground and ready to use. Pick the jar from the shelf, each a glass filled as it stands, and the amount: typed, stepped with − and +, slid, or one you use lately.',
      '<b>Session</b> <kbd>s</kbd> — from what is ground, through a device, at a temperature. It starts from your last session — strain, device, temperature — and the amounts you use are a tap away. Turn the dial, or step it with the arrow keys: the jar’s terpenes light up as the heat passes the point each boils at. Saved, it asks how it went.',
      '<b>Prescription</b> <kbd>b</kbd> — what the pharmacy handed you: one strain or several, each with its grams and price a gram. <b>Read the label</b> reads a photo of the pharmacy label; <b>Read invoices</b> reads the pharmacy’s PDF. Both are read in the browser, and the picture or the PDF goes nowhere.',
      '<b>Weigh</b> <kbd>r</kbd> — put a jar on the scale; see <a href="#weigh">When the scale disagrees</a>.',
      '<b>AVB</b> <kbd>v</kbd> — what you weighed out of a device, or used up.',
    ],
    then: [
      'Every tab shows what the entry will do to the jar before you record it — what goes out, what comes in — and <b>When</b> is now unless you say yesterday or another day.',
      'Anything can be dated back, for the evening you forgot. Nothing can be dated ahead, and nothing can take more than the jar held on its date: a grind dated before the fill it draws on is refused.',
    ],
    shots: [
      { src: 'guide/record-session.webp', alt: 'The Record dialog on its Session tab: jars to pick, device tiles, mode pills, a temperature dial and the amount', caption: 'A session starts from your last: the jar, the device, its dial, and the amounts you use a tap away.', frame: 'dialog', w: 645, h: 884 },
      { src: 'guide/record-fill.webp', alt: 'The Record dialog on its Fill tab, with Read invoices and Read the label', caption: 'A fill: its strains, their prices, and two readers that fill them in for you.', frame: 'dialog', w: 632, h: 704 },
    ],
    record: { tab: 0, label: 'Open Record' },
    cli: [
      { cmd: 'wits buy "Enua 36/1 Citrus Slap" 20g --price 180', note: 'A new product gets a short handle: cslap-361. Prefixes are enough while they are unambiguous.' },
      { cmd: 'wits grind sumo 0.5', out: '[677dd17] grind 0.50g sumo-papaya-bomb-251, 12.44g left in storage' },
      { cmd: 'wits sesh sumo 0.25 --device venty --temp 185', out: `[aceba4e] sesh 0.25g sumo-papaya-bomb-251, 2.51g left in the stash
At 185°C this releases THCA (120°C), Nerolidol (122°C), CBDA (130°C), …` },
      { cmd: 'wits grind sumo 0.4 --date 2026-10-24', note: '--date backdates any of them.' },
    ],
    tui: [
      { keys: ['b'], does: 'buy — a fill' },
      { keys: ['g'], does: 'grind' },
      { keys: ['s'], does: 'a session' },
      { keys: ['r'], does: 'weigh' },
      { keys: ['esc'], does: 'closes the form, and only the form' },
    ],
    tuiShot: term('tui-grind', 'The grind form in the terminal interface', 'The same grind, in the terminal interface: g on any screen.', 880, 480),
    tip: 'Tab completion offers the handles with what is left in each, and only the ones the command can act on: <code>wits completion bash</code> writes the script.',
  },
  {
    id: 'dashboard',
    group: 'Every day',
    title: 'The dashboard',
    icon: 'space_dashboard',
    lead: 'What is happening now. The fill in progress is a ring — sealed, ground, AVB — with the older jars as a halo around it, and beside it how many days it is worth at your rate.',
    body: [
      'Under it: what is sealed, ground and already vaped across the shelf; <b>every prescription not yet emptied</b>, jar by jar, each glass filled to what that prescription holds; the shelf’s supply as a curve to the day it runs out, with the day to order the next prescription by; and twenty weeks of grinding and sessions as calendars.',
      'A jar opens in full from here, or from the Shelf.',
      '<b>Today</b>, under the shelf, is the day so far: the THC in what went through a device, in milligrams — grams × the strain’s per cent × 10, so 0.15 g of a 20 % strain is 30 mg — against a usual day, the THC by strain and the grams by device as rings, each session on a clock of the day, and every session in a row with its share. It is what was in the material, not what reached you.',
    ],
    shots: [web('dashboard', '/', 'The dashboard: the fill as a ring, the shelf’s three accounts, and the open fills jar by jar',
      'Fill 30 on day 59: 36.73 g of its 60 g left, about 19 days’ worth.'),
    { src: 'guide/today.webp', alt: 'Today on the dashboard: 206 mg THC in four sessions, by strain, by device and through the day',
      caption: 'Today: 206 mg THC in 0.67 g, four sessions, against a usual 239 mg.', frame: 'dialog', w: 1440, h: 640 }],
    page: { path: '/', label: 'Open the dashboard' },
    cli: [
      { cmd: 'wits status', out: `On cycle 30, opened 2026-08-29 (day 59)

PRODUCT                       STORAGE  STASH  AVB    LEFT
nice-morning-sun-321          11.37g   3.40g  1.91g  57%
all-nations-mac-doughnut-291  12.42g   3.30g  1.72g  62%
sumo-papaya-bomb-251          12.94g   2.26g  2.15g  65%

total                         36.73g                 61%

36.73g of 60.00g left over 55 days, 23 of them with an entry
8.07g more in 2 older jars, 1 earlier cycle still open
1.93g per active day, 1.69g median, 0.81g per elapsed day
About 19 days' worth at that rate
Order the next prescription by Mon 16 November, with 3 days for the pharmacy.` },
      { cmd: 'wits today', out: `Monday 26 October: 4 sessions, 0.67 g through a device: 206 mg THC and 7 mg CBD in the material

TIME   PRODUCT                DEVICE                           GRAMS  THC      CBD     RATED
07:50  NICE 32/1 Morning Sun  Storz & Bickel Venty at 180°C    0.15g  48.0 mg  1.5 mg  4 of 5
09:30  Sumo 25/1 Papaya Bomb  Storz & Bickel Mighty+ at 190°C  0.22g  55.0 mg  2.2 mg  -
…`, note: 'The last line of wits status says the same in one line.' },
      { cmd: 'wits refill --ics refill.ics', out: `44.80 g on the shelf, at 1.93 g a day with grinding: empty on Thu 19 November.
Order the next prescription by Mon 16 November, with 3 days for the pharmacy.`, note: '--ics writes a calendar reminder for the morning to order by; --lead 5 changes the pharmacy’s days.' },
    ],
    tui: [
      { keys: ['wits'], does: 'on its own, opens the terminal interface on its dashboard' },
      { keys: ['tab'], does: 'the next screen: journal, analysis, storage, stash, sessions, devices' },
      { keys: ['?'], does: 'every key the screen honours' },
      { keys: ['q'], does: 'quits' },
    ],
    tuiShot: term('tui-dashboard', 'The terminal interface’s dashboard', 'The same dashboard in a terminal: storage, stash, sessions, devices and the supply projection.'),
  },
  {
    id: 'forgot',
    group: 'Every day',
    title: 'I forgot to',
    icon: 'checklist',
    lead: 'Everything left undone that the ledger can see, each kind only while there is some. The tab counts them.',
    steps: [
      '<b>Rate a session</b> from the last week, while you still remember it: overall, pain relief, sleep, calm, mood, focus, appetite — any of them, 1 to 5.',
      '<b>Rate a strain</b> used in the last 90 days as a whole: its taste, its aroma, whether you would have it again.',
      '<b>Price a pickup</b> — type the prices, or scan the invoice. <b>Add invoices</b> takes many PDFs at once and prices every fill they match.',
      '<b>Leftovers</b>, where a jar has nothing sealed left but still holds ground grams: tick and clear them together.',
      '<b>AVB waiting</b> in a device, with <b>Collect</b>.',
      'Offered but not counted: the jars to weigh against the scale — <b>Sealed</b>, and <b>Ground</b> for any used in the last month — and the jars with no <b>terpene profile</b> — <b>Research all</b> looks them up on the web, and nothing is written until you keep one.',
    ],
    shots: [web('forgot', '/forgot', 'I forgot to: sessions of the last week waiting for a rating', 'Three sessions waiting for a rating, and six strains.')],
    page: { path: '/forgot', label: 'Open I forgot to' },
    cli: [
      { cmd: 'wits rate last overall=4 sleep=5', out: '[edf332f] rated session aceba4e: overall 4, sleep 5' },
      { cmd: 'wits describe enua-wedding --rating "overall=4 taste=5" --again yes' },
      { cmd: 'wits reconcile purchases', note: 'One question for every purchase without a price; --all for every one.' },
      { cmd: 'wits describe enua-wedding --terpenes "Myrcen, Limonen, Caryophyllen"' },
    ],
    tui: [
      { keys: ['e'], does: 'on the sessions screen, rates the session under the cursor' },
      { keys: ['$'], does: 'prices the purchases still without one' },
      { keys: ['c'], does: 'on the storage screen, clears stale stashes from earlier fills' },
    ],
  },
  {
    id: 'jars',
    group: 'The shelf',
    title: 'The jars, and one jar in full',
    icon: 'inventory_2',
    lead: '<b>Shelf → Jars</b> is a gallery: every product a card across its life. Tick cards and <b>Clear leftovers</b> records what is ground of them as consumed; <b>Play back history</b> replays the shelf from empty; <b>Rate</b> rates the strain; <b>Film</b> records the shelf’s three years as a thirty-second video.',
    body: [
      'A card opens the jar in full: what it is (producer, strain, THC and CBD), its terpenes and its taste as a wheel; every fill of it and what each still holds; what it cost a gram; what was ground and used month by month; its sessions by device, temperature and rating; and the products most like it.',
      '<b>What I carry</b>, on the same page, is a signed QR code of what you hold against the fills that dispensed it — what a German patient must be able to show. Any phone that scans it checks the signature.',
    ],
    shots: [
      web('jars', '/shelf/jars', 'The jars as a gallery of cards', 'Every product as a card, with what it holds of each account.'),
      web('jar', '/shelf/jars/cantourage-mac1-251', 'One jar in full: Cantourage 25/1 MAC1+', 'Cantourage 25/1 MAC1+: 65 g ground over its life, and 111 ratings.'),
    ],
    page: { path: '/shelf/jars', label: 'Open the jars' },
    cli: [
      { cmd: 'wits describe cantourage-mac1-251', out: `Cantourage 25/1 MAC1+ (cantourage-mac1-251)
THC 25 %, CBD 1 %
Terpenes: Limonene, Linalool, Pinene, Caryophyllene, Myrcene
Rated: not yet (--rating)` },
      { cmd: 'wits like cantourage-mac1-251', out: `Like Cantourage 25/1 MAC1+ (citrus, flowers and pine):
   99 %  Cantourage 25/1 MAC 1+ — shares Limonene, Linalool, Pinene, Caryophyllene, Myrcene
   98 %  420 Evolution 25/1 CA MAC: MAC1 — shares Limonene, Linalool, Pinene, Caryophyllene, Myrcene
   87 %  enua 22/1 Black Cherry Punch — shares Limonene, Linalool, Pinene, … — rated 4.1 overall, over 39 sessions
   …` },
      { cmd: 'wits carry --out carry.svg', out: `PRODUCT                            SEALED  GROUND  AVB    DISPENSED
Sumo 25/1 Papaya Bomb              12.44g  2.51g   2.15g  20.00g on 2026-08-29
All Nations 29/1 Mac Doughnut      12.42g  3.30g   1.72g  20.00g on 2026-08-29
…
82.38g in all. Signed by SHA256:…; the QR code opens it at https://thedondope.github.io/wits/verify.html` },
    ],
    tui: [
      { keys: ['space'], does: 'ticks a jar on the storage or stash screen' },
      { keys: ['r'], does: 'weighs the ticked jars together' },
      { keys: ['e'], does: 'corrects a product’s name' },
      { keys: ['p'], does: 'replays the shelf from empty' },
    ],
    tuiShot: term('tui-storage', 'The storage screen of the terminal interface', 'The storage screen: what still holds something, and every jar worked down to zero.'),
  },
  {
    id: 'journal',
    group: 'The shelf',
    title: 'The journal, and putting a mistake right',
    icon: 'receipt_long',
    lead: '<b>Shelf → Journal</b> is every entry, newest first. Undo one and wits records a correction that moves the same grams back; both stay in the log.',
    shots: [web('journal', '/shelf/journal', 'The journal: every entry, newest first', 'The journal, day by day.')],
    page: { path: '/shelf/journal', label: 'Open the journal' },
    cli: [
      { cmd: 'wits log --oneline -n 4', out: `77bf22b  2026-10-26  adjust  30.00g  cannabistada-gorilla-glue-4-221
a854ec6  2026-10-26  adjust  25.00g  gr-nhorn-gorilla-glue-4-221
882bd9d  2026-10-26  adjust  59.00g  imc-hindu-kush-201
12c4428  2026-10-26  adjust  50.00g  avaay-blueberry-haze-161` },
      { cmd: 'wits show aceba4e', out: `sesh aceba4e497d96827a853d31ebdb341fa3660ed0c30e9e642221c63a6cfeeabfa
entry 2619, chained onto 677dd17

Product     Sumo 25/1 Papaya Bomb (sumo-papaya-bomb-251)
Moved       0.25 g, stash → consumed
In the jar  stash      2.76 g →   2.51 g
            consumed   2.65 g →   2.90 g
Device      Storz & Bickel Venty at 185°C
Fill        30, opened 2026-08-29
Rated       overall 4, sleep 5` },
      { cmd: 'wits revert aceba4e --reason "misread the scale"' },
    ],
    tui: [
      { keys: ['e'], does: 'amends an amount' },
      { keys: ['d'], does: 'undoes an entry' },
      { keys: ['v'], does: 'shows the corrections behind what stands' },
      { keys: ['f'], does: 'filters' },
      { keys: ['←', '→'], does: 'browses the entries as cards' },
    ],
    tuiShot: term('tui-journal', 'The journal screen of the terminal interface', 'The journal in a terminal, an entry as a card above the list.'),
  },
  {
    id: 'weigh',
    group: 'The shelf',
    title: 'When the scale disagrees',
    icon: 'scale',
    lead: 'Ledgers drift: a session goes unlogged, a grind is forgotten. Grams do not vanish, so less than the ledger says is offered as what was forgotten; anything else is recorded as a difference, and the jar agrees with the scale again. The past is not edited.',
    steps: [
      'Press <b>Record → Weigh</b>, or <kbd>r</kbd> anywhere — or <b>Sealed</b> and <b>Ground</b> under <b>I forgot to</b>.',
      'Pick the jar and what is on the scale — sealed, ground, or already vaped.',
      'Type what the scale reads into its display. The ledger’s figure is under it and the difference lights up; a reading that agrees writes nothing.',
      'Less than the ledger? <b>I forgot to record a grind</b> (sealed) or <b>sessions</b> (ground) writes them, dated after the jar’s last entry; <b>It went missing</b> records the grams as lost. After a grind, weigh what is ground next — the dialog stays for it.',
    ],
    shots: [{ src: 'guide/record-weigh.webp', alt: 'The Weigh tab: a jar on the scale reading 12.6 g against the ledger’s 12.94 g, offered as a forgotten grind', caption: 'On the scale: 0.34 g light, and most likely a grind you forgot to record.', frame: 'dialog', w: 630, h: 845 }],
    record: { tab: 3, label: 'Weigh a jar' },
    cli: [
      { cmd: 'wits reconcile storage sumo 12.4 --dry-run', out: `storage holds 12.44g by the ledger and 12.40g on the scale: -0.04g

Dry run. Nothing was written. Re-run without --dry-run to record it.` },
      { cmd: 'wits reconcile', note: 'With no arguments it asks: which account, which jars, then each reading in turn, and of a jar found short whether a grind or sessions were forgotten. Nothing is written until every question is answered.' },
      { cmd: 'wits reconcile stash sumo 6.5 --forgot', note: 'Puts a shortfall down to forgotten sessions — of the usual size, or your gear’s load — without asking.' },
    ],
    tui: [{ keys: ['r'], does: 'weighs the ticked jars, the one under the cursor, or the fullest; a jar found short asks whether something was forgotten' }],
  },
  {
    id: 'devices',
    group: 'The shelf',
    title: 'Devices and temperatures',
    icon: 'air',
    lead: '<b>Shelf → Devices</b> lists what a session is had with, what heats a manual one, and the grinders. <b>Add a device</b> starts from a preset — the Venty, the Mighty+, the Herb Frolic, the Tornado and more — or from nothing.',
    body: [
      'A device has its maker, how it heats, its temperature range and its modes, each with its own temperature and timer, and for a vaporizer or glass what <b>a load holds</b> — 0.15 g on most presets, 0.1 g on the Simrells — which sizes a session you forgot to record. Its card shows what it has done: sessions and grams, when it was last used, and its price per session so far. A retired device is kept, since its sessions name it.',
      'Wits knows the boiling point of every cannabinoid and terpene, so a number on a dial reads as what it releases — up to the point it starts producing benzene.',
    ],
    shots: [web('devices', '/shelf/devices', 'The devices page: vaporizers with their sessions and grams', 'Every device by what it is for, with what it has done.')],
    page: { path: '/shelf/devices', label: 'Open the devices' },
    cli: [
      { cmd: 'wits device add "Volcano Hybrid" --min 40 --max 230 --default 185 --load 0.15' },
      { cmd: 'wits temps 190', out: `COMPOUND         BOILS AT  EFFECTS
THCA             120°C     anti-inflammatory, anti-epileptic, anti-proliferic
Nerolidol        122°C     sedative, anti-fungal, anti-malarial, anti-parasitic, skin penetration enhancer
CBDA             130°C     anti-inflammatory, anti-proliferic
β-Caryophyllene  130°C     anti-inflammatory, anti-malarial, cytoprotective, analgesic
…` },
    ],
    tui: [
      { keys: ['a'], does: 'adds a device on the devices screen' },
      { keys: ['e'], does: 'edits one' },
      { keys: ['d'], does: 'removes one' },
    ],
  },
  {
    id: 'avb',
    group: 'The shelf',
    title: 'AVB',
    icon: 'recycling',
    lead: 'Already vaped bud is what is left once a session has been through a device. When you empty a device, weigh what comes out and record it on <b>Record → AVB</b>; the dashboard’s AVB card shows the last yield.',
    body: [
      'What came out is shared between the products put through that device since it was last emptied, in proportion to what each put in. More cannot come out than went in. What stays behind in <b>consumed</b> is what the vapour took.',
    ],
    record: { tab: 4, label: 'Record AVB' },
    cli: [
      { cmd: 'wits avb collect 0.21', note: 'Weighed out of a device; --device when more than one has sessions waiting.' },
      { cmd: 'wits avb', out: `102.93g in the AVB jar

COLLECTED   DEVICE                 AVB    FROM                  YIELD
2026-10-24  limelight-herb-frolic  1.28g  2.35g in 10 sessions  54%
2026-10-21  storz-bickel-venty     1.70g  3.28g in 13 sessions  52%
2026-10-12  storz-bickel-mighty    1.55g  3.30g in 15 sessions  47%
…` },
      { cmd: 'wits avb use 0.1 --note tincture' },
    ],
  },
  {
    id: 'trip',
    group: 'The shelf',
    title: 'Packing for a trip',
    icon: 'luggage',
    lead: '<b>Shelf → Trip</b> says what to take: the days away at this fill’s rate, with some to spare, drawn from the jars on the shelf — and signs a QR code for exactly that.',
    steps: [
      'Pick the days away — <b>Weekend</b>, <b>A week</b>, <b>Two weeks</b> — or slide to any number up to six weeks.',
      'Set how much to take on top; 20 % covers a day’s delay or a heavier evening.',
      '<b>Evenly</b> takes about the same from every jar, so the trip has a mix and the jars go down together; <b>Oldest first</b> empties the oldest fill’s jars.',
      'Each jar shows what it gives, lit: its ground grams first, then sealed ones, which you grind before you go. Nothing is recorded until you do.',
      'The QR code is signed for exactly what is packed. Any phone that scans it checks the signature.',
    ],
    shots: [web('trip', '/shelf/trip', 'The Trip page: a week away, packed evenly from five jars', 'A week away: 16.30 g, about 3.3 g from each of five jars.')],
    page: { path: '/shelf/trip', label: 'Open Trip' },
    cli: [
      { cmd: 'wits pack --days 10', out: `Take 23.20 g: 10 days at 1.93 g a day, with 20 % to spare.

PRODUCT                            GROUND  SEALED  TAKE   DAYS  LEFT
420 Evolution 27/1 Ice Cream Cake  4.60g   0.00g   4.60g  2.4   7.07g
Enua 36/1 Citrus Slap              4.60g   0.00g   4.60g  2.4   5.01g
All Nations 29/1 Mac Doughnut      3.30g   1.40g   4.70g  2.4   11.02g
…` },
      { cmd: 'wits pack --days 4 --margin 0 --oldest --out trip.svg', note: 'Oldest first, nothing on top, and the QR code written to a file.' },
      { cmd: 'wits verify trip.txt', note: 'A statement written with --out trip.txt checks like any other.' },
    ],
  },
  {
    id: 'analysis',
    group: 'Insights',
    title: 'Analysis, and the record played back',
    icon: 'monitoring',
    lead: '<b>Insights → Analysis</b> is the long view: this fill, the last 30 or 90 days, 12 months, or all time. Press play and everything replays from empty, an entry a moment.',
    body: [
      'The daily chart fills in, the products race each other up the ranking, the fills grow on their timeline. The panel above them is the record in time — the grams ground, a blue tick at every pickup — lit up to the playhead: point anywhere on it to go there, or step with <kbd>←</kbd> <kbd>→</kbd>. Under it the <b>tape</b> runs, the newest entries in a row, each new one pushing the others along. <code>?at=2025-02-14</code> opens it as the record stood that day.',
      '<b>Tolerance</b> draws grams a day against how the sessions went, both over the same 14 days rolling, with breaks of a week or more shaded behind. Tolerance creeps when the grams climb and the rating stays flat.',
    ],
    shots: [web('analysis', '/insights/analysis', 'Analysis: grams a day, the products ranked, and the replay', 'This fill: 44.4 g ground on 23 of its 55 days, 1.93 g an active day.')],
    page: { path: '/insights/analysis', label: 'Open Analysis' },
    cli: [
      { cmd: 'wits export --from 2026-08-01 --to 2026-10-25 > autumn.md', note: 'The same range as Markdown: printable, diffable, and legible without wits.' },
      { cmd: 'wits export --site docs/', note: 'Every fill as a page, for GitHub Pages.' },
    ],
    tui: [
      { keys: ['s'], does: 'changes the scope' },
      { keys: ['p'], does: 'plays the replay, or pauses it' },
      { keys: ['←', '→'], does: 'steps an entry at a time' },
      { keys: ['+', '-'], does: 'retunes the speed' },
    ],
    tuiShot: term('tui-analysis', 'The analysis screen of the terminal interface', 'Analysis in a terminal: a braille chart with its seven-day average, the products ranked, the rhythm.'),
  },
  {
    id: 'effects',
    group: 'Insights',
    title: 'Effects, and the map of the strains',
    icon: 'psychology',
    lead: '<b>Insights → Effects</b> is what the ratings add up to: every product against every dimension, each dimension by temperature, and by time of day as a wheel.',
    body: [
      'Every mean says how many ratings it stands on, and one that stands on a single rating is hatched: a rating, not a finding.',
      'Under it is <b>a map of the strains</b>, laid out by terpene profile, so strains that taste alike sit together whatever they are called. Pick one for its taste, its terpenes, and the five most like it. Then <b>the terpenes on the shelf</b>: each one, what it tastes of, and how the sessions of the strains that lead with it went.',
    ],
    shots: [web('effects', '/insights/effects', 'Effects: products by dimension, by temperature and by time of day', 'What works, by product, temperature and time of day.')],
    page: { path: '/insights/effects', label: 'Open Effects' },
    cli: [
      { cmd: 'wits rate last sleep=5 calm=4 --note "slept through"' },
      { cmd: 'wits like enua-wedding' },
    ],
    tui: [{ keys: ['e'], does: 'on the sessions screen, rates a session: ← → or 1–5 to score, 0 to leave one out' }],
    tuiShot: term('tui-sessions', 'The sessions screen of the terminal interface', 'The sessions screen: the newest, as rated, and what works by product.'),
  },
  {
    id: 'history',
    group: 'Insights',
    title: 'History, and a page for the prescriber',
    icon: 'timeline',
    lead: '<b>Insights → History</b> is every day as a calendar and every fill as a lane on one timeline, overlapping the way fills do. From here, download <b>the report</b>: one page for an appointment.',
    body: [
      'The report covers the days you ask for: grams a day, the weeks as bars, which strains, what the sessions did — each mean with its count — and today’s fill and shelf with the date the next prescription is due. Its footer names the journal’s head, so it can be checked against a seal.',
    ],
    shots: [web('history', '/insights/history', 'History: a calendar of every day and every fill as a lane', 'Three years of days, and every fill on one timeline.')],
    page: { path: '/insights/history', label: 'Open History' },
    cli: [
      { cmd: 'wits report --from 2026-08-01 --to 2026-10-25', out: 'Wrote a report for 2026-08-01 to 2026-10-25 to wits-report-2026-10-25.pdf' },
      { cmd: 'wits report --seal', note: 'Seals the journal first, so the page names the seal it can be checked against.' },
    ],
  },
  {
    id: 'costs',
    group: 'Insights',
    title: 'What it costs',
    icon: 'euro',
    lead: '<b>Insights → Costs</b> adds the prices up: what was paid, a gram over everything and lately, and a month at this fill’s rate.',
    body: [
      'Every purchase is a dot for its price a gram, with a line through each fill’s. Each strain is a row you sort by any column, gathering the same cultivar under every name it came by; each pharmacy is named from its invoices, and told on the charts by its shape as well as its colour. Only purchases with a price count, and how many have none is said, not guessed at.',
    ],
    shots: [web('costs', '/insights/costs', 'Costs: what was paid, a gram, and every purchase over time',
      '€10,156 for 1,244 g, priced from the pharmacy invoices.')],
    page: { path: '/insights/costs', label: 'Open Costs' },
    cli: [
      { cmd: 'wits price 3f9a2c1 120,50 --note "the label said 120"' },
      { cmd: 'wits costs', out: `10156.01 € for 1244.00g with a price, 8.16 € a gram; 1 purchase without one

FILL  DISPENSED   GRAMS   PAID      A GRAM
30    2026-08-29  60.00g  351.80 €  5.86 €
29    2026-07-09  60.00g  431.80 €  7.20 €
28    2026-06-16  25.00g  159.75 €  6.39 €
…` },
    ],
    tui: [{ keys: ['$'], does: 'walks every purchase without a price' }],
  },
  {
    id: 'year',
    group: 'Insights',
    title: 'The year, on one card',
    icon: 'auto_awesome',
    lead: '<b>Insights → Year</b> is the year in wits: what was ground and on how many days, then a card for each finding — the months, every day, the strains, the terpene of the year, the longest break, the temperature and the strain rated best, the device, and what it cost.',
    body: [
      'Every figure is the year’s own, to today in a year still running. Pick another year at the top, or open one with <code>?year=2025</code>. A temperature is named best only on five ratings or more; a break is a week or more with nothing ground and no session, as the tolerance curve counts them.',
    ],
    shots: [web('year', '/insights/year?year=2025', 'The year 2025 on one card', '2025: 446.3 g on 221 days, a 53-day break, 190–199 °C rated best.')],
    page: { path: '/insights/year', label: 'Open the year' },
    cli: [
      { cmd: 'wits review 2025', out: `╭─────────────────────────────────────────────────────────────╮
│  2025 in wits                                               │
│  446.34 g ground on 221 of 365 days · 2.02 g an active day  │
│  11 prescriptions · 309 sessions · 25 strains               │
╰─────────────────────────────────────────────────────────────╯

  Oct  ███████████████████████▌          49.96 g
  Nov  █████████████████████████▉        55.00 g
  Dec  ████████████████████████████████  67.98 g  ← the most
  …
  Longest break        53 days, 21 March to 12 May (2 breaks in all)
  Best temperature     190–199 °C, 4.2 overall over 67 ratings
  Terpene of the year  Limonene — citrus, lemon, orange, 60 % of the grams ground
  Dearest gram         10.99 €, Pedanios 22/1 DNK Ghost Train Haze on 6 February` },
      { cmd: 'wits review', note: 'This year, so far.' },
    ],
  },
  {
    id: 'keep',
    group: 'Keeping it',
    title: 'Backups, and more than one device',
    icon: 'sync',
    lead: 'A <b>bundle</b> is the whole repository as one small file, restored exactly, hash chain included. A <b>remote</b> is a folder several devices reach — a Syncthing share, a USB stick — holding one encrypted file.',
    body: [
      'A pull works the way <code>git rebase</code> does: what both devices share is kept, the remote’s entries are taken as they are, and this device’s own are chained again on top. A push is refused until this device holds everything on the remote, so nothing another wrote is written over.',
      'The phone keeps its own copy and syncs through the same folder, by hand: see <a href="#phone">On the phone</a>.',
    ],
    cli: [
      { cmd: 'wits bundle --gzip --out history.wits.gz', note: 'The workbook’s 1429 entries are 6,693 bytes gzipped.' },
      { cmd: 'wits restore history.wits.gz' },
      { cmd: 'wits remote add ~/Sync/wits', note: 'Its key is .wits/remote.key: bring it to every other device, and pass it to wits remote add --key there.' },
      { cmd: 'wits push\nwits pull' },
    ],
  },
  {
    id: 'seal',
    group: 'Keeping it',
    title: 'A record others can check',
    icon: 'verified',
    lead: 'The journal’s last hash fixes everything before it. A <b>seal</b> signs that hash with a key of your own, and a timestamp has it committed into Bitcoin, so a doctor can check a report against the journal without trusting you or wits.',
    cli: [
      { cmd: 'wits seal --timestamp', note: 'Sends one SHA-256, and nothing else, to the OpenTimestamps calendars: the one network call wits makes, and only when asked.' },
      { cmd: 'wits seal', out: `Made a sealing key in .wits/seal.key; its fingerprint is SHA256:hdi36VOO…
Sealed the journal's 2619 entries, head aceba4e, as 2026-10-26-aceba4e497d9.seal` },
      { cmd: 'wits verify .wits/seals/2026-10-26-aceba4e497d9.seal' },
      { cmd: 'wits seal --key', note: 'The key’s fingerprint, to hand once to whoever checks. Never the key file.' },
    ],
  },
  {
    id: 'keys',
    group: 'Around',
    title: 'Keys and themes',
    icon: 'keyboard',
    lead: 'Every key the terminal has is here too. Once you use one, a line along the foot shows the ones the page answers; <kbd>?</kbd> shows all of them, always.',
    steps: [
      '<kbd>g</kbd> <kbd>s</kbd> <kbd>b</kbd> <kbd>r</kbd> <kbd>v</kbd> record: grind, session, fill, weigh, AVB.',
      '<kbd>1</kbd>–<kbd>4</kbd> the tabs; <kbd>h</kbd> <kbd>l</kbd> every page in turn.',
      '<kbd>p</kbd> replays, <kbd>j</kbd> <kbd>k</kbd> move, <kbd>space</kbd> ticks, <kbd>o</kbd> opens a jar.',
      '<kbd>t</kbd> the next theme, <kbd>T</kbd> the one before; or the palette in the header.',
    ],
    body: [
      'The palette holds 41 themes. They differ in more than colour — Paper is cream and serif, Phosphor a green CRT with scanlines, Blueprint a cyanotype grid — and every one keeps the accounts’ colours: storage blue, stash green, AVB amber. <code>?theme=paper</code> opens a page in one without keeping it, and <code>?record=grind</code> opens the form: a link to bookmark.',
    ],
    shots: [
      web('theme-paper', '/?theme=paper', 'The dashboard in the Paper theme', 'Paper.'),
      web('theme-phosphor', '/?theme=phosphor', 'The dashboard in the Phosphor theme', 'Phosphor.'),
      web('theme-blueprint', '/?theme=blueprint', 'The dashboard in the Blueprint theme', 'Blueprint.'),
    ],
    tui: [
      { keys: ['t'], does: 'picks a theme: wits’ own, or one of twenty editor themes; enter keeps it' },
      { keys: ['tab'], does: 'the next screen' },
      { keys: ['?'], does: 'every key' },
      { keys: ['q'], does: 'quits' },
    ],
  },
  {
    id: 'phone',
    group: 'Around',
    title: 'On the phone',
    icon: 'smartphone',
    lead: 'The same interface runs on a phone, with wits itself inside it: compiled to WebAssembly, keeping its own copy of the journal in the browser, and refusing the same things in the same words. It works offline once it has loaded.',
    steps: [
      'Open <code>thedondope.github.io/wits</code> and <b>Add to home screen</b> from the browser’s menu.',
      'On the device that made the shared folder, show the key: <code>cat .wits/remote.key</code>.',
      'On the phone, open <b>Sync</b> on the menu (⋮), open <code>wits.age</code> from the shared folder, and paste the key the first time.',
      '<b>Sync</b> pulls the folder’s entries in, chains the phone’s own on top and pushes; <b>Save wits.age</b> hands the result to your Files or Syncthing app to save over the folder’s copy.',
    ],
    shots: [
      { src: 'guide/phone-dashboard.webp', alt: 'The dashboard on a phone, with the dock under the thumb', caption: 'The dashboard on a phone: the dock under the thumb, Record raised.', frame: 'phone', w: 750, h: 1624 },
      { src: 'guide/phone-forgot.webp', alt: 'I forgot to on a phone', caption: 'I forgot to, one card a row.', frame: 'phone', w: 750, h: 1624 },
    ],
    cli: [{ cmd: 'wits pull', note: 'On the computer, after the phone saved over wits.age: its entries arrive here.' }],
  },
];
