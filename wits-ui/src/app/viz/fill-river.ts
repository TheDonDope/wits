import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { scaleLinear, scaleTime } from 'd3-scale';

import { Cycle } from '../data/models';
import { hostWidth } from './width';

const LANE = 12;
const GAP = 4;
const M = { top: 26, right: 16, bottom: 8, left: 44 };

interface Bar {
  c: Cycle;
  x: number;
  w: number;
  y: number;
  left: number;
  tone: number;
  days: number;
}

/**
 * Every fill as a lane on one timeline, from the day it was picked up to the
 * day its own jars ran empty. Fills overlap — a jar outliving its month is
 * normal — and here the overlap is seen rather than inferred. An open fill
 * runs on to today, its own remaining grams lit at its end.
 */
@Component({
  selector: 'w-fill-river',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.viewBox]="'0 0 ' + width() + ' ' + height()" [attr.height]="height()" role="img"
      aria-label="Every fill on one timeline">
      <defs>
        <linearGradient id="river-open" x1="0" x2="1">
          <stop offset="0" stop-color="var(--w-violet)" stop-opacity=".25" />
          <stop offset="1" stop-color="var(--w-mint)" stop-opacity=".9" />
        </linearGradient>
        <pattern id="river-sheen" width="24" height="12" patternUnits="userSpaceOnUse" patternTransform="skewX(-30)">
          <rect width="10" height="12" fill="rgba(255,255,255,0.12)" />
        </pattern>
      </defs>
      @for (t of ticks(); track t.label) {
        <line class="grid" [attr.x1]="t.x" [attr.x2]="t.x" [attr.y1]="m.top - 8" [attr.y2]="height() - m.bottom" />
        <text class="tick" [attr.x]="t.x + 4" [attr.y]="m.top - 12">{{ t.label }}</text>
      }
      @for (b of bars(); track b.c.number) {
        <g class="lane" [style.animation-delay.ms]="b.c.number * 22" (pointerenter)="hovered.set(b)"
          (pointerleave)="hovered.set(null)">
          <text class="num-l" [attr.x]="m.left - 8" [attr.y]="b.y + 10" text-anchor="end">{{ b.c.number }}</text>
          <rect [attr.x]="b.x" [attr.y]="b.y" [attr.width]="b.w" [attr.height]="lane" rx="6"
            [attr.fill]="b.c.open ? 'url(#river-open)' : 'var(--w-storage)'"
            [attr.fill-opacity]="b.c.open ? 1 : b.tone" />
          @if (b.c.open) {
            <rect class="sheen" [attr.x]="b.x" [attr.y]="b.y" [attr.width]="b.w" [attr.height]="lane" rx="6"
              fill="url(#river-sheen)" />
            <rect [attr.x]="b.x + b.w - b.left" [attr.y]="b.y" [attr.width]="b.left" [attr.height]="lane" rx="6"
              class="left" />
          }
        </g>
      }
      @if (hovered(); as h) {
        <g [attr.transform]="'translate(' + tipX(h) + ',' + (h.y + 18) + ')'" class="tipg">
          <rect width="230" height="58" rx="10" class="tip" />
          <text x="12" y="20" class="tip-h">Fill {{ h.c.number }} · {{ h.c.dispensed.toFixed(0) }} g dispensed</text>
          <text x="12" y="36" class="tip-t">{{ span(h) }}</text>
          <text x="12" y="51" class="tip-t">
            {{ h.c.open ? h.c.remaining.toFixed(2) + ' g of it still sealed' : h.c.ground_during.toFixed(0) + ' g ground on the shelf meanwhile' }}
          </text>
        </g>
      }
    </svg>
  `,
  styles: `
    :host { display: block; width: 100%; }
    svg { display: block; width: 100%; overflow: visible; }
    .grid { stroke: rgba(255,255,255,0.05); }
    .tick { fill: var(--w-dim); font: 600 11px var(--w-num); }
    .num-l { fill: var(--w-dim); font: 500 10px var(--w-num); }
    .lane { opacity: 0; animation: slide .7s cubic-bezier(.2,.8,.2,1) forwards; cursor: default; }
    .lane:hover rect:first-of-type { filter: brightness(1.4); }
    .sheen { animation: sheen 2.4s linear infinite; mix-blend-mode: screen; }
    .left { fill: #fff; opacity: .85; filter: drop-shadow(0 0 6px var(--w-mint)); }
    .tip { fill: rgba(16,20,28,0.95); stroke: rgba(255,255,255,0.12); }
    .tip-h { fill: #fff; font: 600 12px 'Inter Variable'; }
    .tip-t { fill: var(--w-dim); font: 500 11px 'Inter Variable'; }
    .tipg { pointer-events: none; }
    @keyframes slide { from { opacity: 0; transform: translateX(-18px); } to { opacity: 1; transform: none; } }
    @keyframes sheen { to { transform: translateX(24px); } }
  `,
})
export class FillRiver {
  readonly cycles = input.required<Cycle[]>();

  protected readonly width = hostWidth();
  protected readonly lane = LANE;
  protected readonly m = M;
  protected readonly hovered = signal<Bar | null>(null);

  private readonly oldestFirst = computed(() => [...this.cycles()].sort((a, b) => a.number - b.number));

  private readonly x = computed(() => {
    const cs = this.oldestFirst();
    const first = cs.length ? new Date(cs[0].start) : new Date();
    return scaleTime().domain([first, new Date()]).range([M.left, this.width() - M.right]);
  });

  protected readonly height = computed(() => M.top + this.oldestFirst().length * (LANE + GAP) + M.bottom);

  protected readonly bars = computed<Bar[]>(() => {
    const x = this.x();
    const most = Math.max(...this.oldestFirst().map((c) => c.dispensed), 1);
    const tone = scaleLinear().domain([0, most]).range([0.35, 0.95]);
    return this.oldestFirst().map((c, i) => {
      const start = new Date(c.start);
      const end = c.open || !c.end ? new Date() : new Date(c.end);
      const x0 = x(start);
      const w = Math.max(x(end) - x0, 6);
      return {
        c,
        x: x0,
        w,
        y: M.top + i * (LANE + GAP),
        left: c.open && c.dispensed > 0 ? Math.max((c.remaining / c.dispensed) * w, 3) : 0,
        tone: tone(c.dispensed),
        days: Math.round((end.getTime() - start.getTime()) / 86400000),
      };
    });
  });

  protected readonly ticks = computed(() => {
    const x = this.x();
    return x.ticks(Math.max(Math.floor(this.width() / 120), 2)).map((d) => ({
      x: x(d),
      label: d.toLocaleDateString(undefined, { month: 'short', year: '2-digit' }),
    }));
  });

  protected span(b: Bar): string {
    const fmt = (s: string | Date) => new Date(s).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
    return b.c.open ? `since ${fmt(b.c.start)} · ${b.days} days, open` : `${fmt(b.c.start)} → ${fmt(b.c.end!)} · ${b.days} days`;
  }

  protected tipX(b: Bar): number {
    return Math.min(Math.max(b.x + b.w / 2 - 115, 0), this.width() - 230);
  }
}
