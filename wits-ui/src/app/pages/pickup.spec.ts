import { describe, expect, it } from 'vitest';

import { InvoiceReading, UnpricedEntry } from '../data/models';
import { byFill, fillCents, fromInvoice, priceLines } from './pickup';

function bought(hash: string, fill: number, product: string, grams = 20): UnpricedEntry {
  return {
    hash, fill, fill_start: `2026-08-${String(fill - 1).padStart(2, '0')}T10:00:00Z`, type: 'purchase',
    occurred_at: '', recorded_at: '', product, product_name: product, grams, from: '', to: 'storage',
  };
}

const sun = bought('a', 30, 'nice-morning-sun-321');
const doughnut = bought('b', 30, 'all-nations-mac-doughnut-291');
const papaya = bought('c', 30, 'sumo-papaya-bomb-251');

describe('a fill priced together', () => {
  it('groups purchases by the fill they came in, newest first', () => {
    const fills = byFill([sun, bought('x', 29, 'enua'), doughnut, papaya]);
    expect(fills.map((f) => [f.fill, f.lines.length, f.grams])).toEqual([[30, 3, 60], [29, 1, 20]]);
  });

  it('works out the prices as the server does: 20 g at 5,80 € with VAT is 138,04 €', () => {
    const sums = fillCents([sun, doughnut, papaya], { a: { perGram: 5.8 }, b: { perGram: 5.84 } }, true);
    expect(sums.lines).toEqual({ a: 13804, b: 13899, c: 0 });
    expect(sums.typed).toBe(23280);
    expect(sums.vat).toBe(27703 - 23280);
    expect(sums.total).toBe(27703);
    expect(priceLines([sun, papaya], { a: { perGram: 5.8 } }, true)).toEqual([
      { purchase: 'a', per_gram: '5.8' }, { purchase: 'c', per_gram: '' }]);
  });

  it('takes an invoice’s lines by the match, at its totals, with no VAT added', () => {
    const reading: InvoiceReading = {
      date: '2026-08-28', number: '2608', net_cents: 0, vat_cents: 0, rate: 19, gross_cents: 35180, sum_cents: 35180, other: [],
      lines: [
        { name: 'All Nations 29/1 Mac Doughnut', slug: doughnut.product, printed: 'All Nations GM 29/1 Mac Doughnut', grams: 20, per_gram_cents: 584, total_cents: 11680 },
        { name: 'NICE 32/1 Morning Sun', slug: sun.product, printed: 'NICE 32/1 MOS Morning Sun', grams: 20, per_gram_cents: 580, total_cents: 11600 },
        { name: 'Unknown', printed: 'Something else', grams: 10, per_gram_cents: 900, total_cents: 9000 },
      ],
      match: { fill: 30, start: '2026-08-29T10:00:00Z', days: 1, lines: ['b', 'a', ''], priced: 0 },
    };
    const { priced, notes } = fromInvoice(byFill([sun, doughnut, papaya])[0], reading, true);
    expect(Object.keys(priced).sort()).toEqual(['a', 'b']);
    expect(notes).toEqual(['Something else is not in this fill']);
    const sums = fillCents([sun, doughnut, papaya], priced, true);
    expect(sums.invoice).toBe(23280);
    expect(sums.vat).toBe(0);
    expect(priceLines([doughnut], priced, true)).toEqual([{ purchase: 'b', total: '116.80', note: 'invoice 2608' }]);
  });
});

describe('a pickup with no price to be had', () => {
  it('marks what is left without a price, and keeps what was given', () => {
    expect(priceLines([sun, doughnut], { a: { perGram: 5.8 } }, true, true)).toEqual([
      { purchase: 'a', per_gram: '5.8' }, { purchase: 'b', unknown: true }]);
    expect(priceLines([doughnut], {}, true, false)).toEqual([{ purchase: 'b', per_gram: '' }]);
  });
});
