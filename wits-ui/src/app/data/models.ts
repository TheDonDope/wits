// The shapes wits-server answers with (pkg/api). Every figure keeps the scope
// it has everywhere in wits — the fill (one prescription's own grams), the
// shelf (every jar standing today), the jar (one product across its life) —
// and the comments say which.

export interface Point {
  date: string;
  grams: number;
}

/** The shelf's grams since the fill arrived, then on at the observed rate. */
export interface Projection {
  history: Point[];
  projected: Point[];
  empty_on?: string;
}

/** One product's row of the fill — the fill's scope. */
export interface FillProduct {
  slug: string;
  name: string;
  dispensed: number;
  sealed: number;
  stash: number;
  avb: number;
}

/** The prescription in progress — the fill's scope. */
export interface Fill {
  number: number;
  start: string;
  day: number;
  dispensed: number;
  remaining: number;
  remaining_pct: number;
  per_active_day: number;
  days_worth: number;
  /** What older fills still hold: the seam between the fill and the shelf. */
  carried: { grams: number; jars: number; cycles: number };
  products: FillProduct[];
  shelf_projection: Projection;
  /** When to order the next prescription: the projection's empty day, less the pharmacy's lead time. */
  refill?: { order_by: string; empty_on: string; lead_days: number; overdue: boolean };
}

/** Every jar standing today — the shelf's scope. */
export interface Shelf {
  storage: number;
  stash: number;
  avb: number;
  jars: number;
}

export interface Entry {
  hash: string;
  type: 'purchase' | 'grind' | 'sesh' | 'avb-collect' | 'avb-use' | 'adjust';
  occurred_at: string;
  recorded_at: string;
  product?: string;
  product_name?: string;
  grams: number;
  from: string;
  to: string;
  device?: string;
  temperature?: number;
  /** A session's mode on its device, what heated it, and that one's setting. */
  mode?: string;
  heater?: string;
  setting?: string;
  /** A session's timer, seconds. */
  timer?: number;
  note?: string;
  reverts?: string;
  corrected?: boolean;
}

/** A purchase without a price, with the fill — the pickup, the prescription — it came in. */
export interface UnpricedEntry extends Entry {
  fill: number;
  fill_start: string;
}

export interface Overview {
  now: string;
  chain_ok: boolean;
  fill: Fill | null;
  /** Every fill still holding grams of its own, newest first — each in its own scope. */
  fills: Fill[];
  shelf: Shelf;
}

/** A strain rated as a whole, apart from any one session: each score 1–5, or left out. */
export interface StrainRating {
  overall?: number;
  taste?: number;
  aroma?: number;
  again?: 'yes' | 'no';
  note?: string;
  at?: string;
}

/** One product across its lifetime — the jar's scope. */
export interface Jar {
  slug: string;
  name: string;
  manufacturer?: string;
  cultivar?: string;
  thc?: number;
  cbd?: number;
  /** Dominant first; empty until written down. */
  terpenes: string[];
  /** The strain rated as a whole; none until it is. */
  rating?: StrainRating;
  storage: number;
  stash: number;
  avb: number;
  bought: number;
  ground: number;
  fills: number;
  last_seen: string;
  /** When it was last ground or had in a session; none if never. */
  last_used?: string;
}

export interface Cycle {
  number: number;
  start: string;
  end?: string;
  open: boolean;
  dispensed: number;
  /** Every grind during the fill's tenure, from whichever jar — the shelf's. */
  ground_during: number;
  remaining: number;
  products: string[];
}

/** What a device is, as it is added or changed. */
export interface DeviceFields {
  name: string;
  kind?: string;
  min_temp?: number;
  max_temp?: number;
  default_temp?: number;
  category: DeviceCategory;
  maker?: string;
  model?: string;
  country?: string;
  power?: '' | 'electric' | 'manual';
  signature?: '' | 'convection' | 'conduction' | 'hybrid';
  use?: '' | 'session' | 'on-demand' | 'both';
  /** Glass that holds water: a bubbler, a rig with water. */
  water?: boolean;
  /** What it is set to when used, and asked which of: a boost, a session mode, an induction heater's wattage. */
  modes: DeviceMode[];
  /** How long, in seconds, it stays on unused before it turns itself off. */
  auto_off?: number;
  /** The grams one fill holds, as it is usually loaded. */
  load?: number;
  cents?: number;
  shop?: string;
  bought_on?: string;
  link?: string;
  notes?: string;
  retired?: boolean;
}

