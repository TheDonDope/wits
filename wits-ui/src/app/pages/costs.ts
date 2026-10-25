import { CurrencyPipe, DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';

import { Wits } from '../data/wits';
import { Pickup, PickupCard, Scan, byFill } from './pickup';

/**
 * What the prescriptions cost, and the purchases still to be priced, walked
 * a fill at a time — reconcile purchases, grouped as they were picked up, a
 * price a gram for each strain and VAT on top when it is due, or its invoice
 * scanned (PickupCard). A price goes beside the journal: the purchase is never
 * edited.
 */
@Component({
  selector: 'w-costs',
  imports: [CurrencyPipe, DatePipe, DecimalPipe, MatSlideToggleModule, PickupCard],
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

    <section class="glass pad">
      <header>
        <div>
          <span class="eyebrow">reconcile purchases</span>
          <p class="dim">A fill at a time, as the prescription was picked up: what each strain cost a gram, or scan its invoice. A strain left blank keeps no price, and nothing is ever edited.</p>
        </div>
        <mat-slide-toggle [checked]="all()" (change)="setAll($event.checked)">every purchase, not only those in storage</mat-slide-toggle>
      </header>
      @for (f of shown(); track f.fill) {
        <w-pickup [pickup]="f" [position]="index() + 1 + ' of ' + pickups().length" [fills]="numbers()"
          [handed]="handed()?.fill === f.fill ? handed()!.scan : null" (saved)="next()" (skipped)="next()" (jump)="jump($event)" />
      } @empty {
        <p class="dim">{{ pickups().length ? 'All of them walked.' : 'Every purchase here has its price.' }}</p>
      }
    </section>

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
    header { display: flex; justify-content: space-between; gap: 16px; flex-wrap: wrap; align-items: flex-start; }
    header p { margin: 6px 0 0; font-size: 13px; }
    .dim { color: var(--w-dim); }
    .bad { color: var(--w-warn); margin: 0; }
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
  protected readonly all = signal(false);
  protected readonly pickups = signal<Pickup[]>([]);
  protected readonly index = signal(0);
  /** An invoice read on one fill's card that is another's, on its way there. */
  protected readonly handed = signal<{ fill: number; scan: Scan } | null>(null);

  /** The fill being priced, as a list of one: a new card for each fill, so none keeps another's figures. */
  protected readonly shown = computed(() => this.pickups().slice(this.index(), this.index() + 1));
  protected readonly numbers = computed(() => this.pickups().map((p) => p.fill));
  protected readonly fills = computed(() => [...(this.wits.costs.value()?.fills ?? [])].reverse());
  protected readonly months = computed(() => [...(this.wits.costs.value()?.months ?? [])].reverse().slice(0, 12));
  protected readonly peak = computed(() => Math.max(1, ...this.months().map((m) => m.cents)));

  constructor() {
    void this.load();
  }

  protected setAll(all: boolean): void {
    this.all.set(all);
    void this.load();
  }

  private async load(): Promise<void> {
    this.pickups.set(byFill(await this.wits.unpriced(this.all())));
    this.index.set(0);
  }

  protected next(): void {
    this.handed.set(null);
    this.index.update((i) => i + 1);
  }

  protected jump(to: { fill: number; scan: Scan }): void {
    const i = this.pickups().findIndex((p) => p.fill === to.fill);
    if (i >= 0) {
      this.handed.set(to);
      this.index.set(i);
    }
  }
}
