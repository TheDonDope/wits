import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatSnackBar } from '@angular/material/snack-bar';

import { Findings } from '../data/models';
import { Wits } from '../data/wits';

/** A typed list of terpenes as the server takes it: by commas, dominant first. */
export function splitTerpenes(typed: string): string[] {
  return typed.split(/[,;]/).map((t) => t.trim()).filter((t) => t !== '');
}

/**
 * A jar's terpene profile: the one written down, dominant first, or a button
 * that looks it up on the web — a search for the product as the pharmacy
 * sells it, and Leafly's profile of its strain. What is found is offered,
 * with where it was found, to be kept as it is, changed, or let go; nothing is
 * written down until it is kept.
 */
@Component({
  selector: 'w-terpenes',
  imports: [FormsModule, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, MatProgressSpinnerModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (editing()) {
      @if (found(); as f) {
        @if (f.sources.length) {
          <p class="dim note">Found on {{ f.sources.length }} {{ f.sources.length === 1 ? 'page' : 'pages' }} — the terpenes most of them
            name first. Check it against the jar's label before keeping it.</p>
          <ul class="sources">
            @for (s of f.sources; track s.url) {
              <li>
                <a [href]="s.url" target="_blank" rel="noopener noreferrer">{{ s.site }}</a>
                @if (s.strain) { <span class="tag">the strain, not this product</span> }
                <span class="dim">{{ s.terpenes.join(', ') }}</span>
              </li>
            }
          </ul>
        } @else {
          <p class="dim note">No page found names this product's terpenes.
            <a [href]="f.search" target="_blank" rel="noopener noreferrer">Search yourself</a>, or type them from the label.</p>
        }
      }
      <mat-form-field appearance="outline" subscriptSizing="dynamic" class="field">
        <mat-label>Terpenes, dominant first</mat-label>
        <input matInput [(ngModel)]="typed" placeholder="Myrcene, Limonene, Caryophyllene" (keydown.enter)="keep()" />
      </mat-form-field>
      <div class="actions">
        <button mat-flat-button (click)="keep()" [disabled]="saving()"><mat-icon>check</mat-icon>Keep this profile</button>
        <button mat-button (click)="cancel()">Cancel</button>
      </div>
    } @else if (looking()) {
      <p class="looking dim"><mat-spinner diameter="18" /> Searching the web for {{ name() }}…</p>
    } @else if (terpenes().length) {
      <ol class="chips">
        @for (t of terpenes(); track t; let i = $index) {
          <li class="chip" [class.lead]="i === 0">{{ t }}</li>
        }
      </ol>
      <div class="actions">
        <button mat-button (click)="edit()"><mat-icon>edit</mat-icon>Edit</button>
        <button mat-button (click)="research()"><mat-icon>travel_explore</mat-icon>Research again</button>
      </div>
    } @else {
      <p class="dim note">No terpene profile written down yet.</p>
      <div class="actions">
        <button mat-flat-button (click)="research()"><mat-icon>travel_explore</mat-icon>Research terpene profile</button>
        <button mat-button (click)="edit()"><mat-icon>edit</mat-icon>Type it in</button>
      </div>
    }
    @if (error(); as e) { <p class="error">{{ e }}</p> }
  `,
  styles: `
    :host { display: grid; gap: 12px; }
    .note { margin: 0; font-size: 13px; }
    .dim { color: var(--w-dim); }
    .looking { display: flex; gap: 10px; align-items: center; margin: 0; }
    .chips { display: flex; flex-wrap: wrap; gap: 8px; list-style: none; margin: 0; padding: 0; }
    .chip { padding: 3px 12px; border-radius: 99px; font-size: 13px;
      background: color-mix(in srgb, var(--w-stash) 14%, transparent); color: var(--w-stash); }
    .chip.lead { background: color-mix(in srgb, var(--w-stash) 30%, transparent); font-weight: 600; }
    .actions { display: flex; flex-wrap: wrap; gap: 8px; }
    .sources { margin: 0; padding: 0; list-style: none; display: grid; gap: 6px; font-size: 13px; }
    .sources li { display: flex; flex-wrap: wrap; gap: 4px 10px; align-items: baseline; min-width: 0; }
    .sources a { color: var(--w-primary); }
    .tag { font-size: 11px; padding: 0 8px; border-radius: 99px; color: var(--w-warn);
      background: color-mix(in srgb, var(--w-warn) 12%, transparent); }
    .field { width: 100%; }
    .error { margin: 0; color: var(--w-warn); font-size: 13px; }
  `,
})
export class Terpenes {
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  readonly slug = input.required<string>();
  readonly name = input.required<string>();
  readonly terpenes = input.required<string[]>();

  protected readonly looking = signal(false);
  protected readonly editing = signal(false);
  protected readonly saving = signal(false);
  protected readonly found = signal<Findings | null>(null);
  protected readonly error = signal('');
  protected typed = '';

  protected async research(): Promise<void> {
    this.looking.set(true);
    this.error.set('');
    try {
      const f = await this.wits.research(this.slug());
      this.found.set(f);
      this.typed = (f.suggested.length ? f.suggested : this.terpenes()).join(', ');
      this.editing.set(true);
    } catch (e) {
      this.error.set((e as Error).message);
    } finally {
      this.looking.set(false);
    }
  }

  protected edit(): void {
    this.found.set(null);
    this.typed = this.terpenes().join(', ');
    this.editing.set(true);
  }

  protected cancel(): void {
    this.editing.set(false);
    this.found.set(null);
  }

  protected async keep(): Promise<void> {
    this.saving.set(true);
    try {
      const out = await this.wits.terpenes(this.slug(), splitTerpenes(this.typed));
      this.snack.open(out.message, undefined, { duration: 4000 });
      this.editing.set(false);
      this.found.set(null);
    } catch (e) {
      this.error.set((e as Error).message);
    } finally {
      this.saving.set(false);
    }
  }
}
