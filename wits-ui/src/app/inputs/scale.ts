import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';

/**
 * A jeweller's scale on the screen: the jar's reading typed into its display,
 * what the ledger says under it, and the difference between them lit as soon
 * as there is one — over in one colour, short in another, level when they
 * agree.
 */
@Component({
  selector: 'w-scale',
  imports: [DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="scale" [class.short]="diff() !== null && diff()! < 0" [class.over]="diff() !== null && diff()! > 0"
      [class.level]="diff() === 0">
      <label class="lcd">
        <input type="number" inputmode="decimal" min="0" step="0.01" placeholder="0.00" aria-label="On the scale, in grams"
          [value]="value() ?? ''" (input)="typed($event)" />
        <span class="g">g</span>
      </label>
      <div class="platform" aria-hidden="true"></div>
      <div class="foot">
        <span class="dim">the ledger says <b class="num">{{ expected() | number: '1.2-2' }} g</b></span>
        @if (diff() !== null) {
          <span class="delta num" aria-live="polite">
            @if (diff() === 0) { level } @else { {{ diff()! > 0 ? '+' : '−' }}{{ abs() | number: '1.2-2' }} g }
          </span>
        }
      </div>
    </div>
  `,
  styles: `
    :host { display: block; --tone: var(--w-dim); }
    .scale { display: grid; gap: 10px; padding: 16px 18px 14px; border-radius: 20px; border: 1px solid var(--w-glass-edge);
      background: linear-gradient(180deg, color-mix(in srgb, var(--w-fg) 6%, transparent), color-mix(in srgb, var(--w-fg) 2%, transparent)); }
    .short { --tone: var(--w-warn); }
    .over { --tone: var(--w-stash); }
    .level { --tone: var(--w-good); }
    .lcd { display: flex; align-items: baseline; justify-content: flex-end; gap: 6px; padding: 10px 16px; border-radius: 12px; cursor: text;
      background: color-mix(in srgb, var(--w-bg) 55%, transparent); border: 1px solid color-mix(in srgb, var(--tone) 40%, transparent);
      box-shadow: inset 0 2px 10px color-mix(in srgb, var(--w-bg) 80%, transparent), 0 0 18px color-mix(in srgb, var(--tone) 18%, transparent); transition: border-color .3s, box-shadow .3s; }
    .lcd:focus-within { border-color: var(--w-primary); }
    .lcd input { width: 100%; min-width: 0; border: 0; outline: 0; background: none; text-align: right;
      color: color-mix(in srgb, var(--tone) 55%, var(--w-fg)); font: 600 44px/1 var(--w-num); letter-spacing: .06em;
      text-shadow: 0 0 12px color-mix(in srgb, var(--tone) 45%, transparent); appearance: textfield; -moz-appearance: textfield; }
    .lcd input::-webkit-outer-spin-button, .lcd input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
    .lcd input::placeholder { color: color-mix(in srgb, var(--w-fg) 18%, transparent); }
    .g { font: 500 18px var(--w-num); color: var(--w-dim); }
    .platform { height: 6px; margin: 0 18%; border-radius: 0 0 8px 8px; background: color-mix(in srgb, var(--w-fg) 12%, transparent); }
    .foot { display: flex; justify-content: space-between; align-items: center; gap: 10px; font-size: 13px; flex-wrap: wrap; }
    .dim { color: var(--w-dim); }
    b { color: var(--w-fg); font-weight: 600; }
    .delta { padding: 2px 10px; border-radius: 99px; font-weight: 600; color: var(--tone);
      background: color-mix(in srgb, var(--tone) 16%, transparent); }
    @media (prefers-reduced-motion: reduce) { .lcd { transition: none; } }
  `,
})
export class ScaleInput {
  readonly value = model<number | null>(null);
  readonly expected = input(0);
  protected readonly diff = computed(() => {
    const v = this.value();
    return v === null ? null : Math.round((v - this.expected()) * 100) / 100;
  });
  protected readonly abs = computed(() => Math.abs(this.diff() ?? 0));

  protected typed(e: Event): void {
    const el = e.target as HTMLInputElement;
    // Half a number — "0." on the way to "0.4" — reads as nothing; keep what was there until it is whole.
    if (el.validity.badInput) {
      return;
    }
    const v = el.value;
    this.value.set(v === '' ? null : Math.round(Number(v) * 100) / 100);
  }
}
