import { Signal, computed, signal } from '@angular/core';

import { skippable } from './derive';
import { Entry } from './models';

/** The autoplay intervals, slowest first — the terminal's playSpeeds. */
export const SPEEDS = [800, 400, 200, 100, 50];

/**
 * The ledger played back: everything a screen shows is derived from an
 * append-only log, so any of it can start from empty and grow the way the
 * record grew. A port of the terminal's player — the same rules, so a replay
 * here and one there tell the same story — with a scrubber added.
 *
 * The playhead counts the entries applied; -1 is live, everything applied.
 * Adjustments ride along silently: applied with whatever they follow, never
 * stopped on, never narrated.
 */
export class Replay {
  readonly playhead = signal(-1);
  readonly playing = signal(false);
  readonly speed = signal(2);

  private timer: ReturnType<typeof setTimeout> | undefined;

  constructor(private readonly events: Signal<Entry[]>) {}

  /** The entries applied: all of them when live. */
  readonly played = computed(() => {
    const at = this.playhead();
    const all = this.events();
    return at < 0 ? all : all.slice(0, Math.min(at, all.length));
  });

  readonly live = computed(() => this.playhead() < 0);

  /** How many entries there are to play. */
  readonly size = computed(() => this.events().length);

  /** The newest entry that took the stage — never a silent one. */
  readonly current = computed<Entry | null>(() => {
    if (this.live()) {
      return null;
    }
    const played = this.played();
    for (let i = played.length - 1; i >= 0; i--) {
      if (!skippable(played[i])) {
        return played[i];
      }
    }
    return null;
  });

  /** How many entries the story counts, and how far into it the head is. */
  readonly position = computed(() => {
    const count = (es: Entry[]) => es.filter((e) => !skippable(e)).length;
    return { at: count(this.played()), of: count(this.events()) };
  });

  /** Starts from empty when live or played out, or pauses a running replay. */
  toggle(): void {
    if (this.playing()) {
      this.pause();
      return;
    }
    const total = this.events().length;
    if (!total) {
      return;
    }
    const at = this.playhead();
    if (at < 0 || at >= total) {
      this.playhead.set(0);
    }
    this.playing.set(true);
    this.schedule();
  }

  pause(): void {
    this.playing.set(false);
    clearTimeout(this.timer);
  }

  /** One entry on or back, pausing, passing over the silent ones. */
  step(delta: 1 | -1): void {
    const all = this.events();
    const total = all.length;
    if (!total) {
      return;
    }
    this.pause();
    let at = this.playhead() < 0 ? total : this.playhead();
    at += delta;
    while (at > 0 && at < total && skippable(all[at - 1])) {
      at += delta;
    }
    this.playhead.set(at >= total ? -1 : Math.max(at, 0));
  }

  /** Moves the head to an entry, as the scrubber does; the end is live. */
  scrub(at: number): void {
    this.pause();
    this.playhead.set(at >= this.events().length ? -1 : Math.max(at, 0));
  }

  /**
   * Stands the head at the end of a day: the record as it was that evening.
   * A day after the last entry is live.
   */
  scrubTo(day: string): void {
    const all = this.events();
    const at = all.findIndex((e) => e.occurred_at.slice(0, 10) > day);
    this.scrub(at < 0 ? all.length : at);
  }

  /** Back to now. */
  goLive(): void {
    this.pause();
    this.playhead.set(-1);
  }

  retune(delta: 1 | -1): void {
    this.speed.update((s) => Math.min(Math.max(s + delta, 0), SPEEDS.length - 1));
  }

  /** Stops the clock; a page calls this when it goes. */
  destroy(): void {
    clearTimeout(this.timer);
  }

  private schedule(): void {
    clearTimeout(this.timer);
    this.timer = setTimeout(() => {
      this.advance();
    }, SPEEDS[this.speed()]);
  }

  private advance(): void {
    if (!this.playing()) {
      return;
    }
    const all = this.events();
    let at = this.playhead() + 1;
    while (at < all.length && skippable(all[at - 1])) {
      at++;
    }
    if (at >= all.length) {
      this.playing.set(false);
      this.playhead.set(-1);
      return;
    }
    this.playhead.set(at);
    this.schedule();
  }
}
