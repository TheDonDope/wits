import { describe, expect, it } from 'vitest';

import { InvoiceReading } from '../data/models';
import { Read, found, invoiceLines, newestFirst, tickable } from './invoice-batch';

function reading(date: string, lines: string[], priced = 0): InvoiceReading {
  return {
    date, number: `INV-${date}`, net_cents: 0, vat_cents: 0, rate: 19, gross_cents: 0, sum_cents: 0, other: [],
    lines: lines.map((_, i) => ({ name: `strain ${i}`, printed: `strain ${i}`, grams: 20, per_gram_cents: 580, total_cents: 11600 + i })),
    match: { fill: 30, start: '2026-08-29T10:00:00Z', days: 1, lines, priced },
  };
}

const file = new File([], 'x.pdf');

describe('a batch of invoices', () => {
  it('ticks an invoice that matched a fill not yet priced', () => {
    expect(tickable({ file, reading: reading('2026-08-28', ['a', 'b']), ticked: false })).toBe(true);
    expect(tickable({ file, reading: reading('2026-08-28', ['a', 'b'], 2), ticked: false })).toBe(false);
    expect(tickable({ file, reading: reading('2026-08-28', ['', '']), ticked: false })).toBe(false);
    expect(tickable({ file, error: 'not a pdf', ticked: false })).toBe(false);
  });

  it('prices only the lines its match found, at the invoice’s totals', () => {
    const r = reading('2024-12-04', ['', 'b']);
    expect(found({ file, reading: r, ticked: true })).toBe(1);
    expect(invoiceLines(r)).toEqual([{ purchase: 'b', total: '116.01', note: 'invoice INV-2024-12-04' }]);
  });

  it('lists the newest invoice first, one not read last', () => {
    const rs: Read[] = [
      { file, reading: reading('2024-12-02', []), ticked: false },
      { file, error: 'no', ticked: false },
      { file, reading: reading('2026-08-28', []), ticked: false },
    ];
    expect(rs.sort(newestFirst).map((r) => r.reading?.date ?? 'none')).toEqual(['2026-08-28', '2024-12-02', 'none']);
  });
});
