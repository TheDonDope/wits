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
  shelf: Shelf;
}

/** One product across its lifetime — the jar's scope. */
export interface Jar {
  slug: string;
  name: string;
  manufacturer?: string;
  cultivar?: string;
  thc?: number;
  cbd?: number;
  storage: number;
  stash: number;
  avb: number;
  bought: number;
  ground: number;
  fills: number;
  last_seen: string;
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

export interface Device {
  slug: string;
  name: string;
  kind?: string;
  min_temp?: number;
  max_temp?: number;
  default_temp?: number;
  sessions: number;
  grams: number;
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
}

/** One product on the map of the strains: placed by profile, coloured by effect. */
export interface Strain {
  slug: string;
  name: string;
  x: number;
  y: number;
  thc: number;
  cbd: number;
  terpenes: string[];
  dispensed: number;
  overall?: number;
  ratings: number;
  like: string[];
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
  note?: string;
}

/** One product of a fill as the form sends it: a line's total, or a price a gram, or neither. */
export interface FillLine {
  name: string;
  grams: number;
  per_gram?: string;
  total?: string;
  /** Beside the price: the invoice it is from. */
  note?: string;
}
