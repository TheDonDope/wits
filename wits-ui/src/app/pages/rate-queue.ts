import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';

import { Dimension, Entry } from '../data/models';
import { Wits } from '../data/wits';
import { ScoreDots } from '../viz/score-dots';

/** What each dimension asks, in a word a tap can answer. */
export const ASKS: Record<Dimension, string> = {
  overall: 'overall',
  pain: 'pain relief',
  sleep: 'sleep',
  calm: 'calm',
  mood: 'mood',
  focus: 'focus',
  appetite: 'appetite',
};

/**
 * Sessions waiting for a rating, a card each: a few taps on what it did, a
 * line about it, saved one at a time. The newest three, the rest a tap away.
 */
@Component({
  selector: 'w-rate-queue',
  imports: [DatePipe, DecimalPipe, FormsModule, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, ScoreDots],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="cards">
      @for (s of every() ? sessions() : sessions().slice(0, 3); track s.hash; let i = $index) {
        <article class="card glass" [attr.id]="'rate-' + i">
          <header>
            <strong>{{ s.grams | number: '1.2-2' }} g {{ s.product_name }}</strong>
            <span class="dim">{{ s.occurred_at | date: 'EEE d MMM · HH:mm' }}
              @if (s.device) { · {{ s.device }} } @if (s.temperature) { · {{ s.temperature }} °C }</span>
          </header>
          <div class="dims">
            @for (d of dimensions(); track d) {
              <span class="ask" [class.lead]="d === 'overall'">{{ asks[d] }}</span>
              <w-score-dots [label]="asks[d]" [value]="draft(s.hash)[d] ?? null" (valueChange)="set(s.hash, d, $event)"
                [style.--tone]="d === 'overall' ? 'var(--w-accent)' : 'var(--w-primary)'" />
            }
          </div>
          <mat-form-field appearance="outline" subscriptSizing="dynamic">
            <mat-label>A line about it</mat-label>
            <input matInput [ngModel]="notes()[s.hash] ?? ''" (ngModelChange)="note(s.hash, $event)" />
          </mat-form-field>
          <button mat-flat-button (click)="save(s)" [disabled]="!scored(s.hash) || busy()">
            <mat-icon>done</mat-icon> Save
          </button>
        </article>
      }
    </div>
    @if (sessions().length > 3) {
      <button mat-button class="more" (click)="every.set(!every())">
        <mat-icon>{{ every() ? 'expand_less' : 'expand_more' }}</mat-icon>
        {{ every() ? 'Just the newest' : 'Rate ' + (sessions().length - 3) + ' older' }}
      </button>
    }
  `,
  styles: `
    :host { display: grid; gap: 12px; }
    .more { justify-self: center; }
    .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(330px, 100%), 1fr)); gap: 18px; }
    .card { padding: 18px 20px; display: grid; gap: 14px; }
    .card header { display: grid; gap: 2px; }
    .dims { display: grid; grid-template-columns: auto 1fr; gap: 9px 16px; align-items: center; }
    .ask { font-size: 13px; color: var(--w-dim); }
    .ask.lead { color: var(--mat-sys-on-surface); font-weight: 600; }
  `,
})
export class RateQueue {
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  protected readonly asks = ASKS;

  readonly sessions = input.required<Entry[]>();
  readonly dimensions = input.required<Dimension[]>();
  /** Whether every waiting session is offered, or only the newest three. */
  protected readonly every = signal(false);
  protected readonly busy = signal(false);
  /** Scores being given, per session, before they are saved. */
  private readonly drafts = signal<Record<string, Partial<Record<Dimension, number>>>>({});
  protected readonly notes = signal<Record<string, string>>({});

  protected draft(hash: string): Partial<Record<Dimension, number>> {
    return this.drafts()[hash] ?? {};
  }

  protected scored(hash: string): boolean {
    return Object.keys(this.draft(hash)).length > 0;
  }

  protected set(hash: string, d: Dimension, v: number | null): void {
    this.drafts.update((all) => {
      const kept = Object.entries(all[hash] ?? {}).filter(([k]) => k !== d);
      const next = Object.fromEntries(v === null ? kept : [...kept, [d, v]]) as Partial<Record<Dimension, number>>;
      return { ...all, [hash]: next };
    });
  }

  protected note(hash: string, text: string): void {
    this.notes.update((all) => ({ ...all, [hash]: text }));
  }

  protected async save(s: Entry): Promise<void> {
    this.busy.set(true);
    try {
      const out = await this.wits.rate(s.hash, this.draft(s.hash), this.notes()[s.hash] ?? '');
      this.snack.open(out.message, undefined, { duration: 3000 });
    } catch (err) {
      this.snack.open((err as Error).message, 'OK', { duration: 6000 });
    } finally {
      this.busy.set(false);
    }
  }
}
