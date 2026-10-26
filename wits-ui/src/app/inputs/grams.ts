import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** What a press of − or + moves by: a jeweller's scale reads hundredths, a press is five of them. */
const STEP = 0.05;

/** Rounds to the centigram, the precision the ledger keeps. */
function cg(g: number): number {
  return Math.round(g * 100) / 100;
}

/**
 * An amount of grams, said large: typed, stepped by − and +, slid up to what
 * the jar holds, or taken from the amounts used lately with a tap — and all
 * of it, when that is a jar's worth. Null is nothing typed yet.
 */
@Component({
  selector: 'w-grams',
  imports: [DecimalPipe, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <span class="eyebrow">{{ label() }}</span>
    <div class="read">
      <button type="button" class="step" aria-label="Less" (click)="nudge(-1)" [disabled]="(value() ?? 0) <= 0">
        <mat-icon>remove</mat-icon>
      </button>
      <label class="figure">
        <input type="number" inputmode="decimal" min="0" step="0.01" placeholder="0.00" [attr.aria-label]="label() + ', in grams'"
          [value]="value() ?? ''" (input)="typed($event)" />
        <span class="unit">g</span>
      </label>
      <button type="button" class="step" aria-label="More" (click)="nudge(1)" [disabled]="max() !== null && (value() ?? 0) >= max()!">
        <mat-icon>add</mat-icon>
      </button>
    </div>
    @if (max(); as m) {
      <div class="slide">
        <input type="range" min="0" [max]="m" step="0.01" [value]="value() ?? 0" (input)="typed($event)"
          [attr.aria-label]="label() + ', slid'" [style.--fill.%]="share()" />
        <span class="dim num">of {{ m | number: '1.2-2' }} g</span>
      </div>
    }
    @if (lately().length || all()) {
      <div class="chips" role="group" aria-label="Amounts used lately">
        @if (lately().length) {
          <span class="dim">lately</span>
        }
        @for (g of lately(); track g) {
          <button type="button" class="chip" [class.on]="value() === g" (click)="value.set(g)">{{ g | number: '1.2-2' }} g</button>
        }
        @if (all(); as a) {
          <button type="button" class="chip all" [class.on]="value() === a" (click)="value.set(a)">all of it</button>
        }
      </div>
    }
  `,
  styles: `
    :host { display: grid; gap: 8px; }
    .eyebrow { font-size: 11px; }
    .read { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 10px; }
    .step { width: 44px; height: 44px; border-radius: 50%; display: grid; place-items: center; cursor: pointer;
      border: 1px solid var(--w-glass-edge); background: color-mix(in srgb, var(--w-fg) 4%, transparent); color: inherit; }
    .step:hover:not(:disabled) { border-color: var(--w-primary); color: var(--w-primary); }
    .step:disabled { opacity: .35; cursor: default; }
    .figure { display: flex; align-items: baseline; justify-content: center; gap: 4px; padding: 4px 12px; border-radius: 16px;
      background: color-mix(in srgb, var(--w-fg) 4%, transparent); border: 1px solid transparent; cursor: text; }
    .figure:focus-within { border-color: var(--w-primary); }
    .figure input { width: 100%; min-width: 0; max-width: 6ch; border: 0; outline: 0; background: none; color: inherit; text-align: right;
      font: 600 38px/1.1 var(--w-num); letter-spacing: -.02em; -moz-appearance: textfield; appearance: textfield; }
    .figure input::-webkit-outer-spin-button, .figure input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
    .figure input::placeholder { color: color-mix(in srgb, var(--w-fg) 22%, transparent); }
    .unit { font: 500 18px var(--w-num); color: var(--w-dim); }
    .slide { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 12px; align-items: center; font-size: 12px; }
    input[type='range'] { width: 100%; height: 6px; border-radius: 99px; appearance: none; -webkit-appearance: none; cursor: pointer;
      background: linear-gradient(to right, var(--w-primary) var(--fill), color-mix(in srgb, var(--w-fg) 10%, transparent) var(--fill)); }
    input[type='range']::-webkit-slider-thumb { -webkit-appearance: none; width: 18px; height: 18px; border-radius: 50%;
      background: var(--w-primary); box-shadow: 0 0 10px var(--w-primary); }
    input[type='range']::-moz-range-thumb { width: 18px; height: 18px; border: 0; border-radius: 50%;
      background: var(--w-primary); box-shadow: 0 0 10px var(--w-primary); }
    .chips { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; font-size: 12px; }
    .chip { padding: 4px 10px; border-radius: 999px; border: 1px solid var(--w-glass-edge); background: none; color: inherit;
      font: 500 12px var(--w-num); cursor: pointer; }
    .chip:hover { border-color: var(--w-primary); }
    .chip.on { border-color: var(--w-primary); background: color-mix(in srgb, var(--w-primary) 18%, transparent); }
    .chip.all { font-family: var(--w-font); }
    .dim { color: var(--w-dim); }
  `,
})
export class GramsInput {
  readonly value = model<number | null>(null);
  readonly label = input('amount');
  /** The most there is to take; no slider without it. */
  readonly max = input<number | null>(null);
  readonly lately = input<number[]>([]);
  /** Offer all of it: the jar's worth, as one chip. */
  readonly offerAll = input(false);
  protected readonly all = computed(() => (this.offerAll() && (this.max() ?? 0) > 0 ? this.max() : null));
  protected readonly share = computed(() => {
    const m = this.max();
    return m ? Math.min(100, ((this.value() ?? 0) / m) * 100) : 0;
  });

  protected nudge(by: number): void {
    const next = cg(Math.max(0, (this.value() ?? 0) + by * STEP));
    const m = this.max();
    this.value.set(m !== null ? Math.min(next, m) : next);
  }

  protected typed(e: Event): void {
    const el = e.target as HTMLInputElement;
    // Half a number — "0." on the way to "0.4" — reads as nothing; keep what was there until it is whole.
    if (el.validity.badInput) {
      return;
    }
    const v = el.value;
    this.value.set(v === '' ? null : cg(Number(v)));
  }
}
