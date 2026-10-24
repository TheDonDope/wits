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
