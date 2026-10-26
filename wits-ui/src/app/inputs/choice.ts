import { ChangeDetectionStrategy, Component, computed, input, model, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** One thing to choose: what it is called, its icon, and a line under it. */
export interface Choice {
  value: string;
  label: string;
  icon?: string;
  hint?: string;
}

/**
 * One of a few, all in sight: pills in a row for a handful of short words — a
 * mode, an account — or tiles in a grid, each with its icon and a line, for
 * things worth telling apart at a glance, like devices. Arrow keys move the
 * choice, as a radio group's do.
 */
@Component({
  selector: 'w-choice',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (label()) {
      <span class="eyebrow">{{ label() }}</span>
    }
    <div class="set" [class.tiles]="tiles()" role="radiogroup" [attr.aria-label]="label() || null">
      @for (o of shown(); track o.value) {
        <button type="button" role="radio" (keydown)="key($event)" [attr.aria-checked]="value() === o.value" [class.on]="value() === o.value"
          [tabindex]="value() === o.value || (!picked() && $first) ? 0 : -1" (click)="value.set(o.value)">
          @if (o.icon) {
            <mat-icon>{{ o.icon }}</mat-icon>
          }
          <span class="label">{{ o.label }}</span>
          @if (o.hint) {
            <span class="hint">{{ o.hint }}</span>
          }
        </button>
      }
      @if (hidden() > 0) {
        <button type="button" class="more" (click)="open.set(true)"><mat-icon>expand_more</mat-icon>
          <span class="label">{{ hidden() }} more</span></button>
      }
    </div>
  `,
  styles: `
    :host { display: grid; gap: 6px; }
    .eyebrow { font-size: 11px; }
    .set { display: flex; flex-wrap: wrap; gap: 6px; }
    button {
      display: inline-flex; align-items: center; gap: 6px; padding: 7px 14px; border-radius: 999px; cursor: pointer;
      border: 1px solid var(--w-glass-edge); background: color-mix(in srgb, var(--w-fg) 3%, transparent);
      color: inherit; font: 500 13px var(--w-font); transition: border-color .2s, background .2s, transform .2s;
    }
    button:hover { border-color: color-mix(in srgb, var(--w-primary) 60%, transparent); }
    button:focus-visible { outline: 2px solid var(--w-primary); outline-offset: 2px; }
    button.on { border-color: var(--w-primary); background: color-mix(in srgb, var(--w-primary) 18%, transparent); }
    button mat-icon { font-size: 18px; width: 18px; height: 18px; color: var(--w-dim); }
    button.on mat-icon { color: var(--w-primary); }
    .hint { color: var(--w-dim); font: 400 12px var(--w-num); }
    .more { border-style: dashed; color: var(--w-dim); }
    .tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(156px, 1fr)); gap: 8px; }
    .tiles button { display: grid; grid-template-columns: auto minmax(0, 1fr); grid-template-rows: auto auto; gap: 0 8px;
      padding: 10px 12px; border-radius: 14px; text-align: left; }
    .tiles button mat-icon { grid-row: 1 / 3; font-size: 22px; width: 22px; height: 22px; }
    .tiles .label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .tiles .hint { font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .tiles button.on { transform: translateY(-1px); box-shadow: 0 6px 18px color-mix(in srgb, var(--w-primary) 22%, transparent); }
    @media (prefers-reduced-motion: reduce) { button { transition: none; } .tiles button.on { transform: none; } }
  `,
})
export class ChoiceInput {
  readonly options = input.required<Choice[]>();
  readonly label = input('');
  /** Tiles in a grid, each with its icon and line; pills in a row otherwise. */
  readonly tiles = input(false);
  readonly value = model('');
  /** How many to show before the rest fold behind a "more"; all of them at 0. The one chosen always shows. */
  readonly fold = input(0);
  protected readonly open = signal(false);
  protected readonly shown = computed(() => {
    const all = this.options();
    if (!this.fold() || this.open() || all.length <= this.fold() + 1) {
      return all;
    }
    const first = all.slice(0, this.fold());
    const chosen = all.find((o) => o.value === this.value());
    return chosen && !first.includes(chosen) ? [...first, chosen] : first;
  });
  protected readonly hidden = computed(() => this.options().length - this.shown().length);

  protected picked(): boolean {
    return this.options().some((o) => o.value === this.value());
  }

  /** Arrow keys move the choice and the focus with it, round from the ends. */
  protected key(e: KeyboardEvent): void {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    const all = this.shown();
    if (!step || !all.length) {
      return;
    }
    e.preventDefault();
    const at = all.findIndex((o) => o.value === this.value());
    const next = all[(at + step + all.length) % all.length];
    this.value.set(next.value);
    (e.currentTarget as HTMLElement).parentElement?.querySelectorAll('button').item(all.indexOf(next)).focus();
  }
}