export type DeviceCategory = 'vaporizer' | 'induction-heater' | 'torch' | 'grinder' | 'glass';

/** One way a device is run; a mode may heat its own way (on demand by convection, a session hybrid). */
export interface DeviceMode {
  name: string;
  signature?: '' | 'convection' | 'conduction' | 'hybrid';
  /** The temperature it is usually run at, °C. */
  temp?: number;
  /** A session timer, seconds, for a mode that runs a session for a set time. */
  timer?: number;
}

/** A device, and what has been done with it. */
export interface Device extends DeviceFields {
  slug: string;
  sessions: number;
  grams: number;
  /** Sessions it heated: an induction heater's, a torch's. */
  heated: number;
  last_used?: string;
}

export interface Day {
  date: string;
  ground: number;
  seshed: number;
}

export interface Written {
  entry: Entry;
  message: string;
}

/** What a jar holds in each account. */
export interface Accounts {
  storage: number;
  stash: number;
  consumed: number;
  avb: number;
}

/**
 * What a write would record and do, rehearsed by the server (?dry=1) and
 * nothing written: the entries, the message it would say, each jar it
 * touches before and after, and a fill's lines priced as they would be.
 */
export interface Preview {
  entries: Entry[];
  message: string;
  jars: { product: string; name: string; before: Accounts; after: Accounts }[];
  lines?: { product: string; added: boolean; cents: number; vat_cents: number }[];
}

/**
 * What a scale reading short of the ledger would be put down to, not yet
 * written: a grind forgotten, for what is sealed; sessions forgotten, for what
 * is ground. No entries when the reading is not short.
 */
export interface ForgottenPlan {
  expected: number;
  short: number;
  entries: Entry[];
  /** Why a shortfall is not put down to anything forgotten: more sessions than anybody forgets. */
  why?: string;
  /** What forgotten sessions are sized by: the sessions on record, the gear's load, or one a day. */
  sized_by?: 'sessions' | 'load' | 'day';
}

/** What a pharmacy label says about a fill, read from the text of its photograph. */
export interface LabelReading {
  /** The product line, or the name of the product it already is; empty when none was found. */
  name: string;
  thc?: number;
  cbd?: number;
  /** Absent when the label gave no amount. */
  grams?: number;
  /** The product already in the catalog, when the label names one. */
  slug?: string;
}

/** What a session can be rated on, 1 to 5. */
export type Dimension = 'overall' | 'pain' | 'sleep' | 'calm' | 'mood' | 'focus' | 'appetite';

/** A session's standing rating, with the session it rates. */
export interface Rated {
  hash: string;
  at: string;
  scores: Partial<Record<Dimension, number>>;
  note?: string;
  session: Entry;
}

/** What one kind of session did on average, and over how many ratings. */
export interface EffectGroup {
  key: string;
  label: string;
  count: number;
  means: Partial<Record<Dimension, number>>;
  counts: Partial<Record<Dimension, number>>;
}

/** The effect log: what can be rated, what has been, what waits, what it says. */
export interface EffectLog {
  dimensions: Dimension[];
  rated: Rated[];
  unrated: Entry[];
  insights: {
    rated: number;
    by_product: EffectGroup[];
    by_temperature: EffectGroup[];
    by_time_of_day: EffectGroup[];
  };
}

/** The AVB jar: what it holds, what each device has waiting, and every collection's yield. */
export interface Avb {
  held: number;
  waiting: { device: string; grams: number; sessions: number }[];
  yields: { at: string; device: string; collected: number; from: number; sessions: number; ratio: number }[];
}

/** What the prescriptions cost, in cents: by fill, by month, and per session that helped. */
export interface Costs {
  cents: number;
  priced: number;
  per_gram: number;
  unpriced: number;
  helped: number;
  per_helped: number;
  helped_unpriced: number;
  fills: { number: number; start: string; dispensed: number; priced: number; cents: number; per_gram: number; unpriced: number }[];
  months: { month: string; cents: number; grams: number }[];
  /** Every purchase with a price, oldest first. */
  bought: Bought[];
  /** The purchases by strain (cultivar), producer, pharmacy and year: most spent first, years in order. */
  strains: CostGroup[];
  makers: CostGroup[];
  sellers: CostGroup[];
  years: CostGroup[];
}

