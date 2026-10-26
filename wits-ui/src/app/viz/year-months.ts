import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { hostWidth } from './width';

const HEIGHT = 190;
const LABELS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

/**
 * A year's grams ground, a bar a month, the busiest lit and named. In a year
 * still running, the months to come stand as outlines.
 */
@Component({
  selector: 'w-year-months',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.viewBox]="'0 0 ' + width() + ' ' + height" [attr.height]="height" role="img" [attr.aria-label]="aria()">
      <defs>
        <linearGradient id="ym-fill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stop-color="var(--w-stash)" /><stop offset="1" stop-color="color-mix(in srgb, var(--w-stash) 25%, transparent)" />
        </linearGradient>
      </defs>
      @for (b of bars(); track b.i) {
        <g [class.top]="b.top" [class.later]="b.later" [style.--i]="b.i">
          <title>{{ b.title }}</title>
          @if (b.later) {
            <rect [attr.x]="b.x" [attr.y]="base - 12" [attr.width]="b.w" height="12" rx="4" class="ghost" />
          } @else {
            <rect [attr.x]="b.x" [attr.y]="b.y" [attr.width]="b.w" [attr.height]="b.h" rx="6" class="bar" />
          }
          @if (b.top) {
            <text [attr.x]="b.x + b.w / 2" [attr.y]="b.y - 8" text-anchor="middle" class="v">{{ b.value }}</text>
          }
          <text [attr.x]="b.x + b.w / 2" [attr.y]="height - 4" text-anchor="middle" class="m">{{ b.label }}</text>
        </g>
      }
    </svg>
  `,
  styles: `
    :host { display: block; width: 100%; }
    svg { display: block; width: 100%; overflow: visible; }
    .bar { fill: url(#ym-fill); transform-origin: bottom; transform-box: fill-box; animation: rise .9s cubic-bezier(.2,.8,.2,1) both;
      animation-delay: calc(var(--i) * 55ms); }
    .top .bar { fill: var(--w-stash); filter: drop-shadow(0 0 10px color-mix(in srgb, var(--w-stash) 70%, transparent)); }
    .ghost { fill: none; stroke: var(--w-glass-edge); stroke-dasharray: 3 3; }
    .m { fill: var(--w-dim); font: 11px var(--w-font); }
    .top .m { fill: var(--w-fg); font-weight: 600; }
    .v { fill: var(--w-fg); font: 600 13px var(--w-num); }
    @keyframes rise { from { transform: scaleY(0); } }
  `,
})
export class YearMonths {
  readonly months = input.required<number[]>();
  /** The months there have been so far: twelve unless the year is running. */
  readonly upTo = input(12);
  protected readonly width = hostWidth();
  protected readonly height = HEIGHT;
  protected readonly base = HEIGHT - 22;

  protected readonly bars = computed(() => {
    const ms = this.months();
    const top = Math.max(...ms, 0.01);
    const busiest = ms.indexOf(Math.max(...ms));
    const slot = this.width() / 12;
    const w = Math.max(slot * 0.62, 6);
    return ms.map((g, i) => {
      const h = Math.max((g / top) * (this.base - 26), g > 0 ? 4 : 0);
      return { i, x: i * slot + (slot - w) / 2, w, y: this.base - h, h, top: i === busiest && g > 0, later: i >= this.upTo(),
        label: LABELS[i], value: `${g.toFixed(1)} g`, title: `${LABELS[i]}: ${g.toFixed(2)} g ground` };
    });
  });

  protected readonly aria = computed(() =>
    `Grams ground by month: ${this.months().map((g, i) => `${LABELS[i]} ${g.toFixed(1)}`).join(', ')}`);
}
