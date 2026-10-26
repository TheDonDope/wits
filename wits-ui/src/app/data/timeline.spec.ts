import { describe, expect, it } from 'vitest';

import { Entry } from './models';
import { dayAt, place, tape, timeline } from './timeline';

const entry = (type: Entry['type'], occurred_at: string, grams = 1, hash = occurred_at + type): Entry =>
  ({ hash, type, occurred_at, recorded_at: occurred_at, grams, from: '', to: '' });

describe('the replay timeline', () => {
  const fill = [
    entry('purchase', '2026-08-29T10:00:00'),
    entry('purchase', '2026-08-29T10:05:00'),
    entry('grind', '2026-08-30T20:00:00', 0.5),
    entry('grind', '2026-08-30T21:00:00', 0.25),
    entry('grind', '2026-10-25T20:00:00', 1.5),
  ];

  it('gives a short span a slice a day, the heaviest day 1', () => {
    const t = timeline(fill);
    expect(t.bars.length).toBe(58); // 57.4 days, a slice each
    expect(Math.max(...t.bars)).toBe(1);
    expect(t.bars[1]).toBeCloseTo(0.75 / 1.5);
  });

  it('counts a fill of several strains as one pickup', () => {
    expect(timeline(fill).picks).toEqual([0]);
  });

  it('marks the months on a short span and the years on a long one', () => {
    expect(timeline(fill).marks.map((m) => m.label)).toEqual(['Sep', 'Oct']);
    const years = timeline([entry('grind', '2023-10-09T12:00:00'), entry('grind', '2026-10-25T12:00:00')]);
    expect(years.marks.map((m) => m.label)).toEqual(['2024', '2025', '2026']);
    expect(years.bars.length).toBe(120);
  });

  it('turns a point back into the day it falls on', () => {
    const t = timeline(fill);
    expect(dayAt(t, 0)).toBe('2026-08-29');
    expect(dayAt(t, 1)).toBe('2026-10-25');
    expect(place(t, '2026-08-29T10:00:00')).toBe(0);
  });

  it('keeps the newest entries for the tape, newest first, adjustments left out', () => {
    const played = [...fill, entry('adjust', '2026-10-26T01:00:00')];
    expect(tape(played, 2).map((e) => e.occurred_at)).toEqual(['2026-10-25T20:00:00', '2026-08-30T21:00:00']);
  });
});
