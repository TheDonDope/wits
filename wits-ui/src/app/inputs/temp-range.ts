import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';

/**
 * What a device can be set to, from and to, as one band on a scale of
 * temperatures with a thumb at each end — two range inputs laid on one track,
 * so each thumb is its own slider to the keyboard and to a screen reader. The
 * thumbs cannot cross, and the figures are typed beside them as well.
 */
@Component({
  selector: 'w-temp-range',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <span class="eyebrow">it can be set from — to</span>
    <div class="track" [style.--from.%]="share(from() ?? floor())" [style.--to.%]="share(to() ?? ceiling())">
      <div class="band"></div>
      <input type="range" aria-label="The lowest it can be set to" [min]="floor()" [max]="ceiling()" step="5"
        [value]="from() ?? floor()" (input)="slide($event, 'from')" />
      <input type="range" aria-label="The highest it can be set to" [min]="floor()" [max]="ceiling()" step="5"
        [value]="to() ?? ceiling()" (input)="slide($event, 'to')" />
    </div>
    <div class="figs">
      <label class="w-box"><input type="number" aria-label="From, °C" placeholder="—" [value]="from() ?? ''" (input)="typed($event, 'from')" />
        <span class="unit">°C</span></label>
      <span class="dim">to</span>
      <label class="w-box"><input type="number" aria-label="To, °C" placeholder="—" [value]="to() ?? ''" (input)="typed($event, 'to')" />
        <span class="unit">°C</span></label>
    </div>
  `,
  styles: `
    :host { display: grid; gap: 8px; }
    .eyebrow { font-size: 11px; }
    .track { position: relative; height: 28px; }
    .track::before { content: ''; position: absolute; left: 0; right: 0; top: 11px; height: 6px; border-radius: 99px;
      background: color-mix(in srgb, var(--w-fg) 8%, transparent); }
    .band { position: absolute; top: 11px; height: 6px; border-radius: 99px; left: var(--from); right: calc(100% - var(--to));
      background: linear-gradient(to right, var(--w-storage), var(--w-avb), var(--w-warn)); }
    input[type='range'] { position: absolute; inset: 0; width: 100%; margin: 0; background: none; pointer-events: none;
      appearance: none; -webkit-appearance: none; }
    input[type='range']::-webkit-slider-thumb { pointer-events: auto; -webkit-appearance: none; width: 20px; height: 20px;
      border-radius: 50%; background: var(--w-fg); border: 3px solid var(--w-primary); cursor: grab; }
    input[type='range']::-moz-range-thumb { pointer-events: auto; width: 16px; height: 16px; border-radius: 50%;
      background: var(--w-fg); border: 3px solid var(--w-primary); cursor: grab; }
    input[type='range']:focus-visible { outline: none; }
    input[type='range']:focus-visible::-webkit-slider-thumb { box-shadow: 0 0 0 4px color-mix(in srgb, var(--w-primary) 40%, transparent); }
    .figs { display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr); gap: 10px; align-items: center; }
    .dim { color: var(--w-dim); font-size: 13px; }
  `,
})
export class TempRange {
  readonly from = model<number | undefined>(undefined);
  readonly to = model<number | undefined>(undefined);
  /** The scale the band is drawn on. */
  readonly floor = input(0);
  readonly ceiling = input(260);
  protected readonly span = computed(() => Math.max(1, this.ceiling() - this.floor()));

  protected share(t: number): number {
    return ((Math.min(Math.max(t, this.floor()), this.ceiling()) - this.floor()) / this.span()) * 100;
  }

  /** A thumb slid, held back from passing the other. */
  protected slide(e: Event, end: 'from' | 'to'): void {
    const v = Number((e.target as HTMLInputElement).value);
    if (end === 'from') {
      this.from.set(Math.min(v, this.to() ?? this.ceiling()));
    } else {
      this.to.set(Math.max(v, this.from() ?? this.floor()));
    }
  }

  protected typed(e: Event, end: 'from' | 'to'): void {
    const el = e.target as HTMLInputElement;
    if (el.validity.badInput) {
      return;
    }
    const v = el.value === '' ? undefined : Number(el.value);
    if (end === 'from') {
      this.from.set(v);
    } else {
      this.to.set(v);
    }
  }
}
