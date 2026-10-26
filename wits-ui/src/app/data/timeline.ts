import { skippable } from './derive';
import { Entry } from './models';

/** The whole record laid along one line in time, for the replay's scrubber. */
export interface Timeline {
  /** The first and last instant, ms since the epoch. */
  start: number;
  end: number;
  /** Grams ground in each slice of time, the heaviest 1. */
  bars: number[];
  /** Where each pickup stands, 0 to 1; a fill of several strains is one. */
  picks: number[];
  /** Where each year begins, or each month on a span shorter than 14 months, 0 to 1. */
  marks: { at: number; label: string }[];
}

const MS = (iso: string) => Date.parse(iso);

const DAY = 86_400_000;
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

/**
 * Lays the entries out in time, the ground grams in slices: a day each on a
 * short span, never more than 120 on a long one.
 */
export function timeline(events: readonly Entry[]): Timeline {
  if (!events.length) {
    return { start: 0, end: 1, bars: [], picks: [], marks: [] };
  }
  const start = MS(events[0].occurred_at);
  const end = Math.max(MS(events[events.length - 1].occurred_at), start + 1);
  const slices = Math.min(Math.max(Math.ceil((end - start) / DAY), 30), 120);
  const at = (iso: string) => (MS(iso) - start) / (end - start);
  const sums = new Array<number>(slices).fill(0);
  const picks: number[] = [];
  for (const e of events) {
    const x = at(e.occurred_at);
    if (e.type === 'grind') {
      sums[Math.min(Math.floor(x * slices), slices - 1)] += e.grams;
    } else if (e.type === 'purchase' && !picks.some((p) => Math.abs(p - x) < 0.004)) {
      picks.push(x);
    }
  }
  const top = Math.max(...sums, 1e-9);
  return { start, end, bars: sums.map((s) => s / top), picks, marks: marks(start, end, at) };
}

/** The year boundaries on a long span, the months on a short one, January named with its year. */
function marks(start: number, end: number, at: (iso: string) => number): Timeline['marks'] {
  const monthly = end - start < 425 * DAY;
  const first = new Date(start);
  // Months counted from year 0, so stepping one is adding one.
  let n = first.getFullYear() * 12 + (monthly ? first.getMonth() + 1 : 12);
  const out: Timeline['marks'] = [];
  let iso = monthStart(n);
  while (MS(iso) <= end) {
    out.push({ at: at(iso), label: markLabel(n, monthly) });
    n += monthly ? 1 : 12;
    iso = monthStart(n);
  }
  return out;
}

function monthStart(n: number): string {
  return `${String(Math.floor(n / 12))}-${String((n % 12) + 1).padStart(2, '0')}-01T00:00:00`;
}

function markLabel(n: number, monthly: boolean): string {
  const year = String(Math.floor(n / 12));
  if (!monthly) {
    return year;
  }
  return n % 12 === 0 ? `Jan ${year}` : MONTHS[n % 12];
}

/** Where an instant stands on the timeline, 0 to 1. */
export function place(t: Timeline, iso: string): number {
  return Math.min(Math.max((MS(iso) - t.start) / (t.end - t.start), 0), 1);
}

/** The day a point on the timeline falls on, as YYYY-MM-DD in local time. */
export function dayAt(t: Timeline, x: number): string {
  const d = new Date(t.start + Math.min(Math.max(x, 0), 1) * (t.end - t.start));
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${String(d.getFullYear())}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

/** The newest `n` entries that tell the story, newest first: the tape. */
export function tape(played: readonly Entry[], n: number): Entry[] {
  const out: Entry[] = [];
  for (let i = played.length - 1; i >= 0 && out.length < n; i--) {
    if (!skippable(played[i])) {
      out.push(played[i]);
    }
  }
  return out;
}
