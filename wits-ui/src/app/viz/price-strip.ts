import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { YearBought } from '../data/models';

/**
 * A year's purchases along one axis, their price a gram, the cheapest and
 * the dearest flagged at either end: how far apart a gram could cost.
 */
@Component({
  selector: 'w-price-strip',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="strip" role="img" [attr.aria-label]="aria()">
      <span class="axis"></span>
      @for (d of dots(); track $index) {
        <i class="dot" [style.left.%]="d.x" [style.--s]="d.size" [style.--i]="$index" [title]="d.title"></i>
      }
      <span class="tag lo" [style.left.%]="0"><b>{{ euro(lo()) }}</b><small>cheapest</small></span>
      <span class="tag hi" [style.left.%]="100"><b>{{ euro(hi()) }}</b><small>dearest</small></span>
      @if (mean() !== null) {
        <span class="mean" [style.left.%]="mean()"><small>the year's</small></span>
      }
    </div>
  `,
  styles: `
    :host { display: block; padding: 0 28px; }
    .strip { position: relative; height: 92px; }
    .axis { position: absolute; left: 0; right: 0; top: 38px; height: 4px; border-radius: 2px;
      background: linear-gradient(90deg, var(--w-good), var(--w-avb), var(--w-warn)); opacity: .5; }
    .dot { position: absolute; top: calc(40px - var(--s) / 2); width: var(--s); height: var(--s); margin-left: calc(var(--s) / -2);
      border-radius: 50%; background: color-mix(in srgb, var(--w-fg) 70%, transparent); border: 2px solid var(--w-ink);
      animation: drop .6s cubic-bezier(.2,.8,.2,1) both; animation-delay: calc(var(--i) * 30ms); }
    .tag { position: absolute; top: 56px; transform: translateX(-50%); display: grid; justify-items: center; line-height: 1.1; }
    .tag b { font: 600 16px var(--w-num); }
    .lo b { color: var(--w-good); } .hi b { color: var(--w-warn); }
    .tag small, .mean small { font-size: 10px; color: var(--w-dim); text-transform: uppercase; letter-spacing: .08em; }
    .mean { position: absolute; top: 0; transform: translateX(-50%); display: grid; justify-items: center; }
    .mean::after { content: ''; width: 2px; height: 22px; margin-top: 2px; background: var(--w-fg); border-radius: 1px; }
    @keyframes drop { from { opacity: 0; transform: translateY(-14px); } }
  `,
})
export class PriceStrip {
  readonly bought = input.required<YearBought[]>();
  /** The year's price a gram over everything, cents. */
  readonly perGram = input<number | null>(null);

  protected readonly lo = computed(() => Math.min(...this.bought().map((b) => b.per_gram)));
  protected readonly hi = computed(() => Math.max(...this.bought().map((b) => b.per_gram)));
  private readonly x = (c: number) => (this.hi() > this.lo() ? ((c - this.lo()) / (this.hi() - this.lo())) * 100 : 50);

  protected readonly dots = computed(() => {
    const most = Math.max(...this.bought().map((b) => b.grams), 1);
    return this.bought().map((b) => ({ x: this.x(b.per_gram), size: `${String(8 + (b.grams / most) * 12)}px`,
      title: `${b.name}: ${this.euro(b.per_gram)} a gram, ${String(b.grams)} g` }));
  });

  protected readonly mean = computed(() => { const m = this.perGram(); return m === null ? null : this.x(m); });

  protected readonly aria = computed(() =>
    `A gram cost from ${this.euro(this.lo())} to ${this.euro(this.hi())} over ${String(this.bought().length)} purchases`);

  protected euro(cents: number): string {
    return `€${(cents / 100).toFixed(2)}`;
  }
}
