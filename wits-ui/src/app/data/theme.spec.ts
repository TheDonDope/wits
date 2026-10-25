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
    expect(THEMES.length).toBe(21);
  });
});
