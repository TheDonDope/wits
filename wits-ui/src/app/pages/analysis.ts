import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, computed, effect, inject, signal, untracked } from '@angular/core';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { ActivatedRoute } from '@angular/router';

import { breaks, byProduct, dayOf, perDay, summarise, tolerance } from '../data/derive';
import { Keys } from '../data/keys';
import { Cycle, Entry } from '../data/models';
import { Replay } from '../data/replay';
import { Wits } from '../data/wits';
import { BarRace, RaceItem } from '../viz/bar-race';
import { CountUp } from '../viz/count-up';
import { DailyChart } from '../viz/daily-chart';
import { FillRiver } from '../viz/fill-river';
import { Heatmap } from '../viz/heatmap';
import { ToleranceCurve } from '../viz/tolerance-curve';
import { Transport, replayKeys } from '../viz/transport';

/** The terminal's scopes, and how far back each reaches. */
const SCOPES = [
  { key: 'fill', label: 'This fill', days: 0 },
  { key: '30', label: '30 days', days: 30 },
  { key: '90', label: '90 days', days: 90 },
  { key: '365', label: '12 months', days: 365 },
  { key: 'all', label: 'All time', days: -1 },
] as const;

/**
 * The long view, and the record played back through it: how this fill is
 * going, how it compares, where the grams went — and, pressing play, how it
 * all came to be, from empty, an entry at a time.
 */
