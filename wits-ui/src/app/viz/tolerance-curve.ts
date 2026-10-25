import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { scaleLinear, scalePoint } from 'd3-scale';
import { area, curveMonotoneX, line } from 'd3-shape';

import { BREAK_DAYS, Break, TOLERANCE_WINDOW, TolerancePoint } from '../data/derive';
import { hostWidth } from './width';

const H = 240;
const M = { top: 16, right: 40, bottom: 30, left: 40 };

/**
 * Tolerance: grams a day against how the sessions went, both rolling over the
 * same window, on a range that does not move while a replay draws it. The
 * grams are an area on the left scale, the mean overall rating a line on the
 * right, 1 to 5, broken where no rated session falls in the window. Breaks —
 * a week and more without use — are bands behind both, so the curve can be seen to
 * bend through them.
 */
@Component({
  selector: 'w-tolerance',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.viewBox]="'0 0 ' + width() + ' ' + h" [attr.height]="h" role="img"
      aria-label="Grams a day against the sessions' ratings" (pointermove)="hover($event)" (pointerleave)="at.set(null)">
      <defs>
        <linearGradient id="tol-area" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stop-color="var(--w-stash)" stop-opacity=".55" />
          <stop offset="1" stop-color="var(--w-stash)" stop-opacity=".04" />
        </linearGradient>
      </defs>
      @for (b of bands(); track b.from) {
        <rect class="break" [attr.x]="b.x" [attr.y]="m.top" [attr.width]="b.w" [attr.height]="h - m.top - m.bottom" />
      }
      @for (t of gTicks(); track t.v) {
        <line class="grid" [attr.x1]="m.left" [attr.x2]="width() - m.right" [attr.y1]="t.y" [attr.y2]="t.y" />
        <text class="tick" [attr.x]="m.left - 6" [attr.y]="t.y + 4" text-anchor="end">{{ t.v }}</text>
      }
      @if (rated()) {
        @for (t of rTicks(); track t.v) {
          <text class="tick rate" [attr.x]="width() - m.right + 6" [attr.y]="t.y + 4">{{ t.v }}</text>
        }
      }
      <path class="area" [attr.d]="gArea()" fill="url(#tol-area)" />
      <path class="g" [attr.d]="gLine()" />
      <path class="r" [attr.d]="rLine()" />
      @for (t of xTicks(); track t.label) {
        <text class="tick" [attr.x]="t.x" [attr.y]="h - 8" text-anchor="middle">{{ t.label }}</text>
      }
      @if (at(); as p) {
        <line class="cursor" [attr.x1]="p.x" [attr.x2]="p.x" [attr.y1]="m.top" [attr.y2]="h - m.bottom" />
        <g [attr.transform]="'translate(' + p.tx + ',' + m.top + ')'" class="tipg">
          <rect width="190" height="44" rx="10" class="tip" />
          <text x="12" y="18" class="tip-d">{{ p.label }}</text>
          <text x="12" y="35" class="tip-g">{{ p.grams.toFixed(2) }} g a day · {{ p.rating }}</text>
        </g>
      }
    </svg>
    <div class="legend">
      <span><i class="sw g-sw"></i>grams a day, {{ window }} days rolling</span>
      <span><i class="sw r-sw"></i>overall rating, 1–5, same window{{ rated() ? '' : ' — no session rated yet' }}</span>
      <span><i class="sw b-sw"></i>a break: {{ breakDays }} or more days with nothing ground and no session</span>
    </div>
  `,
  styles: `
    :host { display: block; width: 100%; }
    svg { display: block; width: 100%; }
    .grid { stroke: color-mix(in srgb, var(--w-fg) 6%, transparent); stroke-dasharray: 2 6; }
    .tick { fill: var(--w-dim); font: 500 10px var(--w-num); }
    .tick.rate { fill: var(--w-sesh); }
    .break { fill: color-mix(in srgb, var(--w-avb) 9%, transparent); }
    .g { fill: none; stroke: var(--w-stash); stroke-width: 2.2; }
    .r { fill: none; stroke: var(--w-sesh); stroke-width: 2.4; stroke-linecap: round; }
    .cursor { stroke: color-mix(in srgb, var(--w-fg) 25%, transparent); }
    .tip { fill: color-mix(in srgb, var(--w-ink) 95%, transparent); stroke: color-mix(in srgb, var(--w-fg) 12%, transparent); }
    .tip-d { fill: var(--w-dim); font: 600 11px var(--w-font); }
    .tip-g { fill: var(--w-hi); font: 600 12px var(--w-num); }
    .tipg { pointer-events: none; }
    .legend { display: flex; flex-wrap: wrap; gap: 8px 18px; font-size: 12px; color: var(--w-dim); margin-top: 6px; }
    .sw { display: inline-block; width: 14px; height: 4px; border-radius: 2px; margin-right: 6px; vertical-align: middle; }
    .g-sw { background: var(--w-stash); }
    .r-sw { background: var(--w-sesh); }
    .b-sw { background: color-mix(in srgb, var(--w-avb) 45%, transparent); height: 10px; }
  `,
})
export class ToleranceCurve {
  /** Every day of the range; a replay leaves the days it has not reached at nothing. */
  readonly points = input.required<TolerancePoint[]>();
  readonly breaks = input.required<Break[]>();
  /** The last day drawn: the replay's, or the range's end when live. */
  readonly until = input<string | null>(null);

  protected readonly width = hostWidth();
  protected readonly h = H;
  protected readonly m = M;
  protected readonly window = TOLERANCE_WINDOW;
  protected readonly breakDays = BREAK_DAYS;
  protected readonly at = signal<{ x: number; tx: number; grams: number; rating: string; label: string } | null>(null);

  private readonly x = computed(() =>
    scalePoint().domain(this.points().map((p) => p.date)).range([M.left, this.width() - M.right]),
  );

  private readonly g = computed(() =>
    scaleLinear()
      .domain([0, Math.max(...this.points().map((p) => p.grams), 0.5)])
      .nice(4)
      .range([H - M.bottom, M.top]),
  );

  private readonly r = scaleLinear().domain([1, 5]).range([H - M.bottom, M.top]);

  protected readonly rated = computed(() => this.points().some((p) => p.rating !== null));

  /** Whether a day has been reached: a replay draws nothing past its moment. */
  private readonly reached = computed(() => {
    const until = this.until();
    return (p: TolerancePoint) => until === null || p.date <= until;
  });

  protected readonly gArea = computed(() => {
    const x = this.x();
    const g = this.g();
    return area<TolerancePoint>().curve(curveMonotoneX).defined(this.reached())
      .x((p) => x(p.date) ?? 0).y0(g(0)).y1((p) => g(p.grams))(this.points()) ?? '';
  });

  protected readonly gLine = computed(() => {
    const x = this.x();
    const g = this.g();
    return line<TolerancePoint>().curve(curveMonotoneX).defined(this.reached())
      .x((p) => x(p.date) ?? 0).y((p) => g(p.grams))(this.points()) ?? '';
  });

  protected readonly rLine = computed(() => {
    const x = this.x();
    const reached = this.reached();
    return line<TolerancePoint>().curve(curveMonotoneX).defined((p) => p.rating !== null && reached(p))
      .x((p) => x(p.date) ?? 0).y((p) => this.r(p.rating ?? 1))(this.points()) ?? '';
  });

  protected readonly bands = computed(() => {
    const x = this.x();
    const step = x.step();
    return this.breaks().map((b) => {
      const from = (x(b.from) ?? M.left) - step / 2;
      const to = (x(b.to) ?? this.width() - M.right) + step / 2;
      return { from: b.from, x: Math.max(from, M.left), w: Math.max(Math.min(to, this.width() - M.right) - Math.max(from, M.left), 1) };
    });
  });

  protected readonly gTicks = computed(() => this.g().ticks(4).map((v) => ({ v, y: this.g()(v) })));
  protected readonly rTicks = computed(() => [1, 2, 3, 4, 5].map((v) => ({ v, y: this.r(v) })));

  protected readonly xTicks = computed(() => {
    const pts = this.points();
    const count = Math.max(Math.floor(this.width() / 110), 2);
    const every = Math.max(Math.ceil(pts.length / count), 1);
    const x = this.x();
    return pts
      .filter((_, i) => i % every === 0)
      .map((p) => ({
        x: x(p.date) ?? 0,
        label: new Date(`${p.date}T12:00`).toLocaleDateString(undefined, { day: 'numeric', month: 'short' }),
      }));
  });

  protected hover(ev: PointerEvent): void {
    const pts = this.points();
    if (!pts.length) {
      return;
    }
    const svg = ev.currentTarget as SVGSVGElement;
    const box = svg.getBoundingClientRect();
    const px = ((ev.clientX - box.left) / box.width) * this.width();
    const step = this.x().step() || 1;
    const i = Math.min(Math.max(Math.round((px - M.left) / step), 0), pts.length - 1);
    const p = pts[i];
    const x = this.x()(p.date) ?? 0;
    this.at.set({
      x,
      tx: Math.min(x + 10, this.width() - M.right - 190),
      grams: p.grams,
      rating: p.rating === null ? 'no rating' : `rated ${p.rating.toFixed(1)} (${p.ratings})`,
      label: new Date(`${p.date}T12:00`).toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' }),
    });
  }
}
