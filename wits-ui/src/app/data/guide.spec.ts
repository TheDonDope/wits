import { describe, expect, it } from 'vitest';

import { routes } from '../app.routes';
import { GUIDE } from './guide';
import { PAGES } from './pages';

/** Every string of prose a section shows. */
const prose = GUIDE.flatMap((s) => [s.lead, ...(s.body ?? []), ...(s.steps ?? []), ...(s.then ?? []), s.tip ?? '',
  ...(s.cli ?? []).map((e) => e.note ?? '')]);

describe('the user guide', () => {
  it('names each section once', () => {
    const ids = GUIDE.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('links only to sections it has', () => {
    const ids = new Set(GUIDE.map((s) => s.id));
    const links = prose.flatMap((p) => [...p.matchAll(/href="#([a-z-]+)"/g)].map((m) => m[1]));
    expect(links.length).toBeGreaterThan(0);
    expect(links.filter((l) => !ids.has(l))).toEqual([]);
  });

  it('opens only pages there are', () => {
    const paths = new Set(['/', ...PAGES.map((p) => p.path), ...routes.map((r) => `/${r.path ?? ''}`)]);
    const opened = GUIDE.flatMap((s) => (s.page ? [s.page.path] : []));
    expect(opened.filter((p) => !paths.has(p))).toEqual([]);
  });

  it('shows only pictures that are in public/guide', async () => {
    const { readdir } = await import('node:fs/promises');
    const there = new Set((await readdir('public/guide')).map((f) => `guide/${f}`));
    const shots = GUIDE.flatMap((s) => [...(s.shots ?? []), ...(s.tuiShot ? [s.tuiShot] : [])]);
    expect(shots.filter((sh) => !there.has(sh.src)).map((sh) => sh.src)).toEqual([]);
  });

  it('gives every section the same done in a terminal', () => {
    const without = GUIDE.filter((s) => !s.cli?.length && !s.tui?.length).map((s) => s.id);
    expect(without).toEqual([]);
  });
});
