// What the screens compute from a run of entries, in the browser — so a replay
// can show any prefix of the record without asking the server for every frame.
// Each mirrors the Go it is named after, adding in centigrams the way the
// ledger does, so a figure here is the figure wits status gives.

import { Day, Entry } from './models';

/** Grams to centigrams, to the nearest. */
const cg = (g: number) => Math.round(g * 100);

/** The calendar day an entry happened on, as the server wrote it. */
export const dayOf = (e: Entry) => e.occurred_at.slice(0, 10);

/**
 * Adjustments ride along silently in a replay: they are applied, so the
 * balances stay true, but never get a frame of their own — the terminal's
 * skippable().
 */
export const skippable = (e: Entry) => e.type === 'adjust';

export interface Held {
  storage: number;
  stash: number;
  avb: number;
}

/** Every product's accounts after the entries — ledger.Fold's balances. */
export function balances(entries: Entry[]): Map<string, Held> {
  const acc = new Map<string, { storage: number; stash: number; avb: number }>();
  const slot = (product: string) => {
    let a = acc.get(product);
    if (!a) {
      a = { storage: 0, stash: 0, avb: 0 };
      acc.set(product, a);
    }
    return a;
  };
  for (const e of entries) {
    if (!e.product) {
      continue;
    }
    const a = slot(e.product);
    const g = cg(e.grams);
    if (e.from === 'storage' || e.from === 'stash' || e.from === 'avb') {
      a[e.from] -= g;
    }
    if (e.to === 'storage' || e.to === 'stash' || e.to === 'avb') {
      a[e.to] += g;
    }
  }
  const out = new Map<string, Held>();
  for (const [p, a] of acc) {
    out.set(p, { storage: a.storage / 100, stash: a.stash / 100, avb: a.avb / 100 });
  }
  return out;
}

export interface Stats {
  ground: number;
  activeDays: number;
  elapsedDays: number;
  perActiveDay: number;
  perElapsedDay: number;
  median: number;
}

/** The grinds' statistics — ledger.Summarise. */
export function summarise(entries: Entry[]): Stats {
  const perDay = new Map<string, number>();
  let ground = 0;
  let first = '';
  let last = '';
  for (const e of entries) {
    if (e.type !== 'grind') {
      continue;
    }
    const d = dayOf(e);
    perDay.set(d, (perDay.get(d) ?? 0) + cg(e.grams));
    ground += cg(e.grams);
    if (!first || d < first) {
      first = d;
    }
    if (d > last) {
      last = d;
    }
  }
  const amounts = [...perDay.values()].filter((v) => v > 0).sort((a, b) => a - b);
  const active = amounts.length;
  const elapsed = first ? daysBetween(first, last) + 1 : 0;
  const round = (g: number) => Math.round(g * 100) / 100;
  return {
    ground: ground / 100,
    activeDays: active,
    elapsedDays: elapsed,
    perActiveDay: active ? round(ground / 100 / active) : 0,
    perElapsedDay: elapsed ? round(ground / 100 / elapsed) : 0,
    median: median(amounts) / 100,
  };
}

function median(sorted: number[]): number {
  const n = sorted.length;
  if (!n) {
    return 0;
  }
  return n % 2 ? sorted[(n - 1) / 2] : Math.round((sorted[n / 2 - 1] + sorted[n / 2]) / 2);
}

/** Whole days from one calendar date to another. */
export function daysBetween(from: string, to: string): number {
  const [a, b] = [from, to].map((s) => {
    const [y, m, d] = s.split('-').map(Number);
    return Date.UTC(y, m - 1, d);
  });
  return Math.round((b - a) / 86400000);
}

/** Each calendar day from one date to another, inclusive. */
export function eachDay(from: string, to: string): string[] {
  const out: string[] = [];
  const [y, m, d] = from.split('-').map(Number);
  const at = new Date(Date.UTC(y, m - 1, d));
  for (let i = 0; i <= daysBetween(from, to); i++) {
    out.push(at.toISOString().slice(0, 10));
    at.setUTCDate(at.getUTCDate() + 1);
  }
  return out;
}

/** Ground and put through a device, per day — the calendars' and the chart's. */
export function perDay(entries: Entry[]): Day[] {
  const ground = new Map<string, number>();
  const seshed = new Map<string, number>();
  for (const e of entries) {
    const d = dayOf(e);
    if (e.type === 'grind') {
      ground.set(d, (ground.get(d) ?? 0) + cg(e.grams));
    } else if (e.type === 'sesh') {
      seshed.set(d, (seshed.get(d) ?? 0) + cg(e.grams));
    }
  }
  const days = [...new Set([...ground.keys(), ...seshed.keys()])].sort();
  return days.map((date) => ({
    date,
    ground: (ground.get(date) ?? 0) / 100,
    seshed: (seshed.get(date) ?? 0) / 100,
  }));
}

