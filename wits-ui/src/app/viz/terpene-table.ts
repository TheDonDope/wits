import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { familyColor, familyLabel } from '../data/aroma';
import { Terpene } from '../data/models';
import { scoreColor } from './effect-matrix';

/**
 * The terpenes the shelf names, the most named first: what each tastes of,
 * how many products name it — and lead with it — as a bar, and what those
 * products' sessions were rated, as a mark on the 1 to 5 scale. Two measures,
 * two columns, each with its own scale. A terpene's rating weighs each
 * product by its rated sessions and by where the terpene stands in it: a
 * trace counts less than the lead.
 */
@Component({
  selector: 'w-terpene-table',
  imports: [DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="head" aria-hidden="true">
      <span>terpene · tastes of</span><span>products naming it <i class="lead"></i> leading</span><span>rated overall, 1–5</span>
    </div>
    <ul>
      @for (t of rows(); track t.name) {
        <li>
          <div class="what">
            <span class="name"><i class="dot" [style.background]="t.color"></i>{{ t.name }}</span>
            <span class="dim taste">{{ t.flavors.join(', ') || 'no taste known' }}{{ t.familyLabel ? ' · ' + t.familyLabel : '' }}</span>
          </div>
          <div class="count" [attr.aria-label]="t.products + ' products, ' + t.leads + ' leading'">
            <div class="track">
              <div class="all" [style.width.%]="(t.products / most()) * 100"></div>
              <div class="led" [style.width.%]="(t.leads / most()) * 100"></div>
            </div>
            <span class="num">{{ t.products }}<span class="dim"> · {{ t.leads }}</span></span>
          </div>
          <div class="rate">
            @if (t.overall && t.rated) {
              <div class="scale" [attr.aria-label]="'rated ' + (t.overall | number: '1.1-1') + ' over ' + t.rated + ' sessions'">
                <i [style.left.%]="((t.overall - 1) / 4) * 100" [style.background]="t.rateColor"></i>
              </div>
              <span class="num">{{ t.overall | number: '1.1-1' }}<span class="dim"> · {{ t.rated }}</span></span>
            } @else {
              <span class="dim small">not rated yet</span>
            }
          </div>
          @if (t.notes) { <p class="notes dim">{{ t.notes }}</p> }
        </li>
      }
    </ul>
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: block; }
    .head, li { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr) minmax(0, 1fr); gap: 6px 18px; align-items: center; }
    .head { font: 600 9px var(--w-font); letter-spacing: .1em; text-transform: uppercase; color: var(--w-dim); padding-bottom: 6px; }
    .head .lead { display: inline-block; width: 10px; height: 6px; border-radius: 2px; background: var(--w-primary); margin: 0 2px 0 6px; }
    ul { list-style: none; margin: 0; padding: 0; }
    li { padding: 8px 0; border-top: 1px solid var(--w-glass-edge); }
    .what { display: grid; min-width: 0; }
    .name { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 14px; }
    .dot { width: 10px; height: 10px; border-radius: 50%; flex: none; }
    .taste { font-size: 12px; padding-left: 18px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .count, .rate { display: grid; grid-template-columns: minmax(0, 1fr) 54px; gap: 10px; align-items: center; font-size: 13px; }
    .track { position: relative; height: 8px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); }
    .track div { position: absolute; left: 0; top: 0; bottom: 0; border-radius: 99px; }
    .all { background: color-mix(in srgb, var(--w-primary) 35%, transparent); }
    .led { background: var(--w-primary); }
    .scale { position: relative; height: 8px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); }
    .scale i { position: absolute; top: -3px; width: 14px; height: 14px; margin-left: -7px; border-radius: 50%;
      border: 2px solid var(--w-glass); }
    .notes { grid-column: 1 / -1; margin: 0; padding-left: 18px; font-size: 12px; }
    .dim { color: var(--w-dim); }
    .small { font-size: 12px; }
    @include bp.phone {
      .head { display: none; }
      li { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
      .what { grid-column: 1 / -1; }
    }
  `,
})
export class TerpeneTable {
  readonly terpenes = input.required<Terpene[]>();

  protected readonly most = computed(() => Math.max(...this.terpenes().map((t) => t.products), 1));
  protected readonly rows = computed(() =>
    this.terpenes().map((t) => ({
      // The family is named when the flavours do not already say it ("citrus, lemon · citrus").
      ...t, color: familyColor(t.family), familyLabel: t.flavors.includes(familyLabel(t.family)) ? '' : familyLabel(t.family),
      rateColor: t.overall ? scoreColor(t.overall) : 'var(--w-dim)',
    })),
  );
}