@Component({
  selector: 'w-analysis',
  imports: [BarRace, CountUp, DailyChart, DecimalPipe, FillRiver, Heatmap, MatButtonToggleModule, ToleranceCurve, Transport],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="glass head">
      <div>
        <span class="eyebrow">analysis</span>
        <h1>{{ scope().label }} <span class="dim">{{ range().from }} → {{ range().to }}</span></h1>
      </div>
      <mat-button-toggle-group [value]="scope().key" (change)="pick($event.value)" aria-label="Scope">
        @for (s of scopes; track s.key) {
          <mat-button-toggle [value]="s.key">{{ s.label }}</mat-button-toggle>
        }
      </mat-button-toggle-group>
    </section>

    <w-transport [replay]="replay" />

    <section class="tiles">
      <div class="glass tile">
        <span class="eyebrow">ground</span>
        <span class="v num"><w-count [value]="stats().ground" [decimals]="1" [duration]="450" /> g</span>
      </div>
      <div class="glass tile">
        <span class="eyebrow">active days</span>
        <span class="v num"><w-count [value]="stats().activeDays" [decimals]="0" [duration]="450" /></span>
        <span class="dim">of {{ stats().elapsedDays }} elapsed</span>
      </div>
      <div class="glass tile">
        <span class="eyebrow">per active day</span>
        <span class="v num"><w-count [value]="stats().perActiveDay" [duration]="450" /> g</span>
        <span class="dim">median {{ stats().median | number: '1.2-2' }} g</span>
      </div>
      <div class="glass tile">
        <span class="eyebrow">per elapsed day</span>
        <span class="v num"><w-count [value]="stats().perElapsedDay" [duration]="450" /> g</span>
      </div>
    </section>

    <section class="glass pad">
      <span class="eyebrow">grams per day</span>
      <w-daily [days]="days()" [from]="range().from" [to]="range().to" />
    </section>

    <section class="glass pad">
      <header>
        <div>
          <span class="eyebrow">tolerance</span>
          <p class="dim">Grams a day against how the sessions went. Tolerance creeps when the grams climb and the
            rating stays flat; a break shows whether the curve comes back down.</p>
        </div>
        @if (gaps().length) {
          <span class="dim">{{ gaps().length }} {{ gaps().length === 1 ? 'break' : 'breaks' }}, the longest {{ longest() }} days</span>
        }
      </header>
      <w-tolerance [points]="curve()" [breaks]="gaps()" [until]="until()" />
    </section>

    <section class="glass pad">
      <header>
        <span class="eyebrow">by product</span>
        @if (scope().key === 'fill') {
          <span class="key"><i style="background: var(--w-stash)"></i>this fill
            <i style="background: var(--w-avb)"></i>older jars</span>
        }
      </header>
      <w-bar-race [items]="race()" />
    </section>

    @if (scope().key !== 'fill') {
      <section class="glass pad">
        <span class="eyebrow">rhythm</span>
        <w-heatmap [days]="days()" [weeks]="weeks()" tone="var(--w-stash)" />
      </section>
    }

    @if (scope().days === 365 || scope().days === -1) {
      <section class="glass pad">
        <header>
          <span class="eyebrow">fills</span>
          <span class="scope">the fill</span>
        </header>
        <w-fill-river [cycles]="cycles()" [until]="moment()" />
      </section>
    }
  `,
  styles: `
    :host { display: grid; gap: 20px; }
    .head { padding: 18px 24px; display: flex; flex-wrap: wrap; gap: 18px; align-items: center; justify-content: space-between; }
    h1 { font: 600 28px var(--w-num); margin: 4px 0 0; }
    h1 .dim { font: 400 14px 'Inter Variable', sans-serif; margin-left: 8px; }
    .tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 18px; }
    .tile { padding: 18px 22px; display: grid; gap: 4px; }
    .v { font-size: 30px; font-weight: 600; }
    .pad { padding: 20px 24px; display: grid; gap: 12px; }
    header { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; }
    header p { margin: 6px 0 0; font-size: 13px; max-width: 70ch; }
    .dim { color: var(--w-dim); }
    .key { display: flex; gap: 8px; align-items: center; font-size: 12px; color: var(--w-dim); }
    .key i { width: 10px; height: 10px; border-radius: 3px; display: inline-block; margin-left: 8px; }
  `,
})
export class Analysis {
  private readonly wits = inject(Wits);
  protected readonly scopes = SCOPES;
  protected readonly scope = signal<(typeof SCOPES)[number]>(SCOPES[0]);

  /** Every entry that has happened, oldest first. */
  private readonly history = computed(() =>
    [...this.wits.everything.value().entries].reverse(),
  );

  /** The entries the scope covers. */
  private readonly scoped = computed<Entry[]>(() => {
    const s = this.scope();
    const all = this.history();
    if (s.key === 'fill') {
      const start = this.wits.overview.value()?.fill?.start;
      // Instants, not strings: the server writes each time with its offset.
      return start ? all.filter((e) => Date.parse(e.occurred_at) >= Date.parse(start)) : [];
    }
    if (s.days < 0) {
      return all;
    }
    const cutoff = Date.now() - s.days * 86400000;
    return all.filter((e) => Date.parse(e.occurred_at) > cutoff);
  });

  protected readonly replay = new Replay(this.scoped);
  private readonly played = this.replay.played;

  protected readonly range = computed(() => {
    const es = this.scoped();
    const today = new Date().toISOString().slice(0, 10);
    return { from: es.length ? dayOf(es[0]) : today, to: today };
  });

  protected readonly stats = computed(() => summarise(this.played()));
  protected readonly days = computed(() => perDay(this.played()));

  /** Each session's latest overall rating, by its hash. */
  private readonly overall = computed(() => {
    const out = new Map<string, number>();
    for (const r of this.wits.effects.value()?.rated ?? []) {
      if (r.scores.overall !== undefined) {
        out.set(r.session.hash, r.scores.overall);
      }
    }
    return out;
  });

  /** The last day the replay has reached, or null when live. */
  protected readonly until = computed(() => {
    const e = this.replay.current();
    return e ? dayOf(e) : null;
  });

  protected readonly curve = computed(() => tolerance(this.played(), this.overall(), this.range().from, this.range().to));
  protected readonly gaps = computed(() => breaks(this.played(), this.range().from, this.until() ?? this.range().to));
  protected readonly longest = computed(() => Math.max(0, ...this.gaps().map((b) => b.days)));

  protected readonly weeks = computed(() => ({ fill: 9, '30': 5, '90': 14, '365': 53, all: null })[this.scope().key]);

  /** Where the replay stands in time; null when live. */
  protected readonly moment = computed(() => {
    const e = this.replay.current();
    return e ? new Date(e.occurred_at) : null;
  });

  protected readonly race = computed<RaceItem[]>(() => {
    const totals = byProduct(this.played());
    const ground = totals.reduce((s, t) => s + t.grams, 0) || 1;
    const own = new Set(this.wits.overview.value()?.fill?.products.map((p) => p.slug) ?? []);
    const fill = this.scope().key === 'fill';
    return totals.map((t) => ({
      key: t.slug,
      label: t.name,
      value: t.grams,
      note: `${((t.grams / ground) * 100).toFixed(0)}% · ${t.days} ${t.days === 1 ? 'day' : 'days'} · ${(t.grams / Math.max(t.days, 1)).toFixed(2)} g/day`,
      color: fill && !own.has(t.slug) ? 'var(--w-avb)' : 'var(--w-stash)',
    }));
  });

  /** The fills as they stood at the replay's moment. */
  protected readonly cycles = computed<Cycle[]>(() => {
    const at = this.moment();
    const all = this.wits.cycles.value();
    if (!at) {
      return all;
    }
    const t = at.getTime();
    return all
      .filter((c) => Date.parse(c.start) <= t)
      .map((c) => (c.end && Date.parse(c.end) <= t ? c : { ...c, open: true, end: undefined, remaining: 0 }));
  });

  constructor() {
    const destroy = inject(DestroyRef);
    // ?at=2025-06-01 opens the record as it stood that day — a moment to
    // bookmark. It waits for the entries to load, then lets go.
    const query = inject(ActivatedRoute).snapshot.queryParamMap;
    const scope = SCOPES.find((x) => x.key === query.get('scope'));
    if (scope) {
      this.scope.set(scope);
    }
    const at = query.get('at');
    if (at) {
      const once = effect(() => {
        if (this.replay.size() > 0) {
          untracked(() => {
            this.replay.scrubTo(at);
          });
          once.destroy();
        }
      });
    }
    destroy.onDestroy(() => {
      this.replay.destroy();
    });
    inject(Keys).register(
      'Analysis',
      () => [
        ...replayKeys(this.replay),
        { keys: ['s'], help: 'scope', run: () => {
          this.cycleScope();
        } },
      ],
      destroy,
    );
  }

  protected pick(key: string): void {
    const s = SCOPES.find((x) => x.key === key);
    if (s) {
      this.replay.goLive();
      this.scope.set(s);
    }
  }

  private cycleScope(): void {
    const i = SCOPES.findIndex((s) => s.key === this.scope().key);
    this.pick(SCOPES[(i + 1) % SCOPES.length].key);
  }
}
