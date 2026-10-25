import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { scaleLinear, scaleTime } from 'd3-scale';
import { line, symbol, symbolCircle, symbolDiamond, symbolSquare, symbolTriangle } from 'd3-shape';

import { hostWidth } from './width';

/** One dot: a purchase, placed by x (a time in ms, or a value) and y (cents a gram). */
export interface PlotPoint {
  id: string;
  x: number;
  y: number;
  /** Which series it is of — a pharmacy — for its colour and its shape; -1 for none. */
  series: number;
  /** What the tooltip says, a line each. */
  lines: string[];
  /** Drawn faint: not of what is picked. */
  faded?: boolean;
}

/** A series' colour, by its place: the theme's, never a literal. Text never wears these. */
export const SERIES_COLORS = ['var(--w-primary)', 'var(--w-accent)', 'var(--w-avb)', 'var(--w-storage)'];
/** A series' shape, by its place, so a series is told apart without its colour. */
export const SERIES_SHAPES = ['circle', 'square', 'diamond', 'triangle'] as const;
const SYMBOLS = { circle: symbolCircle, square: symbolSquare, diamond: symbolDiamond, triangle: symbolTriangle };

/** The SVG path of a series' marker, centred on 0,0, about 9 px across. */
export function marker(series: number, size = 64): string {
  const shape = series < 0 ? 'circle' : SERIES_SHAPES[series % SERIES_SHAPES.length];
  return symbol(SYMBOLS[shape], size)() ?? '';
}

/** A series' colour, or the dim one for none. */
export function seriesColor(series: number): string {
  return series < 0 ? 'var(--w-dim)' : SERIES_COLORS[series % SERIES_COLORS.length];
}

const H = 280;
const M = { top: 18, right: 18, bottom: 34, left: 52 };

/**
 * Prices a gram as dots: over time, or against a value such as strength.
 * Each dot is a purchase, its pharmacy by colour and by shape; a line, when
 * given, is a trend through them. Hover a dot for what it was; pick one to
 * say so (`picked`).
 */
