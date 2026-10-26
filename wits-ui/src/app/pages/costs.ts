import { DatePipe, DecimalPipe, PercentPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, ElementRef, computed, inject, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { bands, euros, fillTrend, lately, month, sellerSeries } from '../data/costs';
import { Bought } from '../data/models';
import { Todo } from '../data/todo';
import { Wits } from '../data/wits';
import { PlotPoint, PricePlot, marker, seriesColor } from '../viz/price-plot';
import { CostStrains } from './cost-strains';

/** A purchase as a dot: what the tooltip says of it. */
function about(b: Bought): string[] {
  const when = new Date(b.at).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
  return [b.name, `${euros(b.per_gram)} a gram · ${b.grams} g · ${euros(b.cents)}`, `${when} · fill ${b.fill} · ${b.seller || 'typed in'}`];
}

/**
 * What the prescriptions cost, every way it can be asked: paid in all, a
 * gram, lately, and a month at the current rate; every purchase's price a
 * gram over time, around each fill's; each strain's, opened to its every
 * purchase; each pharmacy's; strength against price; and by year, fill and
 * month. Only purchases with a price count, and how many lack one is said.
 * The purchases still to be priced are walked under "I forgot to".
 */
@Component({
  selector: 'w-costs',
  imports: [CostStrains, DatePipe, DecimalPipe, MatIconModule, PercentPipe, PricePlot, RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (wits.costs.value(); as c) {
      <section class="strip">
        <div class="glass stat"><span class="eyebrow">paid, with a price</span>
          <span class="v num">{{ money(c.cents, 0) }}</span>
          <span class="dim">for {{ c.priced | number: '1.0-2' }} g in {{ c.bought.length }} purchases</span></div>
        <div class="glass stat"><span class="eyebrow">a gram</span>
          <span class="v num">{{ money(c.per_gram) }}</span>
          <span class="dim">over every gram with a price</span></div>
        @if (recent().cents) {
          <div class="glass stat"><span class="eyebrow">a gram, lately</span>
            <span class="v num">{{ money(recent().cents) }}</span>
            <span class="dim">over {{ recent().over }}
              @if (c.per_gram) { · <span [class.up]="recent().cents > c.per_gram" [class.down]="recent().cents < c.per_gram">
                {{ recent().cents > c.per_gram ? '+' : '' }}{{ (recent().cents - c.per_gram) / c.per_gram | percent: '1.0-0' }}</span> on all }</span></div>
        }
        @if (monthly(); as m) {
          <div class="glass stat"><span class="eyebrow">a month, at this rate</span>
            <span class="v num">{{ money(m.cents, 0) }}</span>
            <span class="dim">at this prescription's {{ m.grams | number: '1.2-2' }} g a day and {{ money(recent().cents) }} a gram</span></div>
        }
        <div class="glass stat"><span class="eyebrow">without a price</span>
          <span class="v num">{{ c.unpriced }}</span>
          <span class="dim">{{ c.unpriced === 1 ? 'purchase' : 'purchases' }}, counted, not guessed at</span></div>
      </section>

      @if (todo.pickups()) {
        <a class="glass waiting" routerLink="/forgot">
          <mat-icon>euro</mat-icon>
          <span>{{ todo.pickups() }} {{ todo.pickups() === 1 ? 'pickup waits' : 'pickups wait' }} for a price, under <b>I forgot to</b>.</span>
          <mat-icon>chevron_right</mat-icon>
        </a>
      }

      @if (c.bought.length) {
        <section class="glass pad">
          <header>
            <span class="eyebrow">a gram, purchase by purchase</span>
            <span class="dim small">the line is each prescription's · pick a dot for its strain</span>
          </header>
          <w-price-plot [points]="timeline()" [trend]="trend()" label="What a gram cost, every purchase over time"
            (picked)="openStrainOf($event)" />
          <div class="legend">
            @for (s of sellers(); track s.name) {
              <span><svg width="12" height="12" viewBox="-6 -6 12 12" aria-hidden="true"><path [attr.d]="s.shape" [style.fill]="s.color" /></svg>{{ s.name }}</span>
            }
            <span><svg width="18" height="12" aria-hidden="true"><line x1="1" y1="6" x2="17" y2="6" class="trendkey" /></svg>a fill's price a gram</span>
          </div>
        </section>

        <section class="glass pad" id="strains">
          <header>
            <span class="eyebrow">by strain</span>
            <span class="dim small">{{ c.strains.length }} strains · the same cultivar under every name and producer it came by</span>
          </header>
          <w-cost-strains [strains]="c.strains" [bought]="c.bought" [series]="series()" [(picked)]="strain" />
        </section>

        <section class="pair">
          <div class="glass pad">
            <span class="eyebrow">by pharmacy</span>
            @for (s of c.sellers; track s.key) {
              <div class="seller">
                <div class="who">
                  <svg width="12" height="12" viewBox="-6 -6 12 12" aria-hidden="true"><path [attr.d]="shapeOf(s.key)" [style.fill]="colorOf(s.key)" /></svg>
                  <strong>{{ s.name || 'typed in' }}</strong>
                  <span class="dim small">{{ s.first_at | date: 'MMM y' }} – {{ s.last_at | date: 'MMM y' }}</span>
                </div>
                <span class="num r"><b>{{ money(s.per_gram) }}</b> a gram</span>
                <div class="meter"><i [style.width.%]="(s.per_gram / dearest()) * 100" [style.background]="colorOf(s.key)"></i></div>
                <span class="dim small r">{{ s.purchases }} bought · {{ s.grams | number: '1.0-0' }} g · {{ money(s.cents, 0) }}</span>
              </div>
            }
            <p class="dim small">What the invoices charged besides flower — shipping, the narcotics fee — is not in these.</p>
          </div>

          <div class="glass pad">
            <span class="eyebrow">by year</span>
            <table>
              <thead><tr><th>year</th><th class="r">bought</th><th class="r">grams</th><th class="r">paid</th><th class="r">a gram</th></tr></thead>
              <tbody>
                @for (y of c.years; track y.key; let i = $index) {
                  <tr>
                    <td class="num">{{ y.key }}</td>
                    <td class="r num">{{ y.purchases }}</td>
                    <td class="r num">{{ y.grams | number: '1.0-0' }} g</td>
                    <td class="r num">{{ money(y.cents, 0) }}</td>
                    <td class="r num">{{ money(y.per_gram) }}
                      @if (i > 0) {
                        <span class="small" [class.up]="y.per_gram > c.years[i - 1].per_gram" [class.down]="y.per_gram < c.years[i - 1].per_gram">
                          {{ y.per_gram > c.years[i - 1].per_gram ? '+' : '' }}{{ (y.per_gram - c.years[i - 1].per_gram) / c.years[i - 1].per_gram | percent: '1.0-0' }}</span>
                      }
                    </td>
                  </tr>
                }
              </tbody>
            </table>
          </div>
        </section>

        @if (potency().length > 2) {
          <section class="glass pad">
            <header>
              <span class="eyebrow">strength and price</span>
              <span class="dim small">a dot a purchase, by its THC</span>
            </header>
            <w-price-plot [points]="potency()" kind="value" unit=" %" label="What a gram cost against its THC" (picked)="openStrainOf($event)" />
            <div class="bands">
              @for (b of strength(); track b.label) {
                <div><span class="dim small">{{ b.label }}</span><b class="num">{{ money(b.cents) }}</b><span class="dim small">{{ b.purchases }} bought</span></div>
              }
            </div>
          </section>
        }
      } @else {
        <section class="glass empty">
          <mat-icon>receipt_long</mat-icon>
          <p>Price a few pickups — by hand or from their invoices, under <a routerLink="/forgot">I forgot to</a> — and this fills in:
            what a gram cost over time, by strain, by pharmacy and by strength.</p>
        </section>
      }

      <section class="glass pad">
        <span class="eyebrow">by prescription</span>
        <table>
          <thead><tr><th>fill</th><th>dispensed</th><th class="r">grams</th><th class="r">paid</th><th class="r">a gram</th><th class="opt"></th></tr></thead>
          <tbody>
            @for (f of fills(); track f.number) {
              <tr class="fill" [class.on]="openFill() === f.number" (click)="toggleFill(f.number)" tabindex="0" (keydown.enter)="toggleFill(f.number)">
                <td class="num">{{ f.number }}</td><td>{{ f.start | date: 'd MMM y' }}</td>
                <td class="r num">{{ f.dispensed | number: '1.2-2' }} g</td>
                <td class="r num">{{ f.priced ? money(f.cents) : '—' }}</td>
                <td class="r num">{{ f.priced ? money(f.per_gram) : '—' }}</td>
                <td class="dim opt">{{ f.unpriced ? f.unpriced + ' without a price' : '' }}</td>
              </tr>
              @if (openFill() === f.number) {
                <tr class="drill"><td colspan="6">
                  @for (b of fillBuys(); track b.hash) {
                    <div class="buy">
                      <span>{{ b.name }}</span>
                      <span class="dim opt">{{ b.seller || 'typed in' }}</span>
                      <span class="num r">{{ b.grams | number: '1.0-2' }} g · {{ money(b.cents) }} · <b>{{ money(b.per_gram) }}</b> a gram</span>
                    </div>
                  } @empty {
                    <p class="dim">No purchase of this fill has a price.</p>
                  }
                </td></tr>
              }
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
              <span class="r num">{{ money(m.cents) }}</span>
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
    .stat { padding: 18px 22px; display: grid; gap: 4px; align-content: start; }
    .v { font-size: 28px; font-weight: 600; line-height: 1.3; }
    .pad { padding: 22px 26px; display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; align-content: start; }
    .pad header { display: flex; justify-content: space-between; align-items: baseline; flex-wrap: wrap; gap: 6px 12px; }
    .pair { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(380px, 100%), 1fr)); gap: 20px; }
    .dim { color: var(--w-dim); }
    .small { font-size: 12px; }
    .up { color: var(--w-warn); }
    .down { color: var(--w-good); }
    .waiting { display: flex; align-items: center; gap: 12px; padding: 14px 20px; color: inherit; text-decoration: none; }
    .waiting span { flex: 1; }
    .waiting mat-icon:first-child { color: var(--w-primary); }
    .legend { display: flex; flex-wrap: wrap; gap: 6px 18px; font-size: 12px; color: var(--w-dim); }
    .legend span { display: inline-flex; align-items: center; gap: 6px; }
    .trendkey { stroke: color-mix(in srgb, var(--w-fg) 38%, transparent); stroke-width: 2; }
    .seller { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 6px 12px; align-items: center;
      padding: 10px 0; border-top: 1px solid var(--w-glass-edge); }
    .who { display: flex; align-items: baseline; gap: 8px; flex-wrap: wrap; }
    .meter { height: 8px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); overflow: hidden; }
    .meter i { display: block; height: 100%; border-radius: 99px; }
    .bands { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 10px; }
    .bands div { display: grid; gap: 2px; padding: 10px 12px; border-radius: var(--w-radius-dialog);
      background: color-mix(in srgb, var(--w-fg) 4%, transparent); }
    .empty { padding: 32px; display: grid; justify-items: center; text-align: center; gap: 6px; }
    .empty mat-icon { font-size: 40px; width: 40px; height: 40px; color: var(--w-primary); }
    table { width: 100%; border-collapse: collapse; font-size: 14px; }
    th { text-align: left; font-weight: 500; color: var(--w-dim); padding: 6px 8px; }
    td { padding: 6px 8px; border-top: 1px solid var(--w-glass-edge); }
    .r { text-align: right; }
    tr.fill { cursor: pointer; }
    tr.fill:hover, tr.fill.on { background: color-mix(in srgb, var(--w-primary) 7%, transparent); }
    .drill > td { border-top: 0; padding: 4px 8px 12px; }
    .buy { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: 12px; padding: 4px 0; font-size: 13px; }
    .months { display: grid; grid-template-columns: auto 1fr auto; gap: 8px 14px; align-items: center; }
    .bar { height: 8px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); overflow: hidden; }
    .bar i { display: block; height: 100%; background: linear-gradient(90deg, var(--w-primary), var(--w-accent)); border-radius: 99px; }
    @include bp.phone {
      th, td { padding: 6px 4px; }
      .pad { padding: 18px 16px; }
      .opt { display: none; }
      .buy { grid-template-columns: minmax(0, 1fr) auto; }
    }
  `,
})
export class Costs {
  protected readonly wits = inject(Wits);
  protected readonly todo = inject(Todo);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  protected readonly money = euros;
  /** The strain opened in the table, by its key. */
  protected readonly strain = signal<string | null>(null);
  protected readonly openFill = signal<number | null>(null);

  private readonly bought = computed(() => this.wits.costs.value()?.bought ?? []);
  protected readonly series = computed(() => sellerSeries(this.bought()));
  protected readonly sellers = computed(() => [
    ...[...this.series()].map(([name, i]) => ({ name, shape: marker(i, 40), color: seriesColor(i) })),
    ...(this.bought().some((b) => !b.seller) ? [{ name: 'typed in', shape: marker(-1, 40), color: seriesColor(-1) }] : []),
  ]);
  protected readonly recent = computed(() => lately(this.bought()));
  protected readonly monthly = computed(() => {
    const rate = this.wits.overview.value()?.fill?.per_active_day ?? 0;
    return rate > 0 && this.recent().cents ? { grams: rate, cents: month(rate, this.recent().cents) } : null;
  });
  protected readonly timeline = computed<PlotPoint[]>(() => this.bought().map((b) => ({
    id: b.hash, x: new Date(b.at).getTime(), y: b.per_gram, series: this.series().get(b.seller) ?? -1, lines: about(b),
  })));
  protected readonly trend = computed(() => fillTrend(this.bought()));
  protected readonly potency = computed<PlotPoint[]>(() => this.bought().filter((b) => b.thc > 0).map((b) => ({
    id: b.hash, x: b.thc, y: b.per_gram, series: this.series().get(b.seller) ?? -1, lines: about(b),
  })));
  protected readonly strength = computed(() => bands(this.bought()));
  protected readonly dearest = computed(() => Math.max(1, ...(this.wits.costs.value()?.sellers ?? []).map((s) => s.per_gram)));
  protected readonly fills = computed(() => [...(this.wits.costs.value()?.fills ?? [])].reverse());
  protected readonly fillBuys = computed(() => this.bought().filter((b) => b.fill === this.openFill()));
  protected readonly months = computed(() => [...(this.wits.costs.value()?.months ?? [])].reverse().slice(0, 12));
  protected readonly peak = computed(() => Math.max(1, ...this.months().map((m) => m.cents)));

  protected shapeOf(seller: string): string {
    return marker(this.series().get(seller) ?? -1, 40);
  }

  protected colorOf(seller: string): string {
    return seriesColor(this.series().get(seller) ?? -1);
  }

  /** Opens the strain of the purchase a dot stands for, in the table below. */
  protected openStrainOf(hash: string): void {
    const b = this.bought().find((x) => x.hash === hash);
    if (b) {
      this.strain.set(b.strain);
      this.host.nativeElement.querySelector('#strains')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }

  protected toggleFill(n: number): void {
    this.openFill.set(this.openFill() === n ? null : n);
  }
}
