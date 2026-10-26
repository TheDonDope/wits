import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';

import { Wits } from '../data/wits';
import { pictureOf } from '../data/invoice';
import { InvoiceBatch, Read } from './invoice-batch';
import { Pickup, PickupCard, Scan, byFill } from './pickup';

/**
 * The purchases still to be priced, walked a fill at a time — reconcile
 * purchases, grouped as they were picked up: a price a gram for each strain,
 * VAT on top when it is due, or its invoice scanned (PickupCard) — or many
 * invoices at once (InvoiceBatch). Newest fill first, every purchase without
 * a price unless asked for only those in storage. A price goes beside the
 * journal: the purchase is never edited.
 */
@Component({
  selector: 'w-price-walk',
  imports: [InvoiceBatch, MatSlideToggleModule, PickupCard],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <header>
      <p class="dim">A prescription at a time, as it was picked up: what each strain cost a gram, or scan its invoice.
        A strain left blank keeps no price, and nothing is ever edited.</p>
      <mat-slide-toggle [checked]="all()" (change)="setAll($event.checked)">every purchase, not only those in storage</mat-slide-toggle>
    </header>
    <w-invoice-batch [(reads)]="batch" (priced)="load()" (review)="reviewOn($event)" />
    @for (f of shown(); track f.fill) {
      <w-pickup [pickup]="f" [position]="index() + 1 + ' of ' + pickups().length" [fills]="numbers()"
        [handed]="handed()?.fill === f.fill ? handed()!.scan : null" (saved)="next()" (skipped)="next()" (jump)="jump($event)" />
    } @empty {
      <p class="dim">{{ pickups().length ? 'All of them walked.' : 'Every purchase here has its price.' }}</p>
    }
  `,
  styles: `
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; }
    header { display: flex; justify-content: space-between; gap: 16px; flex-wrap: wrap; align-items: flex-start; }
    header p { margin: 0; font-size: 13px; flex: 1 1 320px; }
    .dim { color: var(--w-dim); }
  `,
})
export class PriceWalk {
  private readonly wits = inject(Wits);
  /** Every purchase without a price, or only those still in storage. Every one unless asked. */
  protected readonly all = signal(true);
  protected readonly pickups = signal<Pickup[]>([]);
  protected readonly index = signal(0);
  /** Invoices added many at once, kept here so the list outlives the card being priced. */
  protected readonly batch = signal<Read[]>([]);
  /** An invoice read on one fill's card that is another's, on its way there. */
  protected readonly handed = signal<{ fill: number; scan: Scan } | null>(null);

  /** The fill being priced, as a list of one: a new card for each fill, so none keeps another's figures. */
  protected readonly shown = computed(() => this.pickups().slice(this.index(), this.index() + 1));
  protected readonly numbers = computed(() => this.pickups().map((p) => p.fill));

  constructor() {
    void this.load();
  }

  protected setAll(all: boolean): void {
    this.all.set(all);
    void this.load();
  }

  protected async load(): Promise<void> {
    this.pickups.set(byFill(await this.wits.unpriced(this.all())));
    this.index.set(0);
  }

  protected next(): void {
    this.handed.set(null);
    this.index.update((i) => i + 1);
  }

  /** Opens an invoice of the batch on its pickup's card, its first page drawn beside the figures. */
  protected async reviewOn(r: Read): Promise<void> {
    const fill = r.reading?.match?.fill;
    if (!r.reading || !fill) {
      return;
    }
    if (!this.pickups().some((p) => p.fill === fill)) {
      this.all.set(true);
      await this.load();
    }
    const scan: Scan = { file: r.file.name, picture: await pictureOf(r.file), pdf: URL.createObjectURL(r.file), reading: r.reading };
    this.jump({ fill, scan });
  }

  protected jump(to: { fill: number; scan: Scan }): void {
    const i = this.pickups().findIndex((p) => p.fill === to.fill);
    if (i >= 0) {
      this.handed.set(to);
      this.index.set(i);
    }
  }
}
