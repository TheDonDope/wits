import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, model, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { GlassJar, Layer } from '../viz/glass-jar';

/** A jar to pick: its name, the grams that matter here, and its glass filled as it stands. */
export interface JarOption {
  slug: string;
  name: string;
  grams: number;
  /** What the grams are: sealed, ground, already vaped. */
  what: string;
  layers: Layer[];
  capacity: number;
}

/** Past this many jars a filter is offered above them. */
const FILTERED = 8;

/**
 * The jars themselves to pick from, each a small glass filled to what it
 * holds, in a row that scrolls sideways — the shelf in reach rather than a
 * list of names. Many jars bring a filter.
 */
@Component({
  selector: 'w-jar-pick',
  imports: [DecimalPipe, GlassJar, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="head">
      <span class="eyebrow">{{ label() }}</span>
      @if (jars().length > filtered) {
        <label class="filter"><mat-icon>search</mat-icon>
          <input type="search" placeholder="Find a jar" aria-label="Find a jar" [value]="query()" (input)="find($event)" />
        </label>
      }
    </div>
    @if (shown().length) {
      <div class="row" role="radiogroup" [attr.aria-label]="label()">
        @for (j of shown(); track j.slug) {
          <button type="button" role="radio" (keydown)="key($event)" class="jar" [class.on]="value() === j.slug" [attr.aria-checked]="value() === j.slug"
            [tabindex]="value() === j.slug || (!picked() && $first) ? 0 : -1" (click)="value.set(j.slug)">
            <w-jar [capacity]="j.capacity" [layers]="j.layers" />
            <span class="name">{{ j.name }}</span>
            <span class="num g">{{ j.grams | number: '1.2-2' }} g <span class="dim">{{ j.what }}</span></span>
          </button>
        }
      </div>
    } @else {
      <p class="dim none">{{ jars().length ? 'No jar by that name.' : empty() }}</p>
    }
  `,
  styles: `
    :host { display: grid; gap: 8px; min-width: 0; }
    .head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
    .eyebrow { font-size: 11px; }
    .filter { display: flex; align-items: center; gap: 4px; padding: 2px 10px; border-radius: 999px; border: 1px solid var(--w-glass-edge); }
    .filter mat-icon { font-size: 16px; width: 16px; height: 16px; color: var(--w-dim); }
    .filter input { border: 0; outline: 0; background: none; color: inherit; font: 13px var(--w-font); width: 12ch; }
    .row { display: flex; gap: 8px; overflow-x: auto; scroll-snap-type: x proximity; padding: 4px 2px 8px;
      scrollbar-width: none; mask-image: linear-gradient(to right, var(--w-fg) 88%, transparent); }
    .row::-webkit-scrollbar { display: none; }
    .jar { flex: none; width: 112px; scroll-snap-align: start; display: grid; justify-items: center; gap: 4px; padding: 10px 8px;
      border-radius: 16px; border: 1px solid var(--w-glass-edge); background: color-mix(in srgb, var(--w-fg) 3%, transparent);
      color: inherit; font: inherit; cursor: pointer; transition: border-color .2s, background .2s, transform .2s; }
    .jar:hover { border-color: color-mix(in srgb, var(--w-primary) 60%, transparent); }
    .jar:focus-visible { outline: 2px solid var(--w-primary); outline-offset: 2px; }
    .jar.on { border-color: var(--w-primary); background: color-mix(in srgb, var(--w-primary) 14%, transparent); transform: translateY(-2px);
      box-shadow: 0 8px 22px color-mix(in srgb, var(--w-primary) 24%, transparent); }
    .jar w-jar { width: 56px; height: 78px; }
    .name { font-size: 12px; line-height: 1.25; text-align: center; display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2;
      -webkit-box-orient: vertical; overflow: hidden; min-height: 2.5em; }
    .g { font-size: 12px; font-weight: 600; }
    .dim { color: var(--w-dim); font-weight: 400; }
    .none { margin: 0; font-size: 13px; }
    @media (prefers-reduced-motion: reduce) { .jar { transition: none; } .jar.on { transform: none; } }
  `,
})
export class JarPick {
  readonly jars = input.required<JarOption[]>();
  readonly label = input('jar');
  /** What is said when there is no jar to pick. */
  readonly empty = input('Nothing here.');
  readonly value = model('');
  protected readonly filtered = FILTERED;
  protected readonly query = signal('');
  protected readonly shown = computed(() => {
    const q = this.query().trim().toLowerCase();
    return q ? this.jars().filter((j) => j.name.toLowerCase().includes(q) || j.slug.includes(q)) : this.jars();
  });

  protected picked(): boolean {
    return this.shown().some((j) => j.slug === this.value());
  }

  protected find(e: Event): void {
    this.query.set((e.target as HTMLInputElement).value);
  }

  /** Arrow keys move along the row, the focus with the choice. */
  protected key(e: KeyboardEvent): void {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    const all = this.shown();
    if (!step || !all.length) {
      return;
    }
    e.preventDefault();
    const at = all.findIndex((j) => j.slug === this.value());
    const next = all[(at + step + all.length) % all.length];
    this.value.set(next.slug);
    (e.currentTarget as HTMLElement).parentElement?.querySelectorAll('button').item(all.indexOf(next)).focus();
  }
}
