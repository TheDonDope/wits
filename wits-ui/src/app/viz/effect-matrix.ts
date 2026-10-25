import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { Dimension, EffectGroup } from '../data/models';

/**
 * A mean score's colour: the theme's warning colour for little help, its AVB
 * amber in the middle, its good colour for a lot — mixed by the browser, so it
 * follows the theme.
 */
export function scoreColor(mean: number): string {
  const t = Math.min(Math.max((mean - 1) / 4, 0), 1);
  const [from, to, local] = t < 0.5 ? ['--w-warn', '--w-avb', t * 2] : ['--w-avb', '--w-good', (t - 0.5) * 2];
  return `color-mix(in oklab, var(${to}) ${String(Math.round(local * 100))}%, var(${from}))`;
}

/**
 * Products against what they were rated on: each cell the mean, coloured
 * from rose to mint, with how many ratings it stands on. A mean of one rating
 * is hatched: it is a rating, not a finding.
 */
@Component({
  selector: 'w-effect-matrix',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="grid" [style.grid-template-columns]="'minmax(140px, 260px) repeat(' + dimensions().length + ', minmax(52px, 1fr))'">
      <span></span>
      @for (d of dimensions(); track d) {
        <span class="col">{{ d }}</span>
      }
      @for (g of rows(); track g.key) {
        <span class="row" [title]="g.label">{{ g.label }}<small> · {{ g.count }}</small></span>
        @for (d of dimensions(); track d) {
          @if (g.means[d]; as m) {
            <span class="cell" [class.thin]="(g.counts[d] ?? 0) < 2" [style.--c]="color(m)"
              [title]="g.label + ' · ' + d + ': ' + m.toFixed(2) + ' over ' + g.counts[d] + (g.counts[d] === 1 ? ' rating' : ' ratings')">
              <b class="num">{{ m.toFixed(1) }}</b><i>{{ g.counts[d] }}</i>
            </span>
          } @else {
            <span class="cell none"></span>
          }
        }
      }
    </div>
  `,
  styles: `
    .grid { display: grid; gap: 6px; align-items: center; }
    .col { font: 600 10px var(--w-font); letter-spacing: .1em; text-transform: uppercase;
      color: var(--w-dim); text-align: center; }
    .row { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-weight: 500; }
    .row small { color: var(--w-dim); font-weight: 400; }
    .cell { position: relative; height: 40px; border-radius: 10px; display: grid; place-items: center;
      background: color-mix(in srgb, var(--c) 26%, transparent); border: 1px solid color-mix(in srgb, var(--c) 55%, transparent);
      box-shadow: 0 0 18px -6px var(--c); transition: transform .15s; }
    .cell:hover { transform: scale(1.06); }
    .cell b { color: var(--c); font-size: 15px; }
    .cell i { position: absolute; right: 5px; bottom: 2px; font: 500 9px var(--w-num); color: var(--w-dim); font-style: normal; }
    .cell.thin { background-image: repeating-linear-gradient(45deg, transparent 0 5px, color-mix(in srgb, var(--w-fg) 5%, transparent) 5px 10px);
      box-shadow: none; opacity: .75; }
    .cell.none { background: color-mix(in srgb, var(--w-fg) 3%, transparent); border: 1px dashed color-mix(in srgb, var(--w-fg) 6%, transparent); }
  `,
})
export class EffectMatrix {
  readonly groups = input.required<EffectGroup[]>();
  readonly dimensions = input.required<Dimension[]>();
  protected readonly rows = computed(() => [...this.groups()].sort((a, b) => b.count - a.count || a.label.localeCompare(b.label)));
  protected readonly color = scoreColor;
}
