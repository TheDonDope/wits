import { DestroyRef, ElementRef, Signal, afterNextRender, inject, signal } from '@angular/core';

/**
 * The host element's width as a signal, kept current by a ResizeObserver, so
 * a chart draws at its real size instead of being stretched to it.
 */
export function hostWidth(fallback = 600): Signal<number> {
  const el = inject(ElementRef<HTMLElement>);
  const width = signal(fallback);
  const destroy = inject(DestroyRef);
  afterNextRender(() => {
    const ro = new ResizeObserver(([entry]) => width.set(Math.max(entry.contentRect.width, 120)));
    ro.observe(el.nativeElement);
    destroy.onDestroy(() => ro.disconnect());
  });
  return width;
}

/** Grams as wits writes them: two places, and a thin space before the unit. */
export function grams(g: number): string {
  return `${g.toFixed(2)} g`;
}
