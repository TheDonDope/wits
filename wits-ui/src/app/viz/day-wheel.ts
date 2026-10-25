import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { arc } from 'd3-shape';

import { Dimension, EffectGroup } from '../data/models';
import { scoreColor } from './effect-matrix';

const PARTS = ['night', 'morning', 'afternoon', 'evening'];
const INNER = 34;
const OUTER = 128;

/**
 * The day as a wheel, night at the top: each quarter reaches out as far as the
 * sessions in it scored on one dimension — whether evenings help sleep more
 * than afternoons do, at a glance.
 */
@Component({
  selector: 'w-day-wheel',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="-160 -160 320 320" role="img" [attr.aria-label]="'Mean ' + dimension() + ' by time of day'">
      @for (r of rings; track r) {
        <circle class="ring" [attr.r]="radius(r)" />
      }
      @for (w of wedges(); track w.part) {
        <path class="track" [attr.d]="w.track" />
        @if (w.d) {
          <path class="wedge" [attr.d]="w.d" [style.fill]="w.color" [style.filter]="'drop-shadow(0 0 10px ' + w.color + ')'">
            <title>{{ w.part }}: {{ w.mean?.toFixed(2) }} over {{ w.n }}</title>
          </path>
        }
        <text class="part" [attr.x]="w.lx" [attr.y]="w.ly" text-anchor="middle">{{ w.part }}</text>
        @if (w.mean) {
          <text class="mean num" [attr.x]="w.mx" [attr.y]="w.my" text-anchor="middle">{{ w.mean.toFixed(1) }}</text>
        }
      }
      <circle r="28" class="hub" />
      <text class="hub-t" y="4" text-anchor="middle">{{ dimension() }}</text>
    </svg>
  `,
  styles: `
    :host { display: block; max-width: 320px; margin: auto; }
    svg { width: 100%; overflow: visible; }
    .ring { fill: none; stroke: color-mix(in srgb, var(--w-fg) 6%, transparent); stroke-dasharray: 2 5; }
    .track { fill: color-mix(in srgb, var(--w-fg) 3%, transparent); }
    .wedge { opacity: .9; transition: d .6s; }
    .part { fill: var(--w-dim); font: 600 10px var(--w-font); letter-spacing: .08em; text-transform: uppercase; }
    .mean { fill: var(--w-hi); font-size: 13px; font-weight: 600; }
    .hub { fill: color-mix(in srgb, var(--w-ink) 90%, transparent); stroke: color-mix(in srgb, var(--w-fg) 10%, transparent); }
    .hub-t { fill: var(--w-primary); font: 600 10px var(--w-font); text-transform: uppercase; letter-spacing: .06em; }
  `,
})
export class DayWheel {
  readonly groups = input.required<EffectGroup[]>();
  readonly dimension = input.required<Dimension>();
  protected readonly rings = [2, 3, 4, 5];

  protected radius(score: number): number {
    return INNER + ((score - 1) / 4) * (OUTER - INNER);
  }

  protected readonly wedges = computed(() => {
    const d = this.dimension();
    const by = new Map(this.groups().map((g) => [g.label, g]));
    return PARTS.map((part, i) => {
      const start = (i * Math.PI) / 2 - Math.PI / 4;
      const end = start + Math.PI / 2;
      const mid = (start + end) / 2;
      const g = by.get(part);
      const mean = g?.means[d];
      const pad = 0.04;
      const outer = mean ? this.radius(mean) : 0;
      return {
        part,
        mean,
        n: g?.counts[d] ?? 0,
        color: mean ? scoreColor(mean) : '',
        track: arc()({ innerRadius: INNER, outerRadius: OUTER, startAngle: start + pad, endAngle: end - pad }) ?? '',
        d: mean
          ? (arc().cornerRadius(6)({ innerRadius: INNER, outerRadius: outer, startAngle: start + pad, endAngle: end - pad }) ?? '')
          : '',
        lx: Math.sin(mid) * (OUTER + 18),
        ly: -Math.cos(mid) * (OUTER + 18) + 4,
        mx: Math.sin(mid) * ((INNER + outer) / 2),
        my: -Math.cos(mid) * ((INNER + outer) / 2) + 5,
      };
    });
  });
}
