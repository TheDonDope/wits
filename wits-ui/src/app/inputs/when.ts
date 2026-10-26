import { ChangeDetectionStrategy, Component, computed, model, signal } from '@angular/core';

import { ChoiceInput } from './choice';

/** A day as an input of type date writes it: YYYY-MM-DD, in this zone. */
export function isoDay(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${String(d.getFullYear())}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

/** The day before today, at noon, so no zone puts it on another date. */
function yesterday(): Date {
  const d = new Date();
  d.setDate(d.getDate() - 1);
  d.setHours(12, 0, 0, 0);
  return d;
}

/**
 * When it happened: now, which is most of the time, yesterday, or a day picked
 * from the calendar — never one ahead, since nothing is recorded before it
 * has happened. Null is now.
 */
@Component({
  selector: 'w-when',
  imports: [ChoiceInput],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <w-choice label="when" [options]="options" [value]="kind()" (valueChange)="pick($event)" />
    @if (kind() === 'day') {
      <input class="day" type="date" aria-label="The day it happened" [max]="today" [value]="day()" (change)="typed($event)" />
    }
  `,
  styles: `
    :host { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 8px 12px; }
    .day { font: 500 14px var(--w-num); color: inherit; padding: 7px 12px; border-radius: 12px;
      border: 1px solid var(--w-primary); background: color-mix(in srgb, var(--w-primary) 10%, transparent);
      color-scheme: light dark; }
    .day:focus-visible { outline: 2px solid var(--w-primary); outline-offset: 2px; }
  `,
})
export class WhenInput {
  readonly value = model<Date | null>(null);
  protected readonly today = isoDay(new Date());
  protected readonly options = [
    { value: 'now', label: 'Now', icon: 'bolt' },
    { value: 'yesterday', label: 'Yesterday', icon: 'history' },
    { value: 'day', label: 'Another day', icon: 'calendar_month' },
  ];
  /** What is chosen: now, yesterday, or another day, whose day the calendar holds. */
  protected readonly kind = signal<'now' | 'yesterday' | 'day'>('now');
  protected readonly day = computed(() => {
    const v = this.value();
    return v ? isoDay(v) : '';
  });

  protected pick(kind: string): void {
    if (kind === 'now') {
      this.kind.set('now');
      this.value.set(null);
    } else if (kind === 'yesterday') {
      this.kind.set('yesterday');
      this.value.set(yesterday());
    } else {
      // Another day starts from yesterday, picked again on the calendar.
      this.kind.set('day');
      this.value.set(this.value() ?? yesterday());
    }
  }

  protected typed(e: Event): void {
    const v = (e.target as HTMLInputElement).value;
    if (!v) {
      return;
    }
    const [y, m, d] = v.split('-').map(Number);
    this.value.set(new Date(y, m - 1, d, 12));
  }
}
