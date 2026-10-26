import { describe, expect, it } from 'vitest';

import { routes } from '../app.routes';
import { PAGES, TABS } from './pages';

describe('the tabs', () => {
  it('are four, and every page is in one of them, in the order h and l step', () => {
    expect(TABS.map((t) => t.label)).toEqual(['Dashboard', 'I forgot to', 'Shelf', 'Insights']);
    expect(PAGES.map((p) => p.path)).toEqual([
      '/', '/forgot', '/shelf/jars', '/shelf/journal', '/shelf/devices', '/shelf/trip',
      '/insights/analysis', '/insights/effects', '/insights/history', '/insights/costs', '/insights/year',
    ]);
  });

  it('keep the old addresses leading to their pages', () => {
    const moved: Record<string, string> = {};
    for (const r of routes) {
      if (r.path && typeof r.redirectTo === 'string') {
        moved[r.path] = r.redirectTo;
      }
    }
    expect(moved).toMatchObject({
      jars: 'shelf/jars', journal: 'shelf/journal', analysis: 'insights/analysis',
      effects: 'insights/effects', history: 'insights/history', costs: 'insights/costs',
    });
  });
});
