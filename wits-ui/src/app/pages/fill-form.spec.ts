import { describe, expect, it } from 'vitest';

import { lineOf } from './fill-form';

describe('lineOf', () => {
  it('sends a price a gram as typed, for the server to price, VAT and all', () => {
    expect(lineOf({ id: 1, name: ' Aurora 26/1 Pink Kush ', grams: 15, perGram: 8.5 })).toEqual({
      name: 'Aurora 26/1 Pink Kush', grams: 15, total: undefined, per_gram: '8.5', with_vat: undefined, note: undefined,
    });
  });

  it('says a price has its VAT in it already, as last time’s and an invoice’s have', () => {
    expect(lineOf({ id: 1, name: 'X', grams: 20, perGram: 5.8, inclVAT: true }).with_vat).toBe(true);
  });

  it('keeps an invoice’s total as it was printed, with its invoice beside it', () => {
    const l = lineOf({ id: 1, name: 'X', grams: 20, perGram: 5.84, total: 11680, inclVAT: true, invoice: 'R-12' });
    expect(l.total).toBe('116.80');
    expect(l.per_gram).toBeUndefined();
    expect(l.note).toBe('invoice R-12');
  });
});
