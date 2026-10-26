import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { scaleBand, scaleLinear } from 'd3-scale';

import { grams, hostWidth } from './width';

/** One month of a jar: what was ground of it, and what of it went through a device. */
export interface MonthUse {
  month: string;
  ground: number;
  seshed: number;
}

const HEIGHT = 150;
const PAD = { top: 10, right: 4, bottom: 22, left: 4 };

/**
 * A jar's months side by side: what was ground of it each month, and beside
 * it what went through a device. A month with neither is left as a gap, so
 * a jar put away and come back to shows the time between.
 */
@Component({
  selector: 'w-month-bars',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.width]="width()" [attr.height]="height" role="img" [attr.aria-label]="label()">
      @for (b of bars(); track b.month) {
        <g>
          <title>{{ b.title }}</title>
          <rect [attr.x]="b.x" [attr.y]="b.groundY" [attr.width]="b.w" [attr.height]="b.groundH" rx="2" class="ground" />
          <rect [attr.x]="b.x + b.w" [attr.y]="b.seshY" [attr.width]="b.w" [attr.height]="b.seshH" rx="2" class="sesh" />
          @if (b.tick) {
            <text [attr.x]="b.x + b.w" [attr.y]="height - 6" text-anchor="middle">{{ b.tick }}</text>
          }
        </g>
      }
    </svg>
  `,
  styles: `
    :host { display: block; width: 100%; }
    svg { display: block; overflow: visible; }
    .ground { fill: var(--w-stash); }
    .sesh { fill: var(--w-sesh); }
    text { fill: var(--w-dim); font: 10px var(--w-num); }
  `,
})
export class MonthBars {
  readonly months = input.required<MonthUse[]>();
  protected readonly width = hostWidth();
  protected readonly height = HEIGHT;

  /** Every month from the first to the last, the empty ones included. */
  private readonly span = computed(() => {
    const given = this.months();
    if (!given.length) {
      return [];
    }
    const by = new Map(given.map((m) => [m.month, m]));
    const out: MonthUse[] = [];
    let [y, m] = given[0].month.split('-').map(Number);
    const last = given[given.length - 1].month;
    for (;;) {
      const key = `${String(y)}-${String(m).padStart(2, '0')}`;
      out.push(by.get(key) ?? { month: key, ground: 0, seshed: 0 });
      if (key >= last) {
        return out;
      }
      [y, m] = m === 12 ? [y + 1, 1] : [y, m + 1];
    }
  });

  protected readonly label = computed(() =>
    `Grams ground and vaped each month, ${String(this.span().length)} months`);

  protected readonly bars = computed(() => {
    const months = this.span();
    const x = scaleBand().domain(months.map((m) => m.month))
      .range([PAD.left, this.width() - PAD.right]).paddingInner(0.25);
    const top = Math.max(...months.map((m) => Math.max(m.ground, m.seshed)), 0.01);
    const y = scaleLinear().domain([0, top]).range([HEIGHT - PAD.bottom, PAD.top]);
    const every = Math.ceil(months.length / Math.max(Math.floor(this.width() / 56), 1));
    // A bar no wider than a thumb: a jar of two months is two pairs, not two walls.
    const w = Math.min(x.bandwidth() / 2, 12);
    const inset = x.bandwidth() / 2 - w;
    return months.map((m, i) => {
      return {
        month: m.month,
        x: (x(m.month) ?? 0) + inset,
        w,
        groundY: y(m.ground), groundH: y(0) - y(m.ground),
        seshY: y(m.seshed), seshH: y(0) - y(m.seshed),
        tick: i % every === 0 ? short(m.month) : '',
        title: `${m.month}: ${grams(m.ground)} ground, ${grams(m.seshed)} seshed`,
      };
    });
  });
}

/** A month as a tick says it: "Mar 25". */
function short(month: string): string {
  const [y, m] = month.split('-').map(Number);
  return new Date(y, m - 1, 1).toLocaleString(undefined, { month: 'short' }) + ` ${String(y).slice(2)}`;
}
