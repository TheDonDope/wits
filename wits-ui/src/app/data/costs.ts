import { Bought, CostGroup } from './models';

/**
 * Each pharmacy's series — its colour and shape on every chart — by the order
 * it was first bought from, so a pharmacy keeps its look whatever is shown.
 * A price typed in, with no pharmacy, is -1.
 */
export function sellerSeries(bought: Bought[]): Map<string, number> {
  const out = new Map<string, number>();
  for (const b of bought) {
    if (b.seller && !out.has(b.seller)) {
      out.set(b.seller, out.size);
    }
  }
  return out;
}

/** The price a gram (cents) over some purchases, weighted by their grams. */
export function perGram(bought: Bought[]): number {
  const grams = bought.reduce((s, b) => s + b.grams, 0);
  return grams > 0 ? bought.reduce((s, b) => s + b.cents, 0) / grams : 0;
}

/**
 * The price a gram lately: the purchases of the last `days` before the
 * newest, or — fewer than three of them — the last three fills'.
 */
export function lately(bought: Bought[], days = 90): { cents: number; over: string } {
  const last = bought.at(-1);
  if (!last) {
    return { cents: 0, over: '' };
  }
  const since = new Date(last.at).getTime() - days * 86_400_000;
  const recent = bought.filter((b) => new Date(b.at).getTime() >= since);
  if (recent.length >= 3) {
    return { cents: perGram(recent), over: `the last ${days} days` };
  }
  const fills = [...new Set(bought.map((b) => b.fill))].slice(-3);
  return { cents: perGram(bought.filter((b) => fills.includes(b.fill))), over: 'the last three fills' };
}

/** A fill's price a gram, placed at its first purchase: the trend the dots stand around. */
export function fillTrend(bought: Bought[]): { x: number; y: number }[] {
  const byFill = new Map<number, Bought[]>();
  for (const b of bought) {
    byFill.set(b.fill, [...(byFill.get(b.fill) ?? []), b]);
  }
  return [...byFill.values()].map((bs) => ({ x: new Date(bs[0].at).getTime(), y: perGram(bs) }));
}

/** Strength bands, THC %, each with what a gram cost in it. */
export const BANDS = [
  { label: 'under 20 %', lo: 0, hi: 20 },
  { label: '20–24 %', lo: 20, hi: 25 },
  { label: '25–29 %', lo: 25, hi: 30 },
  { label: '30 % and up', lo: 30, hi: Infinity },
];

export function bands(bought: Bought[]): { label: string; cents: number; grams: number; purchases: number }[] {
  return BANDS.map((band) => {
    const inBand = bought.filter((b) => b.thc >= band.lo && b.thc < band.hi && b.thc > 0);
    return { label: band.label, cents: perGram(inBand), grams: inBand.reduce((s, b) => s + b.grams, 0), purchases: inBand.length };
  }).filter((b) => b.purchases > 0);
}

/** How the price a gram moved from a group's first purchase to its last, as a fraction; 0 for one purchase. */
export function change(g: CostGroup): number {
  return g.purchases > 1 && g.first > 0 ? (g.last - g.first) / g.first : 0;
}

/** What a strain table is sorted by. */
export type StrainSort = 'name' | 'purchases' | 'grams' | 'cents' | 'per_gram' | 'change' | 'last_at';

export function sortStrains(groups: CostGroup[], by: StrainSort, up: boolean): CostGroup[] {
  const value = (g: CostGroup): number | string =>
    by === 'name' ? g.name.toLowerCase() : by === 'change' ? change(g) : by === 'last_at' ? g.last_at : g[by];
  return [...groups].sort((a, b) => {
    const va = value(a);
    const vb = value(b);
    const c = va < vb ? -1 : va > vb ? 1 : 0;
    return up ? c : -c;
  });
}

/** A month of supply at a rate a day and a price a gram, in cents. */
export function month(gramsADay: number, centsAGram: number): number {
  return Math.round(gramsADay * 30 * centsAGram);
}

/** A cents figure as euros, in the page's locale. */
export function euros(cents: number, digits = 2): string {
  return (cents / 100).toLocaleString(undefined, { style: 'currency', currency: 'EUR', minimumFractionDigits: digits, maximumFractionDigits: digits });
}
