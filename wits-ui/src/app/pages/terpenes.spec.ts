import { describe, expect, it } from 'vitest';

import { splitTerpenes } from './terpenes';

describe('splitTerpenes', () => {
  it('reads a typed profile as the server takes it, dominant first', () => {
    expect(splitTerpenes(' Myrcene, Limonene;Caryophyllene ,, ')).toEqual(['Myrcene', 'Limonene', 'Caryophyllene']);
    expect(splitTerpenes('')).toEqual([]);
  });
});