@Component({
  selector: 'w-price-plot',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.viewBox]="'0 0 ' + width() + ' ' + h" [attr.height]="h" role="img" [attr.aria-label]="label()"
      (pointermove)="hover($event)" (pointerleave)="at.set(null)" (click)="pick()" (keydown.enter)="pick()" tabindex="0">
      @for (t of yTicks(); track t.v) {
        <line class="grid" [attr.x1]="m.left" [attr.x2]="width() - m.right" [attr.y1]="t.y" [attr.y2]="t.y" />
        <text class="tick" [attr.x]="m.left - 8" [attr.y]="t.y + 4" text-anchor="end">{{ t.label }}</text>
      }
      @for (t of xTicks(); track t.x) {
        <text class="tick" [attr.x]="t.x" [attr.y]="h - 10" text-anchor="middle">{{ t.label }}</text>
      }
      @if (trendPath()) {
        <path class="trend" [attr.d]="trendPath()" />
      }
      @for (p of placed(); track p.id) {
        <path class="dot" [class.faded]="p.faded" [attr.d]="p.d" [attr.transform]="'translate(' + p.px + ',' + p.py + ')'"
          [style.fill]="p.color" />
      }
      @if (at(); as a) {
        <circle class="ring" [attr.cx]="a.px" [attr.cy]="a.py" r="9" />
        <g [attr.transform]="'translate(' + a.tx + ',' + a.ty + ')'">
          <rect class="tip" [attr.width]="a.w" [attr.height]="16 + a.lines.length * 16" rx="8" />
          @for (l of a.lines; track $index) {
            <text [attr.x]="10" [attr.y]="20 + $index * 16" [class.lead]="$index === 0">{{ l }}</text>
          }
        </g>
      }
    </svg>
  `,
  styles: `
    :host { display: block; width: 100%; }
    svg { display: block; width: 100%; touch-action: pan-y; cursor: crosshair; }
    .grid { stroke: color-mix(in srgb, var(--w-fg) 7%, transparent); stroke-dasharray: 2 6; }
    .tick { fill: var(--w-dim); font: 500 11px var(--w-num); }
    .trend { fill: none; stroke: color-mix(in srgb, var(--w-fg) 38%, transparent); stroke-width: 2; stroke-linejoin: round; }
    .dot { stroke: var(--w-ink); stroke-width: 1.5; }
    .dot.faded { opacity: 0.18; }
    .ring { fill: none; stroke: var(--w-hi); stroke-width: 2; }
    .tip { fill: color-mix(in srgb, var(--w-ink) 94%, transparent); stroke: color-mix(in srgb, var(--w-fg) 12%, transparent); }
    text { fill: var(--w-dim); font: 500 12px var(--w-font); }
    text.lead { fill: var(--w-hi); font-weight: 600; }
  `,
})
export class PricePlot {
  readonly points = input.required<PlotPoint[]>();
  /** Whether x is a time (ms) or a value. */
  readonly kind = input<'time' | 'value'>('time');
  /** How a value on x is written: "%" for a strength. */
  readonly unit = input('');
  /** A trend through the dots, in the same x and y. */
  readonly trend = input<{ x: number; y: number }[]>([]);
  readonly label = input('Prices a gram');
  /** The dot clicked, by its id. */
  readonly picked = output<string>();

  protected readonly width = hostWidth();
  protected readonly h = H;
  protected readonly m = M;
  protected readonly at = signal<{ id: string; px: number; py: number; tx: number; ty: number; w: number; lines: string[] } | null>(null);

  private readonly x = computed(() => {
    const xs = this.points().map((p) => p.x);
    const lo = Math.min(...xs);
    const hi = Math.max(...xs);
    const pad = this.kind() === 'time' ? 0 : 1;
    const domain = xs.length ? [lo - pad, hi + pad || lo + 1] : [0, 1];
    return this.kind() === 'time'
      ? scaleTime().domain(domain.map((v) => new Date(v))).range([M.left, this.width() - M.right])
      : scaleLinear().domain(domain).nice().range([M.left, this.width() - M.right]);
  });

  private readonly y = computed(() => {
    const ys = this.points().map((p) => p.y);
    const hi = Math.max(...ys, 100);
    const lo = Math.min(...ys, hi);
    return scaleLinear().domain([Math.max(0, lo - (hi - lo) * 0.15 - 50), hi + (hi - lo) * 0.1 + 50]).nice(5).range([H - M.bottom, M.top]);
  });

  private px(v: number): number {
    const x = this.x();
    return this.kind() === 'time' ? (x as (d: Date) => number)(new Date(v)) : (x as (n: number) => number)(v);
  }

  protected readonly placed = computed(() => {
    const y = this.y();
    return this.points().map((p) => ({ ...p, px: this.px(p.x), py: y(p.y), d: marker(p.series), color: seriesColor(p.series) }))
      // Faded dots first, so the picked ones are drawn over them.
      .sort((a, b) => Number(b.faded ?? false) - Number(a.faded ?? false));
  });

  protected readonly trendPath = computed(() => {
    const t = this.trend();
    if (t.length < 2) {
      return '';
    }
    const y = this.y();
    return line<{ x: number; y: number }>().x((p) => this.px(p.x)).y((p) => y(p.y))(t) ?? '';
  });

  protected readonly yTicks = computed(() => this.y().ticks(5).map((v) => ({ v, y: this.y()(v), label: `€${(v / 100).toFixed(v % 100 ? 2 : 0)}` })));

  protected readonly xTicks = computed(() => {
    const count = Math.max(Math.floor(this.width() / 90), 2);
    if (this.kind() === 'time') {
      const x = this.x() as ReturnType<typeof scaleTime>;
      const [a, b] = x.domain();
      const years = (b.getTime() - a.getTime()) / 31_557_600_000 > 1.5;
      return x.ticks(count).map((d) => ({
        x: x(d),
        label: years ? String(d.getFullYear()) : d.toLocaleDateString(undefined, { month: 'short', year: '2-digit' }),
      })).filter((t, i, all) => i === 0 || t.label !== all[i - 1].label);
    }
    const x = this.x() as ReturnType<typeof scaleLinear>;
    return x.ticks(count).map((v) => ({ x: x(v), label: `${v}${this.unit()}` }));
  });

  protected hover(ev: PointerEvent): void {
    const svg = ev.currentTarget as SVGSVGElement;
    const box = svg.getBoundingClientRect();
    const mx = ((ev.clientX - box.left) / box.width) * this.width();
    const my = ((ev.clientY - box.top) / box.height) * H;
    let best: ReturnType<PricePlot['placed']>[number] | null = null;
    let dist = 24 * 24;
    for (const p of this.placed()) {
      const d = (p.px - mx) ** 2 + (p.py - my) ** 2;
      if (d < dist) {
        best = p;
        dist = d;
      }
    }
    if (!best) {
      this.at.set(null);
      return;
    }
    const w = Math.min(Math.max(...best.lines.map((l) => l.length)) * 6.6 + 20, this.width() - 8);
    const tx = best.px + 14 + w > this.width() ? Math.max(best.px - 14 - w, 4) : best.px + 14;
    const ty = Math.min(Math.max(best.py - 20, 4), H - 24 - best.lines.length * 16);
    this.at.set({ id: best.id, px: best.px, py: best.py, tx, ty, w, lines: best.lines });
  }

  protected pick(): void {
    const a = this.at();
    if (a) {
      this.picked.emit(a.id);
    }
  }
}