/** A trailing mean over a window of days — the habit under the spikes. */
export function movingAverage(values: number[], window: number): number[] {
  let sum = 0;
  return values.map((v, i) => {
    sum += v;
    if (i >= window) {
      sum -= values[i - window];
    }
    return sum / Math.min(i + 1, window);
  });
}

export interface ProductTotal {
  slug: string;
  name: string;
  grams: number;
  days: number;
}

/** Grams ground per product, heaviest first — the terminal's grindTotals. */
export function byProduct(entries: Entry[]): ProductTotal[] {
  const totals = new Map<string, { name: string; grams: number; days: Set<string> }>();
  for (const e of entries) {
    if (e.type !== 'grind' || !e.product) {
      continue;
    }
    let t = totals.get(e.product);
    if (!t) {
      t = { name: e.product_name ?? e.product, grams: 0, days: new Set() };
      totals.set(e.product, t);
    }
    t.grams += cg(e.grams);
    t.days.add(dayOf(e));
  }
  return [...totals.entries()]
    .map(([slug, t]) => ({ slug, name: t.name, grams: t.grams / 100, days: t.days.size }))
    .sort((a, b) => b.grams - a.grams || a.slug.localeCompare(b.slug));
}

/** How many days the tolerance curve's rolling means run over. */
export const TOLERANCE_WINDOW = 14;
/**
 * How many days without use make a break. Not three: the workbook grinds every
 * two or three days, a few grams lasting a few days, and at three its 90 days
 * held eleven "breaks" that were only the habit. A week stands out from it.
 */
export const BREAK_DAYS = 7;

/** One day of the tolerance curve: grams a day and the effect, both rolling. */
export interface TolerancePoint {
  date: string;
  /** Grams ground per calendar day over the window ending that day. */
  grams: number;
  /** The mean overall rating of the window's rated sessions, or null for none. */
  rating: number | null;
  ratings: number;
}

/** A run of days without grinding. */
export interface Break {
  from: string;
  to: string;
  days: number;
}

/**
 * Tolerance, day by day from one date to another: grams a day against the
 * effect, both over the trailing window. Creep is the grams climbing while
 * the rating stays flat. An undone entry counts in neither; a session counts
 * by its latest overall rating, from overall.
 */
export function tolerance(entries: Entry[], overall: Map<string, number>, from: string, to: string): TolerancePoint[] {
  const days = eachDay(from, to);
  const index = new Map(days.map((d, i) => [d, i]));
  const ground = new Array<number>(days.length).fill(0);
  const sums = new Array<number>(days.length).fill(0);
  const counts = new Array<number>(days.length).fill(0);
  for (const e of entries) {
    const i = index.get(dayOf(e));
    if (i === undefined || e.corrected || e.type === 'adjust') {
      continue;
    }
    if (e.type === 'grind') {
      ground[i] += cg(e.grams);
    }
    const r = e.type === 'sesh' ? overall.get(e.hash) : undefined;
    if (r !== undefined) {
      sums[i] += r;
      counts[i]++;
    }
  }
  const out: TolerancePoint[] = [];
  let g = 0;
  let s = 0;
  let n = 0;
  for (let i = 0; i < days.length; i++) {
    g += ground[i];
    s += sums[i];
    n += counts[i];
    if (i >= TOLERANCE_WINDOW) {
      g -= ground[i - TOLERANCE_WINDOW];
      s -= sums[i - TOLERANCE_WINDOW];
      n -= counts[i - TOLERANCE_WINDOW];
    }
    const span = Math.min(i + 1, TOLERANCE_WINDOW);
    out.push({ date: days[i], grams: g / 100 / span, rating: n ? s / n : null, ratings: n });
  }
  return out;
}

/**
 * The runs of BREAK_DAYS or more days without use between one date and
 * another — nothing ground and no session — a run still going at the end
 * included: a break in progress.
 */
export function breaks(entries: Entry[], from: string, to: string): Break[] {
  const ground = new Set(entries.filter((e) => (e.type === 'grind' || e.type === 'sesh') && !e.corrected).map(dayOf));
  const out: Break[] = [];
  let start = '';
  let run = 0;
  const close = (end: string) => {
    if (run >= BREAK_DAYS) {
      out.push({ from: start, to: end, days: run });
    }
    run = 0;
  };
  const days = eachDay(from, to);
  days.forEach((d, i) => {
    if (ground.has(d)) {
      close(days[i - 1] ?? d);
      return;
    }
    if (run === 0) {
      start = d;
    }
    run++;
  });
  close(days[days.length - 1] ?? to);
  return out;
}
