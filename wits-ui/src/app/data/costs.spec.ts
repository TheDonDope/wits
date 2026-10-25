import { describe, expect, it } from 'vitest';

import { spark } from '../pages/cost-strains';
import { bands, change, fillTrend, lately, month, perGram, sellerSeries, sortStrains } from './costs';
import { Bought, CostGroup } from './models';

function buy(at: string, fill: number, grams: number, cents: number, seller = '', thc = 0): Bought {
  return { hash: `${at}-${fill}`, at, fill, product: 'p', name: 'P', maker: 'M', strain: 's', strain_name: 'S', thc, cbd: 1,
    grams, cents, per_gram: cents / grams, seller };
}

describe('what the costs page works out', () => {
  const bought = [
    buy('2024-01-10', 1, 10, 10000, 'Paunsdorf', 20),
    buy('2025-01-10', 2, 20, 12000, '360Grad', 26),
    buy('2026-05-01', 3, 20, 13000, 'Nordostbahnhof', 31),
    buy('2026-06-01', 4, 10, 6000, 'Nordostbahnhof', 22),
    buy('2026-07-01', 5, 10, 5000, '', 18),
  ];

  it('keeps each pharmacy in its series by when it was first bought from, none for typed in', () => {
    expect([...sellerSeries(bought)]).toEqual([['Paunsdorf', 0], ['360Grad', 1], ['Nordostbahnhof', 2]]);
  });

  it('weights a price a gram by the grams', () => {
    expect(perGram(bought.slice(0, 2))).toBe(22000 / 30);
  });

  it('takes lately as the last 90 days, or the last three fills when that is fewer than three purchases', () => {
    expect(lately(bought)).toEqual({ cents: perGram(bought.slice(2)), over: 'the last 90 days' });
    expect(lately(bought.slice(0, 3)).over).toBe('the last three fills');
  });

  it('draws a fill’s price a gram at its first purchase', () => {
    expect(fillTrend(bought).map((t) => t.y)).toEqual([1000, 600, 650, 600, 500]);
  });

  it('bands purchases by THC', () => {
    expect(bands(bought).map((b) => [b.label, b.purchases])).toEqual([['under 20 %', 1], ['20–24 %', 2], ['25–29 %', 1], ['30 % and up', 1]]);
  });

  it('prices a month at a rate a day', () => {
    expect(month(1.93, 651)).toBe(Math.round(1.93 * 30 * 651));
  });

  it('says how a strain’s price moved, and sorts by it', () => {
    const g = (key: string, first: number, last: number, purchases = 2): CostGroup => ({
      key, name: key, products: [], purchases, grams: 10, cents: 0, per_gram: 0, min: 0, max: 0, first, last, first_at: '', last_at: '',
    });
    expect(change(g('a', 1000, 700))).toBeCloseTo(-0.3);
    expect(change(g('b', 1000, 700, 1))).toBe(0);
    expect(sortStrains([g('a', 1000, 700), g('c', 500, 600)], 'change', false).map((x) => x.key)).toEqual(['c', 'a']);
  });

  it('draws a sparkline in its box, none for one price', () => {
    expect(spark([1000])).toBe('');
    expect(spark([1000, 500], 84, 22)).toBe('2.0,2.0 82.0,20.0');
  });
});