/** One purchase with its price, in cents. */
export interface Bought {
  hash: string;
  at: string;
  fill: number;
  product: string;
  name: string;
  maker: string;
  /** The cultivar, keyed alike under the names it is sold by. */
  strain: string;
  strain_name: string;
  thc: number;
  cbd: number;
  grams: number;
  cents: number;
  per_gram: number;
  /** The pharmacy, as its invoice names it; empty when typed in. */
  seller: string;
}

/** Purchases taken together, with the price a gram (cents) of the cheapest, dearest, first and last. */
export interface CostGroup {
  key: string;
  name: string;
  products: string[];
  purchases: number;
  grams: number;
  cents: number;
  per_gram: number;
  min: number;
  max: number;
  first: number;
  last: number;
  first_at: string;
  last_at: string;
}

/** One product on the map of the strains: placed by profile, coloured by effect. */
/** A product like another: how alike, 0 to 1, and the terpenes the two share. */
export interface Likeness {
  slug: string;
  score: number;
  shared: string[];
}

/** One product on the map of the strains, placed by its terpene profile when it has one. */
export interface Strain {
  slug: string;
  name: string;
  x: number;
  y: number;
  placed: boolean;
  thc: number;
  cbd: number;
  terpenes: string[];
  /** Its taste, family by family, the strongest 1. */
  aroma: Partial<Record<string, number>>;
  /** Its strongest taste family. */
  lead?: string;
  /** Its three strongest flavours in words: "Lemon, pepper and musk". */
  taste?: string;
  dispensed: number;
  overall?: number;
  ratings: number;
  like: Likeness[];
}

/** One terpene the shelf names: what it tastes of, and what its products' sessions were rated. */
export interface Terpene {
  name: string;
  flavors: string[];
  family?: string;
  notes?: string;
  effects: string[];
  boiling_point?: number;
  products: number;
  leads: number;
  /** The mean overall rating of its products' sessions, the lead counting most. */
  overall?: number;
  rated: number;
}

/** The repository being served, and the home folder to browse from. */
export interface Place {
  path: string;
  name: string;
  entries: number;
  home: string;
}

/** A folder, as the repository browser shows it. */
export interface Listing {
  path: string;
  parent?: string;
  repo: boolean;
  dirs: { name: string; path: string; repo: boolean }[];
}

/** What a workbook holds, as wits import reports it, and whether it was written. */
export interface Verdict {
  sheets: number;
  products: number;
  entries: number;
  dispensed: number;
  ground: number;
  fills: number;
  grinds: number;
  first?: string;
  last?: string;
  merged: { slug: string; names: string[] }[];
  anomalies: string[];
  committed: boolean;
}

/** What a pharmacy's invoice says, each line named as the jar it already is where it is one. Cents throughout. */
export interface InvoiceReading {
  /** The pharmacy, as the invoice names it. */
  seller?: string;
  date?: string;
  number?: string;
  lines: { name: string; slug?: string; printed: string; grams: number; per_gram_cents: number; total_cents: number }[];
  net_cents: number;
  vat_cents: number;
  rate: number;
  gross_cents: number;
  sum_cents: number;
  /** What it charged for besides flower by the gram — a vape by the piece, shipping — left out of the fill. */
  other: { name: string; total_cents: number }[];
  /** The fill already recorded that the invoice is for, if one is: priced from it, not recorded again. */
  match?: InvoiceMatch;
}

/**
 * A recorded fill holding an invoice's strains near its date: `days` apart (0
 * the same day), and for each line of the invoice the purchase it is, or ''.
 */
export interface InvoiceMatch {
  fill: number;
  start: string;
  days: number;
  lines: string[];
  priced: number;
}

/** One purchase of a fill being priced: a price a gram, or an invoice's line total; blank keeps no price. */
export interface PriceLine {
  purchase: string;
  per_gram?: string;
  total?: string;
  /** None to be had: written down so the purchase is asked about no more. */
  unknown?: boolean;
  /** The price a gram has its VAT in it already: none is added. */
  with_vat?: boolean;
  note?: string;
}

/** What pricing a fill would write, rehearsed by the server (?dry=1): each line's price and the VAT in it. */
export interface PricedPreview {
  lines: { purchase: string; cents: number; vat_cents: number }[];
  message: string;
}

/** One product of a fill as the form sends it: a line's total, or a price a gram, or neither. */
export interface FillLine {
  name: string;
  grams: number;
  per_gram?: string;
  total?: string;
  /** The price a gram has its VAT in it already: none is added. */
  with_vat?: boolean;
  /** Beside the price: the invoice it is from. */
  note?: string;
}

