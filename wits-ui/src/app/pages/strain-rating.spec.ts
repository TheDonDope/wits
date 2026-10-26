import { describe, expect, it } from 'vitest';

import { ratingLine } from './strain-rating';

describe('ratingLine', () => {
  it('says a strain rating as a line, leaving out what was not rated', () => {
    expect(ratingLine({ overall: 4, taste: 5, again: 'yes' })).toBe('overall 4, taste 5 · again');
    expect(ratingLine({ aroma: 2, again: 'no' })).toBe('aroma 2 · not again');
    expect(ratingLine({ note: 'just a line' })).toBe('');
    expect(ratingLine(undefined)).toBe('');
  });
});
