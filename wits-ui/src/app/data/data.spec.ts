import { DestroyRef, signal } from '@angular/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { balances, breaks, byProduct, movingAverage, perDay, summarise, tolerance } from './derive';
import { capacities, filmFrames } from './film';
import { Binding, Keys } from './keys';
import { Entry } from './models';
import { Replay, SPEEDS } from './replay';

function entry(type: Entry['type'], day: string, grams: number, product = 'wcake', from = '', to = ''): Entry {
  const flows: Record<string, [string, string]> = {
    purchase: ['external', 'storage'],
    grind: ['storage', 'stash'],
    sesh: ['stash', 'consumed'],
    adjust: [from, to],
  };
  const [f, t] = flows[type] ?? ['', ''];
  return {
    hash: `${type}-${day}-${grams}-${Math.random()}`,
    type,
    occurred_at: `${day}T12:00:00+02:00`,
    recorded_at: `${day}T12:00:00+02:00`,
    product,
    product_name: product.toUpperCase(),
    grams,
    from: f,
    to: t,
  };
}

const record: Entry[] = [
  entry('purchase', '2026-07-01', 20),
  entry('grind', '2026-07-02', 0.3),
  entry('grind', '2026-07-02', 0.45),
  entry('sesh', '2026-07-02', 0.5),
  entry('adjust', '2026-07-03', 1, 'wcake', 'storage', 'external'),
  entry('grind', '2026-07-05', 1.25),
  entry('purchase', '2026-07-05', 10, 'lemon'),
  entry('grind', '2026-07-06', 2, 'lemon'),
];

describe('derive', () => {
  it('folds the balances as the ledger does, to the centigram', () => {
    const b = balances(record);
    // 20 − 0.3 − 0.45 − 1 (weighed out) − 1.25; 0.1 + 0.2 in floating point is not 0.3.
    expect(b.get('wcake')).toEqual({ storage: 17, stash: 1.5, avb: 0 });
    expect(b.get('lemon')).toEqual({ storage: 8, stash: 2, avb: 0 });
  });

  it('summarises the grinds as ledger.Summarise does', () => {
    const s = summarise(record);
    expect(s.ground).toBe(4);
    expect(s.activeDays).toBe(3);
    expect(s.elapsedDays).toBe(5);
    expect(s.perActiveDay).toBe(1.33);
    expect(s.perElapsedDay).toBe(0.8);
    expect(s.median).toBe(1.25); // the days were 0.75, 1.25, 2
  });

  it('counts nothing that was undone, though the balances still fold it', () => {
    const mistake = { ...entry('grind', '2026-07-06', 5), corrected: true };
    const fix = { ...entry('adjust', '2026-07-06', 5, 'wcake', 'stash', 'storage'), reverts: mistake.hash };
    const withUndo = [...record, mistake, fix];

    expect(summarise(withUndo)).toEqual(summarise(record));
    expect(perDay(withUndo)).toEqual(perDay(record));
    expect(byProduct(withUndo)).toEqual(byProduct(record));
    expect(balances(withUndo)).toEqual(balances(record));
  });

  it('totals each day, and each product heaviest first', () => {
    expect(perDay(record).map((d) => [d.date, d.ground, d.seshed])).toEqual([
      ['2026-07-02', 0.75, 0.5],
      ['2026-07-05', 1.25, 0],
      ['2026-07-06', 2, 0],
    ]);
    // A tie, 2 g each: the order falls to the slug, so it never flickers.
    expect(byProduct(record).map((p) => [p.slug, p.grams, p.days])).toEqual([
      ['lemon', 2, 1],
      ['wcake', 2, 2],
    ]);
  });

  it('averages over a trailing window', () => {
    expect(movingAverage([2, 4, 6, 8], 2)).toEqual([2, 3, 5, 7]);
  });
});

describe('Replay', () => {
  let r: Replay;
  beforeEach(() => {
    vi.useFakeTimers();
    r = new Replay(signal(record));
  });
  afterEach(() => {
    r.destroy();
    vi.useRealTimers();
  });

  it('plays from empty, an entry a tick, and settles back to live', () => {
    expect(r.live()).toBe(true);
    r.toggle();
    expect(r.playhead()).toBe(0);
    vi.advanceTimersByTime(SPEEDS[r.speed()]);
    expect(r.playhead()).toBe(1);
    expect(r.current()?.type).toBe('purchase');
    vi.advanceTimersByTime(SPEEDS[r.speed()] * 20);
    expect(r.live()).toBe(true);
    expect(r.playing()).toBe(false);
  });

  it('never stops on an adjustment: it rides along with what it follows', () => {
    r.scrub(4); // after the session, before the adjustment
    r.step(1);
    expect(r.playhead()).toBe(6); // the adjustment was applied in passing
    expect(r.current()?.type).toBe('grind');
    r.step(-1);
    expect(r.playhead()).toBe(4);
    expect(r.position()).toEqual({ at: 4, of: 7 });
  });

  it('stands at the end of a day', () => {
    r.scrubTo('2026-07-02');
    expect(r.played().length).toBe(4);
    r.scrubTo('2027-01-01');
    expect(r.live()).toBe(true);
  });

  it('steps past the end into live, and retunes within its speeds', () => {
    r.scrub(7);
    r.step(1);
    expect(r.live()).toBe(true);
    for (let i = 0; i < 10; i++) {
      r.retune(1);
    }
    expect(r.speed()).toBe(SPEEDS.length - 1);
  });
});

