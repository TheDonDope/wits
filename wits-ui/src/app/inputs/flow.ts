import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

/** One account of the jar, before the entry and after it. */
export interface FlowLine {
  label: string;
  before: number;
  after: number;
  /** The account's colour, a theme variable. */
  tone: string;
}

/**
 * What the entry will do to the jar, before it is recorded: each account it
 * moves grams between as a bar, what stays solid, what goes faded, what
 * comes lit — the jar's own grams, so it says the jar.
 */
@Component({
  selector: 'w-flow',
  imports: [DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="head"><span class="eyebrow">what this does</span><span class="scope">the jar</span></div>
    @for (l of rows(); track l.label) {
      <div class="line" [style.--tone]="l.tone">
        <span class="label">{{ l.label }}</span>
        <div class="bar" aria-hidden="true">
          <div class="kept" [style.width.%]="l.kept"></div>
          <div class="moved" [class.in]="l.after > l.before" [style.left.%]="l.kept" [style.width.%]="l.moved"></div>
        </div>
        <span class="num figs">{{ l.before | number: '1.2-2' }}<span class="arrow">→</span><b>{{ l.after | number: '1.2-2' }}</b> g</span>
      </div>
    }
  `,
  styles: `
    :host { display: grid; gap: 8px; padding: 12px 14px; border-radius: 14px; background: color-mix(in srgb, var(--w-fg) 3%, transparent);
      border: 1px dashed var(--w-glass-edge); }
    .head { display: flex; justify-content: space-between; align-items: center; }
    .eyebrow { font-size: 11px; }
    .scope { font-size: 11px; color: var(--w-dim); }
    .line { display: grid; grid-template-columns: 9ch minmax(0, 1fr) auto; gap: 10px; align-items: center; font-size: 12px; }
    .label { color: var(--w-dim); }
    .bar { position: relative; height: 8px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 7%, transparent); overflow: hidden; }
    .kept, .moved { position: absolute; top: 0; bottom: 0; left: 0; border-radius: 99px; transition: width .35s, left .35s; }
    .kept { background: var(--tone); }
    .moved { background: repeating-linear-gradient(45deg, color-mix(in srgb, var(--tone) 45%, transparent) 0 4px, transparent 4px 7px); }
    .moved.in { background: var(--tone); box-shadow: 0 0 10px var(--tone); opacity: .7; }
    .figs { white-space: nowrap; }
    .arrow { color: var(--w-dim); margin: 0 4px; }
    b { font-weight: 600; }
    @media (prefers-reduced-motion: reduce) { .kept, .moved { transition: none; } }
  `,
})
export class FlowPreview {
  readonly lines = input.required<FlowLine[]>();
  /** Every bar against the fullest figure, so the accounts compare. */
  protected readonly rows = computed(() => {
    const top = Math.max(0.01, ...this.lines().flatMap((l) => [l.before, l.after]));
    return this.lines().map((l) => ({
      ...l,
      kept: (Math.min(l.before, l.after) / top) * 100,
      moved: (Math.abs(l.after - l.before) / top) * 100,
    }));
  });
}
