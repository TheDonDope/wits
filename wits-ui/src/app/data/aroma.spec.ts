import { describe, expect, it } from 'vitest';

import { FAMILIES, GROUPS, familyColor, groupOf, terpeneKey } from './aroma';

describe('terpeneKey', () => {
  it('keys a terpene as the server does, whatever the label calls it', () => {
    expect(terpeneKey('β-Myrcen')).toBe(terpeneKey('myrcene'));
    expect(terpeneKey('Trans-Caryophyllen')).toBe('caryophyllen');
    expect(terpeneKey('Alpha-Pinen')).toBe('pinen');
    expect(terpeneKey('Δ-3-Carene')).toBe('caren');
    expect(terpeneKey('Linalool')).not.toBe(terpeneKey('Limonen'));
  });
});

describe('familyColor', () => {
  it('gives a family the theme colour, and none the dim one', () => {
    expect(familyColor('citrus')).toBe('var(--w-avb)');
    expect(familyColor(undefined)).toBe('var(--w-dim)');
  });
});

describe('groupOf', () => {
  it('puts every family in exactly one group', () => {
    for (const f of FAMILIES) {
      expect(GROUPS.filter((g) => g.families.includes(f.id))).toHaveLength(1);
    }
    expect(groupOf('sweet')?.label).toBe('fruity & floral');
    expect(groupOf(undefined)).toBeUndefined();
  });
});
