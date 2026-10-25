import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { scaleLinear, scaleTime } from 'd3-scale';
import { area, curveMonotoneX, line } from 'd3-shape';

import { Point } from '../data/models';
import { hostWidth } from './width';

const H = 260;
const M = { top: 24, right: 24, bottom: 34, left: 44 };

interface Plotted {
  date: Date;
  grams: number;
  ahead: boolean;
}

/**
 * The shelf over time: what stood when the fill arrived, drawn down day by day
 * by every grind since, and on at the observed rate to the day it runs out.
 * The past is a filled curve, the future a dashed line fading toward empty.
 */
@Component({
  selector: 'w-projection',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.viewBox]="'0 0 ' + width() + ' ' + h" [attr.height]="h" (pointermove)="hover($event)"
      (pointerleave)="at.set(null)" role="img" aria-label="Supply projection">
      <defs>
        <linearGradient id="proj-past" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stop-color="var(--w-storage)" stop-opacity="0.55" />
          <stop offset="1" stop-color="var(--w-storage)" stop-opacity="0" />
        </linearGradient>
        <linearGradient id="proj-ahead" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0" stop-color="var(--w-avb)" stop-opacity="0.22" />
          <stop offset="1" stop-color="var(--w-warn)" stop-opacity="0.02" />
        </linearGradient>
        <linearGradient id="proj-ahead-line" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0" stop-color="var(--w-avb)" />
          <stop offset="1" stop-color="var(--w-warn)" />
        </linearGradient>
        <filter id="proj-glow"><feGaussianBlur stdDeviation="4" /></filter>
      </defs>

      @for (t of yTicks(); track t.v) {
        <line class="grid" [attr.x1]="m.left" [attr.x2]="width() - m.right" [attr.y1]="t.y" [attr.y2]="t.y" />
        <text class="tick" [attr.x]="m.left - 8" [attr.y]="t.y + 4" text-anchor="end">{{ t.v }} g</text>
      }
      @for (t of xTicks(); track t.label) {
        <text class="tick" [attr.x]="t.x" [attr.y]="h - 10" text-anchor="middle">{{ t.label }}</text>
      }

      <path class="past-area" [attr.d]="paths().pastArea" />
      <path class="past-glow" [attr.d]="paths().pastLine" filter="url(#proj-glow)" />
      <path class="past-line" [attr.d]="paths().pastLine" />
      <path class="ahead-area" [attr.d]="paths().aheadArea" />
      <path class="ahead-line" [attr.d]="paths().aheadLine" />

      @if (today(); as t) {
        <line class="today" [attr.x1]="t.x" [attr.x2]="t.x" [attr.y1]="m.top" [attr.y2]="h - m.bottom" />
        <circle class="pulse" [attr.cx]="t.x" [attr.cy]="t.y" r="10" />
        <circle class="dot" [attr.cx]="t.x" [attr.cy]="t.y" r="4.5" />
        <text class="label" [attr.x]="t.x" [attr.y]="m.top - 8" text-anchor="middle">today</text>
      }
      @if (empty(); as e) {
        <circle class="end" [attr.cx]="e.x" [attr.cy]="e.y" r="4" />
        <text class="label warn" [attr.x]="e.x - 4" [attr.y]="e.y - 26" text-anchor="end">empty ~ {{ e.label }}</text>
      }

      @if (at(); as p) {
        <line class="cross" [attr.x1]="p.x" [attr.x2]="p.x" [attr.y1]="m.top" [attr.y2]="h - m.bottom" />
        <circle class="probe" [attr.cx]="p.x" [attr.cy]="p.y" r="5" [class.ahead]="p.ahead" />
        <g [attr.transform]="'translate(' + p.tx + ',' + (m.top + 4) + ')'">
          <rect width="128" height="44" rx="10" class="tip" />
          <text x="12" y="18" class="tip-date">{{ p.label }}{{ p.ahead ? ' · projected' : '' }}</text>
          <text x="12" y="35" class="tip-g">{{ p.grams.toFixed(2) }} g on the shelf</text>
        </g>
      }
    </svg>
  `,
  styles: `
    :host { display: block; width: 100%; }
    svg { display: block; width: 100%; touch-action: none; }
    .grid { stroke: color-mix(in srgb, var(--w-fg) 6%, transparent); stroke-dasharray: 2 6; }
    .tick { fill: var(--w-dim); font: 500 11px var(--w-num); }
    .past-area { fill: url(#proj-past); }
    .past-line { fill: none; stroke: var(--w-storage); stroke-width: 2.4; stroke-linecap: round;
      stroke-dasharray: 3000; stroke-dashoffset: 3000; animation: draw 1.8s cubic-bezier(.2,.8,.2,1) forwards; }
    .past-glow { fill: none; stroke: var(--w-storage); stroke-width: 6; opacity: 0.35; }
    .ahead-area { fill: url(#proj-ahead); }
    .ahead-line { fill: none; stroke: url(#proj-ahead-line); stroke-width: 2; stroke-dasharray: 6 6;
      animation: march 1.2s linear infinite; }
    .today { stroke: color-mix(in srgb, var(--w-fg) 18%, transparent); stroke-dasharray: 2 4; }
    .dot { fill: var(--w-hi); }
    .pulse { fill: var(--w-storage); opacity: 0.3; transform-box: fill-box; transform-origin: center;
      animation: pulse 2.2s ease-out infinite; }
    .end { fill: var(--w-warn); }
    .label { fill: var(--w-dim); font: 600 11px var(--w-font); letter-spacing: .06em; }
    .label.warn { fill: var(--w-warn); }
    .cross { stroke: color-mix(in srgb, var(--w-fg) 25%, transparent); }
    .probe { fill: var(--w-storage); stroke: var(--w-hi); stroke-width: 2; }
    .probe.ahead { fill: var(--w-avb); }
    .tip { fill: color-mix(in srgb, var(--w-ink) 92%, transparent); stroke: color-mix(in srgb, var(--w-fg) 10%, transparent); }
    .tip-date { fill: var(--w-dim); font: 600 11px var(--w-font); }
    .tip-g { fill: var(--w-hi); font: 600 13px var(--w-num); }
    @keyframes draw { to { stroke-dashoffset: 0; } }
    @keyframes march { to { stroke-dashoffset: -12; } }
    @keyframes pulse { from { transform: scale(.5); opacity: .5; } to { transform: scale(2.2); opacity: 0; } }
  `,
})
export class ProjectionChart {
  readonly history = input.required<Point[]>();
  readonly projected = input.required<Point[]>();

  protected readonly width = hostWidth();
  protected readonly h = H;
  protected readonly m = M;
  protected readonly at = signal<{ x: number; y: number; tx: number; grams: number; label: string; ahead: boolean } | null>(
    null,
  );

  private readonly points = computed<Plotted[]>(() => [
    ...this.history().map((p) => ({ date: day(p.date), grams: p.grams, ahead: false })),
    ...this.projected().map((p) => ({ date: day(p.date), grams: p.grams, ahead: true })),
  ]);

  private readonly x = computed(() => {
    const pts = this.points();
    const first = pts[0]?.date ?? new Date();
    const last = pts.at(-1)?.date ?? new Date();
    return scaleTime().domain([first, last]).range([M.left, this.width() - M.right]);
  });

  private readonly y = computed(() => {
    const top = Math.max(...this.points().map((p) => p.grams), 1);
    return scaleLinear().domain([0, top]).nice(4).range([H - M.bottom, M.top]);
  });

  protected readonly paths = computed(() => {
    const x = this.x();
    const y = this.y();
    const past = this.points().filter((p) => !p.ahead);
    const lastPast = past.at(-1);
    const ahead = [...(lastPast ? [lastPast] : []), ...this.points().filter((p) => p.ahead)];
    const ln = line<Plotted>().x((p) => x(p.date)).y((p) => y(p.grams)).curve(curveMonotoneX);
    const ar = area<Plotted>().x((p) => x(p.date)).y0(y(0)).y1((p) => y(p.grams)).curve(curveMonotoneX);
    return {
      pastLine: ln(past) ?? '',
      pastArea: ar(past) ?? '',
      aheadLine: ahead.length > 1 ? (ln(ahead) ?? '') : '',
      aheadArea: ahead.length > 1 ? (ar(ahead) ?? '') : '',
    };
  });

  protected readonly yTicks = computed(() => {
    const y = this.y();
    return y.ticks(4).map((v) => ({ v, y: y(v) }));
  });

  protected readonly xTicks = computed(() => {
    const x = this.x();
    const count = Math.max(Math.floor(this.width() / 110), 2);
    return x.ticks(count).map((d) => ({ x: x(d), label: d.toLocaleDateString(undefined, { day: 'numeric', month: 'short' }) }));
  });

  protected readonly today = computed(() => {
    const last = this.points().filter((p) => !p.ahead).at(-1);
    return last ? { x: this.x()(last.date), y: this.y()(last.grams) } : null;
  });

  protected readonly empty = computed(() => {
    const last = this.projected().at(-1);
    if (!last) {
      return null;
    }
    const d = day(last.date);
    return {
      x: this.x()(d),
      y: this.y()(last.grams),
      label: d.toLocaleDateString(undefined, { day: 'numeric', month: 'short' }),
    };
  });

  protected hover(ev: PointerEvent): void {
    const svg = ev.currentTarget as SVGSVGElement;
    const box = svg.getBoundingClientRect();
    const px = ((ev.clientX - box.left) / box.width) * this.width();
    const pts = this.points();
    if (!pts.length) {
      return;
    }
    const x = this.x();
    let best = pts[0];
    for (const p of pts) {
      if (Math.abs(x(p.date) - px) < Math.abs(x(best.date) - px)) {
        best = p;
      }
    }
    const bx = x(best.date);
    this.at.set({
      x: bx,
      y: this.y()(best.grams),
      tx: Math.min(bx + 12, this.width() - M.right - 128),
      grams: best.grams,
      ahead: best.ahead,
      label: best.date.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' }),
    });
  }
}

/** A calendar date, at local noon so no time zone moves it a day. */
function day(s: string): Date {
  const [y, m, d] = s.split('-').map(Number);
  return new Date(y, m - 1, d, 12);
}
