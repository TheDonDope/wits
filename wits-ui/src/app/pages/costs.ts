import { CurrencyPipe, DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';

import { Entry } from '../data/models';
import { Wits } from '../data/wits';

/**
 * What the prescriptions cost, and the purchases still to be priced, walked
 * one at a time — reconcile purchases, as the command line and the terminal
 * have it. A price goes beside the journal: the purchase is never edited.
 */
@Component({
  selector: 'w-costs',
  imports: [CurrencyPipe, DatePipe, DecimalPipe, FormsModule, MatButtonModule, MatFormFieldModule, MatIconModule,
    MatInputModule, MatSlideToggleModule],
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
          <p class="dim">Write down what each was paid. A blank one is skipped, and nothing is ever edited.</p>
        </div>
        <mat-slide-toggle [checked]="all()" (change)="setAll($event.checked)">every purchase, not only those in storage</mat-slide-toggle>
      </header>
      @if (current(); as p) {
        <div class="walk">
          <span class="count num">{{ index() + 1 }} of {{ queue().length }}</span>
          <div class="what">
            <strong>{{ p.product_name }}</strong>
            <span class="dim">{{ p.grams | number: '1.2-2' }} g, dispensed {{ p.occurred_at | date: 'd MMMM y' }}</span>
          </div>
          <mat-form-field appearance="outline" class="euros">
            <mat-label>Paid</mat-label>
            <input matInput [ngModel]="euros()" (ngModelChange)="euros.set($event)" placeholder="120,50" (keydown.enter)="save(p)" />
            <span matTextSuffix>€</span>
          </mat-form-field>
          <button mat-flat-button [disabled]="busy() || !euros().trim()" (click)="save(p)">Save</button>
          <button mat-button (click)="skip()">Skip</button>
        </div>
        @if (error()) {
          <p class="bad">{{ error() }}</p>
        }
      } @else {
        <p class="dim">{{ queue().length ? 'All of them walked.' : 'Every purchase here has its price.' }}</p>
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
    :host { display: grid; gap: 20px; }
    .strip { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 18px; }
    .stat { padding: 18px 22px; display: grid; gap: 4px; }
    .v { font-size: 28px; font-weight: 600; line-height: 1.3; }
    .pad { padding: 22px 26px; display: grid; gap: 14px; }
    header { display: flex; justify-content: space-between; gap: 16px; flex-wrap: wrap; align-items: flex-start; }
    header p { margin: 6px 0 0; font-size: 13px; }
    .dim { color: var(--w-dim); }
    .bad { color: var(--w-warn); margin: 0; }
    .walk { display: flex; flex-wrap: wrap; gap: 14px; align-items: center; }
    .what { display: grid; flex: 1 1 220px; }
    .count { color: var(--w-dim); min-width: 64px; }
    .euros { width: 150px; }
    table { width: 100%; border-collapse: collapse; font-size: 14px; }
    th { text-align: left; font-weight: 500; color: var(--w-dim); padding: 6px 8px; }
    td { padding: 6px 8px; border-top: 1px solid var(--w-glass-edge); }
    .r { text-align: right; }
    .months { display: grid; grid-template-columns: auto 1fr auto; gap: 8px 14px; align-items: center; }
    .bar { height: 8px; border-radius: 99px; background: rgb(255 255 255 / 6%); overflow: hidden; }
    .bar i { display: block; height: 100%; background: linear-gradient(90deg, var(--w-violet), var(--w-mint)); border-radius: 99px; }
  `,
})
export class Costs {
  protected readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  protected readonly all = signal(false);
  protected readonly queue = signal<Entry[]>([]);
  protected readonly index = signal(0);
  protected readonly euros = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal('');

  protected readonly current = computed(() => this.queue().at(this.index()));
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
    this.queue.set(await this.wits.unpriced(this.all()));
    this.index.set(0);
  }

  protected async save(p: Entry): Promise<void> {
    this.busy.set(true);
    this.error.set('');
    try {
      const out = await this.wits.price(p.hash, this.euros());
      this.snack.open(out.message, undefined, { duration: 3000 });
      this.skip();
    } catch (e) {
      this.error.set(e instanceof Error ? e.message : String(e));
    } finally {
      this.busy.set(false);
    }
  }

  protected skip(): void {
    this.euros.set('');
    this.index.update((i) => i + 1);
  }
}
