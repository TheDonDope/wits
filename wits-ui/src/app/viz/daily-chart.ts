import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { scaleBand, scaleLinear } from 'd3-scale';
import { curveMonotoneX, line } from 'd3-shape';

import { eachDay, movingAverage } from '../data/derive';
import { Day } from '../data/models';
import { hostWidth } from './width';

const H = 240;
const M = { top: 16, right: 12, bottom: 30, left: 40 };

/**
 * Grams ground per day across a range that does not move while a replay
 * fills it: the daily amounts as bars, the seven-day average as a line over
 * them — the spikes are the record, the line is the habit.
 */
@Component({
  selector: 'w-daily',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.viewBox]="'0 0 ' + width() + ' ' + h" [attr.height]="h" role="img" aria-label="Grams ground per day"
      (pointermove)="hover($event)" (pointerleave)="at.set(null)">
      <defs>
        <linearGradient id="daily-bar" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stop-color="var(--w-stash)" />
          <stop offset="1" stop-color="var(--w-stash)" stop-opacity=".25" />
        </linearGradient>
        <filter id="daily-glow"><feGaussianBlur stdDeviation="3" /></filter>
      </defs>
      @for (t of yTicks(); track t.v) {
        <line class="grid" [attr.x1]="m.left" [attr.x2]="width() - m.right" [attr.y1]="t.y" [attr.y2]="t.y" />
        <text class="tick" [attr.x]="m.left - 6" [attr.y]="t.y + 4" text-anchor="end">{{ t.v }}</text>
      }
      @for (b of bars(); track b.date) {
        <rect class="bar" [attr.x]="b.x" [attr.y]="b.y" [attr.width]="b.w" [attr.height]="b.h" rx="2" fill="url(#daily-bar)"
          [class.hot]="at()?.date === b.date" />
      }
      <path class="avg-glow" [attr.d]="avg()" filter="url(#daily-glow)" />
      <path class="avg" [attr.d]="avg()" />
      @for (t of xTicks(); track t.label) {
        <text class="tick" [attr.x]="t.x" [attr.y]="h - 8" text-anchor="middle">{{ t.label }}</text>
      }
      @if (at(); as p) {
        <g [attr.transform]="'translate(' + p.tx + ',' + m.top + ')'" class="tipg">
          <rect width="150" height="44" rx="10" class="tip" />
          <text x="12" y="18" class="tip-d">{{ p.label }}</text>
          <text x="12" y="35" class="tip-g">{{ p.grams.toFixed(2) }} g · avg {{ p.avg.toFixed(2) }}</text>
        </g>
      }
    </svg>
    <div class="legend">
      <span><i class="sw bar-sw"></i>ground that day</span>
      <span><i class="sw line-sw"></i>7-day average</span>
    </div>
  `,
  styles: `
    :host { display: block; width: 100%; }
    svg { display: block; width: 100%; }
    .grid { stroke: rgb(255 255 255 / 6%); stroke-dasharray: 2 6; }
    .tick { fill: var(--w-dim); font: 500 10px var(--w-num); }
    .bar { transition: y .35s ease-out, height .35s ease-out; }
    .bar.hot { fill: #fff; }
    .avg { fill: none; stroke: var(--w-violet); stroke-width: 2.4; stroke-linecap: round; }
    .avg-glow { fill: none; stroke: var(--w-violet); stroke-width: 6; opacity: .4; }
    .tip { fill: rgb(16 20 28 / 95%); stroke: rgb(255 255 255 / 12%); }
    .tip-d { fill: var(--w-dim); font: 600 11px 'Inter Variable', sans-serif; }
    .tip-g { fill: #fff; font: 600 12px var(--w-num); }
    .tipg { pointer-events: none; }
    .legend { display: flex; gap: 18px; font-size: 12px; color: var(--w-dim); margin-top: 6px; }
    .sw { display: inline-block; width: 14px; height: 4px; border-radius: 2px; margin-right: 6px; vertical-align: middle; }
    .bar-sw { background: var(--w-stash); }
    .line-sw { background: var(--w-violet); }
  `,
})
export class DailyChart {
  readonly days = input.required<Day[]>();
  readonly from = input.required<string>();
  readonly to = input.required<string>();

  protected readonly width = hostWidth();
  protected readonly h = H;
  protected readonly m = M;
  protected readonly at = signal<{ date: string; tx: number; grams: number; avg: number; label: string } | null>(null);

  private readonly range = computed(() => (this.from() <= this.to() ? eachDay(this.from(), this.to()) : []));

  private readonly values = computed(() => {
    const by = new Map(this.days().map((d) => [d.date, d.ground]));
    return this.range().map((d) => by.get(d) ?? 0);
  });

  private readonly averages = computed(() => movingAverage(this.values(), 7));

  private readonly x = computed(() =>
    scaleBand().domain(this.range()).range([M.left, this.width() - M.right]).paddingInner(0.18),
  );

  private readonly y = computed(() =>
    scaleLinear()
      .domain([0, Math.max(...this.values(), 1)])
      .nice(4)
      .range([H - M.bottom, M.top]),
  );

  protected readonly bars = computed(() => {
    const x = this.x();
    const y = this.y();
    const w = Math.max(x.bandwidth(), 1);
    return this.range().map((date, i) => {
      const v = this.values()[i];
      return { date, x: x(date) ?? 0, w, y: y(v), h: Math.max(y(0) - y(v), v > 0 ? 1.5 : 0) };
    });
  });

  protected readonly avg = computed(() => {
    const x = this.x();
    const y = this.y();
    const half = x.bandwidth() / 2;
    const pts = this.range().map((d, i) => [(x(d) ?? 0) + half, y(this.averages()[i])] as [number, number]);
    return line().curve(curveMonotoneX)(pts) ?? '';
  });

  protected readonly yTicks = computed(() => this.y().ticks(4).map((v) => ({ v, y: this.y()(v) })));

  protected readonly xTicks = computed(() => {
    const range = this.range();
    const count = Math.max(Math.floor(this.width() / 110), 2);
    const every = Math.max(Math.ceil(range.length / count), 1);
    const x = this.x();
    return range
      .filter((_, i) => i % every === 0)
      .map((d) => ({
        x: (x(d) ?? 0) + x.bandwidth() / 2,
        label: new Date(`${d}T12:00`).toLocaleDateString(undefined, { day: 'numeric', month: 'short' }),
      }));
  });

  protected hover(ev: PointerEvent): void {
    const svg = ev.currentTarget as SVGSVGElement;
    const box = svg.getBoundingClientRect();
    const px = ((ev.clientX - box.left) / box.width) * this.width();
    const x = this.x();
    const range = this.range();
    if (!range.length) {
      return;
    }
    const step = x.step();
    const i = Math.min(Math.max(Math.floor((px - M.left) / step), 0), range.length - 1);
    const date = range[i];
    const bx = x(date) ?? 0;
    this.at.set({
      date,
      tx: Math.min(bx + 10, this.width() - M.right - 150),
      grams: this.values()[i],
      avg: this.averages()[i],
      label: new Date(`${date}T12:00`).toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' }),
    });
  }
}
