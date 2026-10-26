import { TestBed } from '@angular/core/testing';
import { beforeEach, describe, expect, it } from 'vitest';

import { THEMES, Theme } from './theme';

describe('Theme', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('starts in the first theme, and puts the choice on the page and in storage', () => {
    const theme = TestBed.inject(Theme);
    expect(theme.current()).toBe('aurora');
    theme.set('paper');
    TestBed.tick();
    expect(document.documentElement.dataset['theme']).toBe('paper');
    expect(localStorage.getItem('wits.theme')).toBe('paper');
  });

  it('steps round the list both ways', () => {
    const theme = TestBed.inject(Theme);
    theme.step(-1);
    expect(theme.current()).toBe(THEMES[THEMES.length - 1].id);
    theme.step(1);
    expect(theme.current()).toBe(THEMES[0].id);
  });

  it('names every theme once', () => {
    expect(new Set(THEMES.map((t) => t.id)).size).toBe(THEMES.length);
    expect(THEMES.length).toBe(41);
  });
});

describe('the themes', () => {
  it('are the ones themes.scss defines, in its order', async () => {
    const { readFile } = await import('node:fs/promises');
    const scss = await readFile('src/themes.scss', 'utf8');
    const block = scss.slice(scss.indexOf('$themes: ('), scss.indexOf('@function w-pick'));
    const defined = [...block.matchAll(/^ {2}([a-z-]+): \(/gm)].map((m) => m[1]);
    expect(THEMES.map((t) => t.id)).toEqual(defined);
  });
});

/** WCAG's contrast ratio between two #rrggbb colours. */
function contrast(a: number[], b: number[]): number {
  const lum = (c: number[]) => {
    const [r, g, bl] = c.map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
    return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
  };
  const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

const rgb = (hex: string) => {
  const h = hex.length === 4 ? hex.slice(1).replace(/(.)/g, '$1$1') : hex.slice(1);
  return [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255);
};
const mix = (a: number[], b: number[], p: number) => a.map((v, i) => v * p + b[i] * (1 - p));

describe('every theme', () => {
  it('sets its dim text at 4.5:1 or more on its background and its cards', async () => {
    const { readFile } = await import('node:fs/promises');
    const scss = await readFile('src/themes.scss', 'utf8');
    const block = scss.slice(scss.indexOf('$themes: ('), scss.indexOf('@function w-pick'));
    const short: string[] = [];
    for (const [, name, body] of block.matchAll(/^ {2}([a-z-]+): \(([\s\S]*?)^ {2}\),/gm)) {
      const kv: Partial<Record<string, string>> = Object.fromEntries(
        [...body.matchAll(/([a-z-]+):\s*(#[0-9a-fA-F]{3,6}|[a-z]+)/g)].map((m) => [m[1], m[2]]));
      const [ink, fg, dim] = [rgb(kv['ink'] ?? ''), rgb(kv['fg'] ?? ''), rgb(kv['dim'] ?? '')];
      const dark = kv['scheme'] !== 'light';
      // A card's colour, as the theme mixin makes it when a theme gives none.
      const given = kv['panel'];
      const panel = given?.startsWith('#') ? rgb(given) : dark ? mix(fg, ink, 0.09) : mix([1, 1, 1], ink, 0.72);
      const worst = Math.min(contrast(dim, ink), contrast(dim, panel));
      if (worst < 4.5) {
        short.push(`${name} ${worst.toFixed(2)}`);
      }
    }
    expect(short).toEqual([]);
  });
});
