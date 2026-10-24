import { ChangeDetectionStrategy, Component, DestroyRef, effect, inject, input, signal, untracked } from '@angular/core';

/**
 * A number that counts up to its value — and from the old value to the new
 * one when it changes, so a grind recorded elsewhere is seen to land.
 */
@Component({
  selector: 'w-count',
  template: `{{ shown().toFixed(decimals()) }}`,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'num' },
})
export class CountUp {
  readonly value = input.required<number>();
  readonly decimals = input(2);
  readonly duration = input(1100);

  protected readonly shown = signal(0);
  private frame = 0;

  constructor() {
    inject(DestroyRef).onDestroy(() => {
      cancelAnimationFrame(this.frame);
    });
    effect(() => {
      const to = this.value();
      const from = untracked(this.shown);
      const ms = this.duration();
      cancelAnimationFrame(this.frame);
      if (typeof matchMedia !== 'function' || matchMedia('(prefers-reduced-motion: reduce)').matches) {
        this.shown.set(to);
        return;
      }
      const start = performance.now();
      const step = (now: number) => {
        const t = Math.min((now - start) / ms, 1);
        const eased = 1 - Math.pow(1 - t, 4);
        this.shown.set(from + (to - from) * eased);
        if (t < 1) {
          this.frame = requestAnimationFrame(step);
        }
      };
      this.frame = requestAnimationFrame(step);
    });
  }
}