describe('Keys', () => {
  function destroyRef() {
    const hooks: (() => void)[] = [];
    return {
      ref: { onDestroy: (f: () => void) => hooks.push(f) } as unknown as DestroyRef,
      destroy: () => {
        hooks.forEach((f) => {
          f();
        });
      },
    };
  }
  const press = (key: string, target: EventTarget = document.body) => {
    const ev = new KeyboardEvent('keydown', { key });
    Object.defineProperty(ev, 'target', { value: target });
    return ev;
  };

  it('lets the page answer a key the shell also claims, and gives it back when the page goes', () => {
    const keys = new Keys();
    const ran: string[] = [];
    const bind = (name: string): Binding[] => [{ keys: ['s'], help: name, run: () => ran.push(name) }];
    keys.register('Everywhere', () => bind('session'), destroyRef().ref);
    const page = destroyRef();
    keys.register('Analysis', () => bind('scope'), page.ref);

    keys.handle(press('s'));
    expect(ran).toEqual(['scope']);
    expect(keys.footer().map((b) => b.help)).toEqual(['scope']);

    page.destroy();
    keys.handle(press('s'));
    expect(ran).toEqual(['scope', 'session']);
  });

  it('leaves alone a key typed into a field, and keeps quiet keys off the footer', () => {
    const keys = new Keys();
    let runs = 0;
    keys.register('Page', () => [
      { keys: ['j'], help: 'move', run: () => runs++ },
      { keys: ['k'], help: 'up', quiet: true, run: () => runs++ },
    ], destroyRef().ref);

    keys.handle(press('j', document.createElement('input')));
    expect(runs).toBe(0);
    keys.handle(press('k'));
    expect(runs).toBe(1);
    expect(keys.footer().map((b) => b.help)).toEqual(['move']);
  });
});

describe('tolerance', () => {
  const day = (n: number) => `2026-07-${String(n).padStart(2, '0')}`;
  const sesh = (n: number, hash: string): Entry => ({ ...entry('sesh', day(n), 0.5), hash });

  it('rolls grams a day and the overall rating over the window', () => {
    const entries = [entry('grind', day(1), 2), entry('grind', day(2), 1), sesh(2, 'a'), sesh(3, 'b')];
    const curve = tolerance(entries, new Map([['a', 4], ['b', 2]]), day(1), day(3));

    expect(curve.map((p) => p.grams)).toEqual([2, 1.5, 1]);
    expect(curve[0].rating).toBeNull();
    expect(curve[2].rating).toBe(3);
    expect(curve[2].ratings).toBe(2);
  });

  it('leaves out what was undone', () => {
    const undone = { ...entry('grind', day(1), 5), corrected: true };
    expect(tolerance([undone], new Map(), day(1), day(1))[0].grams).toBe(0);
  });

  it('drops a day out of the window once it is past it', () => {
    const curve = tolerance([entry('grind', day(1), 14)], new Map(), day(1), day(16));
    expect(curve[13].grams).toBe(1);
    expect(curve[14].grams).toBe(0);
  });

  it('marks a week and more without use as a break, one still going included', () => {
    const entries = [entry('grind', day(1), 1), entry('grind', day(5), 1), entry('grind', day(13), 1), sesh(14, 'a')];
    expect(breaks(entries, day(1), day(22))).toEqual([
      { from: day(6), to: day(12), days: 7 },
      { from: day(15), to: day(22), days: 8 },
    ]);
  });

  it('counts a session as use, though nothing was ground', () => {
    const entries = [entry('grind', day(1), 1), sesh(5, 'a'), entry('grind', day(10), 1)];
    expect(breaks(entries, day(1), day(10))).toEqual([]);
  });
});

describe('filmFrames', () => {
  const at = (n: number) => `2026-07-${String(n).padStart(2, '0')}`;
  const buy = (n: number, product: string, g: number) => entry('purchase', at(n), g, product);
  const grind = (n: number, product: string, g: number) => entry('grind', at(n), g, product);

  it('spaces its moments evenly from the first entry to the last', () => {
    const frames = filmFrames([buy(1, 'a', 20), grind(11, 'a', 5), grind(21, 'a', 5)], 3);
    expect(frames.map((f) => f.day)).toEqual([at(1), at(11), at(21)]);
    expect(frames.map((f) => f.shelf)).toEqual([20, 15, 10]);
  });

  it('shows the shelf, each jar sealed and ground, and lets an emptied jar go', () => {
    const frames = filmFrames([buy(1, 'a', 10), buy(2, 'b', 10), grind(3, 'a', 10)], 3);
    expect(frames[2].jars).toEqual([{ slug: 'b', storage: 10, stash: 0 }]);
    const middle = filmFrames([buy(1, 'a', 10), grind(2, 'a', 4), grind(3, 'a', 1)], 3)[1];
    expect(middle.jars).toEqual([{ slug: 'a', storage: 6, stash: 4 }]);
  });

  it('draws each jar against the most it ever held', () => {
    const frames = filmFrames([buy(1, 'a', 10), grind(2, 'a', 4), buy(3, 'a', 10)], 3);
    expect(capacities(frames).get('a')).toBe(20);
  });
});
