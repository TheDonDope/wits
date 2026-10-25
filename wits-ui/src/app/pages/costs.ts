import { CurrencyPipe, DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { Todo } from '../data/todo';
import { Wits } from '../data/wits';

/**
 * What the prescriptions cost: paid in all, per gram, per session that
 * helped, by fill and by month. The purchases still to be priced are walked
 * under "I forgot to" (PriceWalk).
 */
@Component({
  selector: 'w-costs',
  imports: [CurrencyPipe, DatePipe, DecimalPipe, MatIconModule, RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (wits.costs.value(); as c) {
      <section class="strip">
        <div class="glass stat"><span class="eyebrow">paid, with a price</span>
          <span class="v num">{{ c.cents / 100 | currency: 'EUR' }}</span>
          <span class="dim">for {{ c.priced | number: '1.2-2' }} g</span></div>
        <div class="glass stat"><span class="eyebrow">a gram</span>
          <span class="v num">{{ c.per_gram / 100 | currency: 'EUR' }}</span>
          <span class="dim">over every gram with a price</span></div>
        <div class="glass stat"><span class="eyebrow">a session that helped</span>
          <span class="v num">{{ c.helped ? (c.per_helped / 100 | currency: 'EUR') : '—' }}</span>
          <span class="dim">{{ c.helped }} rated 4 or more overall</span></div>
        <div class="glass stat"><span class="eyebrow">without a price</span>
          <span class="v num">{{ c.unpriced }}</span>
          <span class="dim">{{ c.unpriced === 1 ? 'purchase' : 'purchases' }}, counted, not guessed at</span></div>
      </section>
    }

    @if (todo.pickups()) {
      <a class="glass waiting" routerLink="/forgot">
        <mat-icon>euro</mat-icon>
        <span>{{ todo.pickups() }} {{ todo.pickups() === 1 ? 'pickup waits' : 'pickups wait' }} for a price, under <b>I forgot to</b>.</span>
        <mat-icon>chevron_right</mat-icon>
      </a>
    }

    @if (wits.costs.value(); as c) {
      <section class="glass pad">
        <span class="eyebrow">by fill</span>
        <table>
          <thead><tr><th>fill</th><th>dispensed</th><th class="r">grams</th><th class="r">paid</th><th class="r">a gram</th><th></th></tr></thead>
          <tbody>
            @for (f of fills(); track f.number) {
              <tr>
                <td class="num">{{ f.number }}</td><td>{{ f.start | date: 'd MMM y' }}</td>
                <td class="r num">{{ f.dispensed | number: '1.2-2' }} g</td>
                <td class="r num">{{ f.priced ? (f.cents / 100 | currency: 'EUR') : '—' }}</td>
                <td class="r num">{{ f.priced ? (f.per_gram / 100 | currency: 'EUR') : '—' }}</td>
                <td class="dim">{{ f.unpriced ? f.unpriced + ' without a price' : '' }}</td>
              </tr>
            }
          </tbody>
        </table>
      </section>
      @if (c.months.length) {
        <section class="glass pad">
          <span class="eyebrow">by month</span>
          <div class="months">
            @for (m of months(); track m.month) {
              <span class="num">{{ m.month }}</span>
              <div class="bar"><i [style.width.%]="(m.cents / peak()) * 100"></i></div>
              <span class="r num">{{ m.cents / 100 | currency: 'EUR' }}</span>
            }
          </div>
        </section>
      }
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .strip { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 18px; }
    .stat { padding: 18px 22px; display: grid; gap: 4px; }
    .v { font-size: 28px; font-weight: 600; line-height: 1.3; }
    .pad { padding: 22px 26px; display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; }
    .dim { color: var(--w-dim); }
    .waiting { display: flex; align-items: center; gap: 12px; padding: 14px 20px; color: inherit; text-decoration: none; }
    .waiting span { flex: 1; }
    .waiting mat-icon:first-child { color: var(--w-primary); }
    table { width: 100%; border-collapse: collapse; font-size: 14px; }
    th { text-align: left; font-weight: 500; color: var(--w-dim); padding: 6px 8px; }
    td { padding: 6px 8px; border-top: 1px solid var(--w-glass-edge); }
    @include bp.phone { th, td { padding: 6px 4px; } .pad { padding: 18px 16px; } }
    .r { text-align: right; }
    .months { display: grid; grid-template-columns: auto 1fr auto; gap: 8px 14px; align-items: center; }
    .bar { height: 8px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); overflow: hidden; }
    .bar i { display: block; height: 100%; background: linear-gradient(90deg, var(--w-primary), var(--w-accent)); border-radius: 99px; }
  `,
})
export class Costs {
  protected readonly wits = inject(Wits);
  protected readonly todo = inject(Todo);
  protected readonly fills = computed(() => [...(this.wits.costs.value()?.fills ?? [])].reverse());
  protected readonly months = computed(() => [...(this.wits.costs.value()?.months ?? [])].reverse().slice(0, 12));
  protected readonly peak = computed(() => Math.max(1, ...this.months().map((m) => m.cents)));
}
