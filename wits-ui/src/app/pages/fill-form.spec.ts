import { describe, expect, it } from 'vitest';

import { rowCents } from './fill-form';

describe('rowCents', () => {
  it('works a price out as the server does, so the form shows what is written down', () => {
    // 30 g at 2.05 € with VAT is 7318.5 cents: worked in euros first it rounded to 7318, the server's 7319.
    expect(rowCents({ id: 1, name: 'X', grams: 30, perGram: 2.05 }, true)).toBe(7319);
    expect(rowCents({ id: 1, name: 'X', grams: 10, perGram: 5 }, true)).toBe(5950);
    expect(rowCents({ id: 1, name: 'X', grams: 10, perGram: 5 }, false)).toBe(5000);
  });

  it('keeps an invoice’s total, and never adds VAT to an invoice’s prices', () => {
    expect(rowCents({ id: 1, name: 'X', grams: 20, perGram: 5.84, total: 11680, inclVAT: true }, true)).toBe(11680);
    expect(rowCents({ id: 1, name: 'X', grams: 10, perGram: 5.84, inclVAT: true }, true)).toBe(5840);
  });
});
