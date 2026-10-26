import { SymbolType, symbolCircle, symbolDiamond, symbolSquare, symbolStar, symbolTriangle } from 'd3-shape';

/**
 * What terpenes taste of, as the server says it (pkg/cannabis): the eight
 * families a taste wheel goes round, each with a colour of the theme's, and
 * the key two labels agree on for one terpene.
 */

/** One taste family: its id as the server sends it, a label, and its colour. */
export interface Family {
  id: string;
  label: string;
  color: string;
}

/**
 * The eight, round the wheel from the bright to the deep. The colours are
 * the theme's own, mixed where it has fewer than eight, so every theme
 * keeps them apart.
 */
export const FAMILIES: Family[] = [
  { id: 'citrus', label: 'citrus', color: 'var(--w-avb)' },
  { id: 'fruity', label: 'fruity', color: 'var(--w-warn)' },
  { id: 'floral', label: 'floral', color: 'var(--w-sesh)' },
  { id: 'sweet', label: 'sweet', color: 'color-mix(in oklab, var(--w-warn) 45%, var(--w-sesh))' },
  { id: 'herbal', label: 'herbal', color: 'var(--w-accent)' },
  { id: 'woody', label: 'pine & wood', color: 'var(--w-stash)' },
  { id: 'earthy', label: 'earthy', color: 'color-mix(in oklab, var(--w-avb) 40%, var(--w-dim))' },
  { id: 'spicy', label: 'spicy', color: 'color-mix(in oklab, var(--w-warn) 55%, var(--w-avb))' },
];

/** A family's colour, or the dim one for none. */
export function familyColor(id?: string): string {
  return FAMILIES.find((f) => f.id === id)?.color ?? 'var(--w-dim)';
}

/** A family's label, or nothing for none. */
export function familyLabel(id?: string): string {
  return FAMILIES.find((f) => f.id === id)?.label ?? '';
}

/** A strain's place on the map by its strongest taste: five groups of the eight families. */
export interface Group {
  id: string;
  label: string;
  color: string;
  /** The marker shape: identity never rests on colour alone. */
  symbol: SymbolType;
  families: string[];
}

/**
 * The map's five groups. Eight colours could not be kept apart in every
 * theme, and the dashboard palette's green and rose are one to a red-green
 * colour-blind eye, so each group has a shape of its own as well as a colour,
 * and is named wherever it shows.
 */
export const GROUPS: Group[] = [
  { id: 'citrus', label: 'citrus', color: 'var(--w-avb)', symbol: symbolCircle, families: ['citrus'] },
  { id: 'floral', label: 'fruity & floral', color: 'var(--w-sesh)', symbol: symbolDiamond, families: ['fruity', 'floral', 'sweet'] },
  { id: 'pine', label: 'herbal & pine', color: 'var(--w-stash)', symbol: symbolTriangle, families: ['herbal', 'woody'] },
  { id: 'earthy', label: 'earthy', color: 'var(--w-dim)', symbol: symbolSquare, families: ['earthy'] },
  { id: 'spicy', label: 'spicy', color: 'var(--w-warn)', symbol: symbolStar, families: ['spicy'] },
];

/** The map's group a strongest taste falls in. */
export function groupOf(lead?: string): Group | undefined {
  return GROUPS.find((g) => lead !== undefined && g.families.includes(lead));
}

const PREFIXES = new Set(['alpha', 'beta', 'gamma', 'delta', 'trans', 'cis', 'para', 'α', 'β', 'γ', 'δ']);

/**
 * A terpene's name as two labels agree on it, as cannabis.Key says: its
 * longest word less isomer prefixes, lowercase, without the e German leaves
 * off — "β-Myrcen" and "myrcene" are one.
 */
export function terpeneKey(name: string): string {
  let best = '';
  for (const w of name.toLowerCase().split(/[^\p{L}]+/u)) {
    if (!PREFIXES.has(w) && w.length > 1 && w.length > best.length) {
      best = w;
    }
  }
  return best.replace(/e$/, '');
}