/** One purchase of a product, with its price when one is written down. */
export interface Pickup {
  hash: string;
  at: string;
  grams: number;
  cents?: number;
  per_gram?: number;
  unknown?: boolean;
  priced: boolean;
  seller?: string;
  note?: string;
}

/** One fill of a product — the fill's scope: what it dispensed of it, what is still sealed. */
export interface JarFill {
  number: number;
  start: string;
  open: boolean;
  dispensed: number;
  sealed: number;
  pickups: Pickup[];
}

/** One product in full — the jar's scope, with each of its fills in theirs. */
export interface JarDetail extends Jar {
  country?: string;
  radiated?: boolean;
  strain: string;
  first_seen: string;
  /** Through a device as logged sessions; cleared from the stash unlogged. */
  seshed: number;
  flushed: number;
  fill_list: JarFill[];
  price: { cents: number; priced: number; per_gram: number; unpriced: number };
  months: { month: string; ground: number; seshed: number }[];
  sessions: {
    count: number;
    grams: number;
    first?: string;
    last?: string;
    devices: { slug: string; name: string; sessions: number; grams: number }[];
    min_temp?: number;
    max_temp?: number;
    rated: number;
    scores: Partial<Record<Dimension, number>>;
    /** The latest sessions, newest first; a hash and scores when rated. */
    recent: Rated[];
  };
  like: (Likeness & { name: string })[];
  taste?: string;
  aroma: Partial<Record<string, number>>;
}

/** A page that names a product's terpenes: a search result, or the strain's profile on Leafly. */
export interface TerpeneSource {
  site: string;
  title: string;
  url: string;
  terpenes: string[];
  /** About the strain, not this product: another grower's harvest can run differently. */
  strain?: boolean;
}

/** What a terpene lookup found, and a profile from it, dominant first. Nothing is written down. */
export interface Findings {
  search: string;
  sources: TerpeneSource[];
  suggested: string[];
}

/** The year on one card, as wits review gathers it: every figure the year's own. */
export interface YearReview {
  year: number;
  years: number[];
  from: string;
  to: string;
  days: number;
  running: boolean;
  ground: number;
  grind_days: number;
  per_grind_day: number;
  sessions: number;
  seshed: number;
  dispensed: number;
  pickups: number;
  products: number;
  months: number[];
  daily: Day[];
  strains: { name: string; ground: number; share: number }[];
  busiest: Day | null;
  break: { from: string; to: string; days: number; breaks: number } | null;
  best_temperature: { label: string; mean: number; count: number } | null;
  best_strain: { label: string; mean: number; count: number } | null;
  device: { name: string; sessions: number; seshed: number } | null;
  terpene: { name: string; flavors: string[] | null; share: number } | null;
  /** Cents. */
  paid: number;
  priced: number;
  /** Cents a gram. */
  per_gram: number;
  dearest: YearBought | null;
  cheapest: YearBought | null;
  bought: YearBought[];
}

export interface YearBought {
  at: string;
  name: string;
  seller: string;
  grams: number;
  cents: number;
  per_gram: number;
}

/** What to take on a trip, as wits pack plans it, signed for exactly that. */
export interface Packing {
  days: number;
  margin: number;
  mode: 'even' | 'oldest';
  rate: number;
  need: number;
  packed: number;
  shelf: number;
  short: boolean;
  lines: {
    name: string;
    stash: number;
    sealed: number;
    held_stash: number;
    held_sealed: number;
    days: number;
    fills: { day: string; grams: number }[];
  }[];
  statement: string;
  url: string;
  fingerprint: string;
  qr: string;
}

/** The day so far, as wits today says it: THC and CBD in milligrams, in the material put through a device. */
export interface DayToday {
  day: string;
  sessions: {
    hash: string;
    at: string;
    name: string;
    grams: number;
    thc_pct: number;
    cbd_pct: number;
    thc_mg: number;
    cbd_mg: number;
    known: boolean;
    device: string;
    temperature: number;
    mode: string;
    overall: number;
  }[];
  seshed: number;
  thc_mg: number;
  cbd_mg: number;
  unknown: number;
  ground: number;
  grinds: { name: string; grams: number }[];
  picked: { name: string; grams: number }[];
  avb_collected: number;
  avb_used: number;
  strains: { name: string; sessions: number; grams: number; thc_mg: number }[];
  devices: { name: string; sessions: number; grams: number; thc_mg: number }[];
  usual: { days: number; seshed: number; thc_mg: number };
}
