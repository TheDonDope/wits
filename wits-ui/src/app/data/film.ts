// The replay as a film: the record cut into evenly spaced moments, each
// folded the way the replay folds it (derive.ts), so a film of three years
// spends as long on a quiet month as on a busy one.

import { dayOf } from './derive';
import { Entry } from './models';

/** One jar in a frame: what it holds, sealed and ground, in grams. */
export interface FilmJar {
  slug: string;
  storage: number;
  stash: number;
}

/** One frame: the day it shows, and the jars on the shelf that day. */
export interface FilmFrame {
  day: string;
  shelf: number;
  jars: FilmJar[];
}

const cg = (g: number) => Math.round(g * 100);

/**
 * The frames of a film of the entries: count moments evenly from the first
 * entry to the last, each with the jars that hold sealed grams then — the
 * shelf. A stash the record never drew down stays out of it: the imported
 * history has no sessions, and its stashes only ever grow. Jars keep the
 * order they first arrived in, so none jumps about as others come and go.
 */
export function filmFrames(entries: Entry[], count: number): FilmFrame[] {
  const sorted = [...entries].sort((a, b) => Date.parse(a.occurred_at) - Date.parse(b.occurred_at));
  if (!sorted.length || count < 1) {
    return [];
  }
  const first = Date.parse(sorted[0].occurred_at);
  const span = Math.max(Date.parse(sorted[sorted.length - 1].occurred_at) - first, 1);
  const held = new Map<string, { storage: number; stash: number }>();
  const arrived: string[] = [];
  const frames: FilmFrame[] = [];
  let next = 0;
  for (let f = 0; f < count; f++) {
    const until = first + (span * f) / Math.max(count - 1, 1);
    while (next < sorted.length && Date.parse(sorted[next].occurred_at) <= until) {
      apply(held, arrived, sorted[next]);
      next++;
    }
    const jars = arrived
      .map((slug) => ({ slug, ...(held.get(slug) ?? { storage: 0, stash: 0 }) }))
      .filter((j) => j.storage > 0)
      .map((j) => ({ slug: j.slug, storage: j.storage / 100, stash: Math.max(j.stash, 0) / 100 }));
    const last = sorted[Math.max(next - 1, 0)];
    frames.push({ day: dayOf(last), shelf: jars.reduce((s, j) => s + cg(j.storage), 0) / 100, jars });
  }
  return frames;
}

/** Folds one entry into the running balances, in centigrams. */
function apply(held: Map<string, { storage: number; stash: number }>, arrived: string[], e: Entry): void {
  if (!e.product) {
    return;
  }
  let a = held.get(e.product);
  if (!a) {
    a = { storage: 0, stash: 0 };
    held.set(e.product, a);
    arrived.push(e.product);
  }
  const g = cg(e.grams);
  if (e.from === 'storage' || e.from === 'stash') {
    a[e.from] -= g;
  }
  if (e.to === 'storage' || e.to === 'stash') {
    a[e.to] += g;
  }
}

/** The most each product ever held sealed: how tall its jar is drawn. */
export function capacities(frames: FilmFrame[]): Map<string, number> {
  const out = new Map<string, number>();
  for (const f of frames) {
    for (const j of f.jars) {
      out.set(j.slug, Math.max(out.get(j.slug) ?? 0, j.storage + j.stash));
    }
  }
  return out;
}
