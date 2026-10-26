import { DatePipe, DecimalPipe, PercentPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, model, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';

import { StrainSort, change, euros, sortStrains } from '../data/costs';
import { Bought, CostGroup } from '../data/models';
import { PlotPoint, PricePlot, marker, seriesColor } from '../viz/price-plot';

/** A strain's prices as a sparkline's points, in a box w by h. */
export function spark(prices: number[], w = 84, h = 22): string {
  if (prices.length < 2) {
    return '';
  }
  const lo = Math.min(...prices);
  const hi = Math.max(...prices);
  const span = hi - lo || 1;
  return prices.map((p, i) => `${((i / (prices.length - 1)) * (w - 4) + 2).toFixed(1)},${(h - 2 - ((p - lo) / span) * (h - 4)).toFixed(1)}`).join(' ');
}

/**
 * What each strain cost, a row each: how often it was bought, the grams, what
 * was paid, a gram on average and its range, and how the price moved from the
 * first purchase to the last, drawn as a line. A row opens the strain: every
 * purchase of it, by every producer and pharmacy, priced over time.
 */
@Component({
  selector: 'w-cost-strains',
  imports: [DatePipe, DecimalPipe, MatButtonModule, MatIconModule, MatSlideToggleModule, PercentPipe, PricePlot],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="bar">
      <mat-slide-toggle [checked]="again()" (change)="again.set($event.checked)">only strains bought more than once</mat-slide-toggle>
      <span class="dim">{{ rows().length }} of {{ strains().length }}</span>
    </div>
    <div class="wrap">
      <table>
        <thead>
          <tr>
            @for (c of columns; track c.by) {
              <th [class.r]="c.right" [class.opt]="c.optional">
                <button type="button" (click)="sortBy(c.by)" [attr.aria-sort]="by() === c.by ? (up() ? 'ascending' : 'descending') : null">
                  {{ c.label }}@if (by() === c.by) { <mat-icon>{{ up() ? 'arrow_upward' : 'arrow_downward' }}</mat-icon> }
                </button>
              </th>
            }
            <th class="opt">over time</th>
          </tr>
        </thead>
        <tbody>
          @for (g of rows(); track g.key) {
            <tr [class.on]="picked() === g.key" (click)="pick(g.key)" tabindex="0" (keydown.enter)="pick(g.key)">
              <td><strong>{{ g.name }}</strong><span class="dim small">{{ makers(g) }}</span></td>
              <td class="r num">{{ g.purchases }}</td>
              <td class="r num opt">{{ g.grams | number: '1.0-2' }} g</td>
              <td class="r num">{{ money(g.cents, 0) }}</td>
              <td class="r num">{{ money(g.per_gram) }}<span class="dim small opt">{{ g.purchases > 1 ? money(g.min) + '–' + money(g.max) : '' }}</span></td>
              <td class="r num" [class.up]="delta(g) > 0.005" [class.down]="delta(g) < -0.005">
                {{ g.purchases > 1 ? (delta(g) > 0 ? '+' : '') + (delta(g) | percent: '1.0-0') : '—' }}</td>
              <td class="r opt dim">{{ g.last_at | date: 'MMM y' }}</td>
              <td class="opt">
                @if (sparks().get(g.key); as pts) {
                  <svg width="84" height="22" aria-hidden="true"><polyline [attr.points]="pts" /></svg>
                }
              </td>
            </tr>
            @if (picked() === g.key) {
              <tr class="drill"><td colspan="8">
                <div class="detail">
                  <header>
                    <div>
                      <strong>{{ g.name }}</strong>
                      <span class="dim">{{ g.products.length }} {{ g.products.length === 1 ? 'product' : 'products' }}: {{ g.products.join(' · ') }}</span>
                    </div>
                    <button mat-icon-button type="button" (click)="pick(g.key); $event.stopPropagation()" aria-label="Close"><mat-icon>close</mat-icon></button>
                  </header>
                  @if (picks().length > 1) {
                    <w-price-plot [points]="points()" [label]="'What a gram of ' + g.name + ' cost, purchase by purchase'" />
                  }
                  <table class="buys">
                    <thead><tr><th>bought</th><th>product</th><th class="opt">pharmacy</th><th class="r">grams</th><th class="r">paid</th><th class="r">a gram</th></tr></thead>
                    <tbody>
                      @for (b of picks(); track b.hash) {
                        <tr>
                          <td class="num">{{ b.at | date: 'd MMM y' }}<span class="dim small">prescription {{ b.fill }}</span></td>
                          <td>{{ b.name }}</td>
                          <td class="opt">
                            <svg width="12" height="12" viewBox="-6 -6 12 12" aria-hidden="true"><path [attr.d]="shape(b)" [style.fill]="color(b)" /></svg>
                            {{ b.seller || 'typed in' }}</td>
                          <td class="r num">{{ b.grams | number: '1.0-2' }}</td>
                          <td class="r num">{{ money(b.cents) }}</td>
                          <td class="r num">{{ money(b.per_gram) }}</td>
                        </tr>
                      }
                    </tbody>
                  </table>
                </div>
              </td></tr>
            }
          }
        </tbody>
      </table>
    </div>
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; gap: 10px; }
    .bar { display: flex; justify-content: space-between; align-items: center; gap: 10px; flex-wrap: wrap; }
    .dim { color: var(--w-dim); }
    .small { display: block; font-size: 11px; }
    .wrap { overflow-x: auto; }
    table { width: 100%; border-collapse: collapse; font-size: 14px; }
    th { text-align: left; font-weight: 500; color: var(--w-dim); padding: 4px 8px; white-space: nowrap; }
    th button { all: unset; cursor: pointer; display: inline-flex; align-items: center; gap: 2px; }
    th mat-icon { font-size: 14px; width: 14px; height: 14px; }
    td { padding: 7px 8px; border-top: 1px solid var(--w-glass-edge); vertical-align: middle; }
    tbody > tr:not(.drill) { cursor: pointer; }
    tbody > tr:not(.drill):hover, tr.on { background: color-mix(in srgb, var(--w-primary) 7%, transparent); }
    .r { text-align: right; }
    .up { color: var(--w-warn); }
    .down { color: var(--w-good); }
    polyline { fill: none; stroke: var(--w-primary); stroke-width: 1.6; stroke-linejoin: round; }
    .drill > td { padding: 0 0 12px; border-top: 0; }
    .detail { display: grid; gap: 12px; padding: 14px 16px; border-radius: var(--w-radius-dialog);
      background: color-mix(in srgb, var(--w-fg) 4%, transparent); }
    .detail header { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; }
    .detail header div { display: grid; gap: 2px; }
    .buys { font-size: 13px; }
    .buys td { padding: 5px 8px; }
    @include bp.phone {
      .opt { display: none; }
      th, td { padding: 6px 4px; }
      .detail { padding: 12px 10px; }
    }
  `,
})
export class CostStrains {
  readonly strains = input.required<CostGroup[]>();
  readonly bought = input.required<Bought[]>();
  /** Each pharmacy's series, for its colour and shape. */
  readonly series = input.required<Map<string, number>>();
  /** The strain opened, by its key. */
  readonly picked = model<string | null>(null);

  protected readonly columns: { by: StrainSort; label: string; right?: boolean; optional?: boolean }[] = [
    { by: 'name', label: 'strain' },
    { by: 'purchases', label: 'bought', right: true },
    { by: 'grams', label: 'grams', right: true, optional: true },
    { by: 'cents', label: 'paid', right: true },
    { by: 'per_gram', label: 'a gram', right: true },
    { by: 'change', label: 'first → last', right: true },
    { by: 'last_at', label: 'last', right: true, optional: true },
  ];
  protected readonly by = signal<StrainSort>('cents');
  protected readonly up = signal(false);
  protected readonly again = signal(false);
  protected readonly money = euros;
  protected readonly delta = change;

  protected readonly rows = computed(() =>
    sortStrains(this.strains().filter((g) => !this.again() || g.purchases > 1), this.by(), this.up()));
  protected readonly sparks = computed(() => {
    const byKey = new Map<string, number[]>();
    for (const b of this.bought()) {
      byKey.set(b.strain, [...(byKey.get(b.strain) ?? []), b.per_gram]);
    }
    return new Map([...byKey].map(([k, ps]): [string, string] => [k, spark(ps)]).filter(([, s]) => s));
  });
  protected readonly picks = computed(() => this.bought().filter((b) => b.strain === this.picked()));
  protected readonly points = computed<PlotPoint[]>(() => this.picks().map((b) => ({
    id: b.hash,
    x: new Date(b.at).getTime(),
    y: b.per_gram,
    series: this.series().get(b.seller) ?? -1,
    lines: [b.name, `${euros(b.per_gram)} a gram · ${b.grams} g · ${euros(b.cents)}`,
      `${new Date(b.at).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })} · ${b.seller || 'typed in'}`],
  })));

  protected makers(g: CostGroup): string {
    const names = [...new Set(this.bought().filter((b) => b.strain === g.key).map((b) => b.maker).filter((m) => m))];
    return names.join(' · ');
  }

  protected sortBy(by: StrainSort): void {
    if (this.by() === by) {
      this.up.update((u) => !u);
    } else {
      this.by.set(by);
      this.up.set(by === 'name');
    }
  }

  protected pick(key: string): void {
    this.picked.set(this.picked() === key ? null : key);
  }

  protected shape(b: Bought): string {
    return marker(this.series().get(b.seller) ?? -1, 40);
  }

  protected color(b: Bought): string {
    return seriesColor(this.series().get(b.seller) ?? -1);
  }
}
