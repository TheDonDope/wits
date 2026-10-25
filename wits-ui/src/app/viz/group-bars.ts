import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { Dimension, EffectGroup } from '../data/models';
import { scoreColor } from './effect-matrix';

/**
 * One dimension's mean across groups, as columns on the 1–5 scale: the
 * temperature bands, say, warmest last. A column is as solid as the ratings
 * behind it — one rating barely shows.
 */
@Component({
  selector: 'w-group-bars',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="cols">
      @for (b of bars(); track b.key) {
        <div class="col" [title]="b.label + ': ' + b.mean.toFixed(2) + ' over ' + b.n + (b.n === 1 ? ' rating' : ' ratings')">
          <span class="v num">{{ b.mean.toFixed(1) }}</span>
          <div class="track">
            <div class="fill" [style.height.%]="((b.mean - 1) / 4) * 100" [style.background]="b.color"
              [style.opacity]="b.weight" [style.box-shadow]="'0 0 18px ' + b.color"></div>
          </div>
          <span class="l">{{ b.label }}</span>
          <span class="n dim">{{ b.n }}×</span>
        </div>
      } @empty {
        <p class="dim">Nothing rated on {{ dimension() }} here yet.</p>
      }
    </div>
  `,
  styles: `
    .cols { display: flex; gap: 14px; align-items: flex-end; min-height: 200px; overflow-x: auto; padding-top: 8px; }
    .col { display: grid; justify-items: center; gap: 6px; min-width: 54px; }
    .v { font-weight: 600; }
    .track { width: 30px; height: 140px; border-radius: 10px; background: color-mix(in srgb, var(--w-fg) 5%, transparent);
      display: flex; align-items: flex-end; overflow: hidden; }
    .fill { width: 100%; border-radius: 10px; transition: height .6s cubic-bezier(.2,.8,.2,1); min-height: 4px; }
    .l { font: 500 11px var(--w-num); color: var(--mat-sys-on-surface); white-space: nowrap; }
    .n { font-size: 10px; }
  `,
})
export class GroupBars {
  readonly groups = input.required<EffectGroup[]>();
  readonly dimension = input.required<Dimension>();

  protected readonly bars = computed(() => {
    const d = this.dimension();
    const rated = this.groups().filter((g) => g.means[d] !== undefined);
    const most = Math.max(...rated.map((g) => g.counts[d] ?? 0), 1);
    return rated.map((g) => {
      const mean = g.means[d] ?? 0;
      const n = g.counts[d] ?? 0;
      return { key: g.key, label: g.label, mean, n, color: scoreColor(mean), weight: 0.35 + 0.65 * (n / most) };
    });
  });
}
