import { describe, expect, it } from 'vitest';

import { rows } from './invoice';

describe('rows', () => {
  it('gathers a PDF’s text into rows top to bottom, cells left to right, apart by tabs', () => {
    const items = [
      { str: '116,80 €', x: 500, y: 600 },
      { str: 'All Nations GM 29/1 Mac Doughnut', x: 60, y: 600.8 },
      { str: '20 g', x: 380, y: 599.5 },
      { str: 'Name', x: 60, y: 620 },
      { str: '5,84 €', x: 440, y: 600 },
      { str: 'Gesamtbetrag inkl. USt', x: 380, y: 540 },
    ];

    expect(rows(items)).toBe(
      'Name\nAll Nations GM 29/1 Mac Doughnut\t20 g\t5,84 €\t116,80 €\nGesamtbetrag inkl. USt',
    );
  });
});
