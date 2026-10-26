import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { quantile } from 'd3-array';

import { Day } from '../data/models';
import { hostWidth } from './width';

interface Cell {
  key: string;
  x: number;
  y: number;
  grams: number;
  level: number;
  col: number;
  label: string;
}

interface Row {
  label: string;
  total: number;
  y: number;
  cells: Cell[];
  months: { x: number; label: string }[];
}

const GAP = 3;
const LEFT = 44;
const RIGHT = 70;

/**
 * Days as a calendar of squares, a column a week: a year a row when the whole
 * history is asked for, or only the last weeks for the front page. The ramp is
 * cut at the history's own quantiles, so a heavy day reads as heavy for this
 * person rather than against a scale made up for someone else.
 */
@Component({
  selector: 'w-heatmap',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.viewBox]="'0 0 ' + width() + ' ' + height()" [attr.height]="height()" role="img"
      [attr.aria-label]="'Calendar of grams ' + (metric() === 'ground' ? 'ground' : 'through a device')">
      <defs>
        <filter [attr.id]="glowId"><feGaussianBlur stdDeviation="2.2" /></filter>
      </defs>
      @for (row of rows(); track row.label) {
        <text class="year" [attr.x]="0" [attr.y]="row.y + cell() * 3.6">{{ row.label }}</text>
        @for (m of row.months; track m.x) {
          <text class="month" [attr.x]="m.x" [attr.y]="row.y - 6">{{ m.label }}</text>
        }
        @for (c of row.cells; track c.key) {
          @if (c.level === 4) {
            <rect class="glow" [attr.x]="c.x" [attr.y]="c.y" [attr.width]="cell()" [attr.height]="cell()" rx="3"
              [attr.filter]="'url(#' + glowId + ')'" [style.fill]="tone()" />
          }
          <rect class="cell" [attr.x]="c.x" [attr.y]="c.y" [attr.width]="cell()" [attr.height]="cell()" rx="3"
            [style.fill]="c.level ? tone() : null" [style.fill-opacity]="opacity[c.level]"
            [style.animation-delay.ms]="c.col * 9" (pointerenter)="hovered.set(c)" (pointerleave)="hovered.set(null)" />
        }
        @if (!weeks()) {
          <text class="total" [attr.x]="width()" [attr.y]="row.y + cell() * 3.6" text-anchor="end">
            {{ row.total.toFixed(0) }} g
          </text>
        }
      }
      @if (hovered(); as h) {
        <g [attr.transform]="'translate(' + tipX(h) + ',' + (h.y - 40) + ')'" class="tipg">
          <rect width="150" height="32" rx="8" class="tip" />
          <text x="10" y="20" class="tip-t">{{ h.label }} · {{ h.grams.toFixed(2) }} g</text>
        </g>
      }
    </svg>
    <div class="legend">
      <span class="dim">less</span>
      @for (o of opacity; track $index) {
        <i [style.background]="$index ? tone() : 'color-mix(in srgb, var(--w-fg) 5%, transparent)'" [style.opacity]="$index ? o : 1"></i>
      }
      <span class="dim">more</span>
    </div>
  `,
  styles: `
    :host { display: block; width: 100%; }
    svg { display: block; width: 100%; overflow: visible; }
    .cell { fill: color-mix(in srgb, var(--w-fg) 5%, transparent); opacity: 0; animation: in .5s ease-out forwards; }
    .cell:hover { stroke: var(--w-hi); stroke-width: 1.2; }
    .glow { opacity: .8; }
    .year, .total { fill: var(--w-dim); font: 600 12px var(--w-num); }
    .month { fill: var(--w-dim); font: 500 10px var(--w-font); }
    .tip { fill: color-mix(in srgb, var(--w-ink) 94%, transparent); stroke: color-mix(in srgb, var(--w-fg) 12%, transparent); }
    .tip-t { fill: var(--w-hi); font: 600 12px var(--w-num); }
    .tipg { pointer-events: none; }
    .legend { display: flex; gap: 4px; align-items: center; justify-content: flex-end; margin-top: 8px; font-size: 11px; }
    .legend i { width: 11px; height: 11px; border-radius: 3px; display: inline-block; }
    @keyframes in { from { opacity: 0; transform: scale(.4); } to { opacity: 1; transform: none; } }
    .cell { transform-box: fill-box; transform-origin: center; }
  `,
})
export class Heatmap {
  readonly days = input.required<Day[]>();
  readonly metric = input<'ground' | 'seshed'>('ground');
  /** Only the last weeks, as one row; all years when not given. */
  readonly weeks = input<number | null>(null);
  readonly tone = input('var(--w-accent)');
  /** One year's row alone, the year on one card's; every year when not given. */
  readonly year = input<number | null>(null);

  protected readonly width = hostWidth();
  protected readonly glowId = `heat-glow-${Math.random().toString(36).slice(2, 8)}`;
  protected readonly opacity = [1, 0.22, 0.45, 0.7, 1];
  protected readonly hovered = signal<Cell | null>(null);

  private readonly byDate = computed(() => {
    const m = new Map<string, number>();
    for (const d of this.days()) {
      m.set(d.date, this.metric() === 'ground' ? d.ground : d.seshed);
    }
    return m;
  });

  /** The ramp's cuts: the quartiles of the days that had anything at all. */
  private readonly cuts = computed(() => {
    const vals = [...this.byDate().values()].filter((v) => v > 0).sort((a, b) => a - b);
    return [0.25, 0.5, 0.75].map((q) => quantile(vals, q) ?? 0);
  });

  /** A year on a narrow screen is two half years, so all of it fits. */
  private readonly halves = computed(() => this.weeks() === null && this.width() < 560);

  protected readonly cell = computed(() => {
    const cols = this.weeks() ?? (this.halves() ? 27 : 53);
    const usable = this.width() - (this.weeks() ? 0 : LEFT + RIGHT);
    return Math.max(Math.min(Math.floor(usable / cols) - GAP, 18), 6);
  });

  protected readonly rows = computed<Row[]>(() => {
    const weeks = this.weeks();
    const today = new Date();
    if (weeks) {
      const end = endOfWeek(today);
      const start = new Date(end);
      start.setDate(end.getDate() - weeks * 7 + 1);
      return [this.row('', start, end, 16, 0)];
    }
    const dates = [...this.byDate().keys()].sort();
    if (!dates.length) {
      return [];
    }
    const only = this.year();
    const step = (this.cell() + GAP) * 7 + 34;
    const first = only ?? Number(dates[0].slice(0, 4));
    const rows: Row[] = [];
    if (this.halves()) {
      // Each year as two rows, the year named on its first.
      for (let year = only ?? today.getFullYear(), i = 0; year >= first; year--, i += 2) {
        rows.push(this.row(only === null ? String(year) : '', new Date(year, 0, 1), new Date(year, 5, 30), 18 + i * step, LEFT),
          this.row('', new Date(year, 6, 1), new Date(year, 11, 31), 18 + (i + 1) * step, LEFT));
      }
      return rows;
    }
    for (let year = only ?? today.getFullYear(), i = 0; year >= first; year--, i++) {
      rows.push(this.row(String(year), new Date(year, 0, 1), new Date(year, 11, 31), 18 + i * step, LEFT));
    }
    return rows;
  });

  protected readonly height = computed(() => {
    const rows = this.rows();
    const last = rows.at(-1);
    return last ? last.y + (this.cell() + GAP) * 7 + 8 : 40;
  });

  private row(label: string, from: Date, to: Date, top: number, left: number): Row {
    const size = this.cell() + GAP;
    const start = startOfWeek(from);
    const cells: Cell[] = [];
    const months: { x: number; label: string }[] = [];
    let total = 0;
    const today = new Date();
    for (let d = new Date(start); d <= to; d.setDate(d.getDate() + 1)) {
      if (d < from || d > today) {
        continue;
      }
      const col = Math.floor((d.getTime() - start.getTime()) / (7 * 86400000));
      const dow = (d.getDay() + 6) % 7;
      const key = iso(d);
      const g = this.byDate().get(key) ?? 0;
      total += g;
      if (d.getDate() === 1 || (cells.length === 0 && label === '')) {
        months.push({ x: left + col * size, label: d.toLocaleDateString(undefined, { month: 'short' }) });
      }
      cells.push({
        key,
        x: left + col * size,
        y: top + dow * size,
        grams: g,
        level: this.level(g),
        col,
        label: d.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' }),
      });
    }
    return { label, total, y: top, cells, months };
  }

  private level(g: number): number {
    if (g <= 0) {
      return 0;
    }
    const [a, b, c] = this.cuts();
    return g <= a ? 1 : g <= b ? 2 : g <= c ? 3 : 4;
  }

  protected tipX(c: Cell): number {
    return Math.min(Math.max(c.x - 70, 0), this.width() - 150);
  }
}

function iso(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

function startOfWeek(d: Date): Date {
  const s = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  s.setDate(s.getDate() - ((s.getDay() + 6) % 7));
  return s;
}

function endOfWeek(d: Date): Date {
  const s = startOfWeek(d);
  s.setDate(s.getDate() + 6);
  return s;
}
