import { ChangeDetectionStrategy, Component, effect, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';

import { StrainRating } from '../data/models';
import { Wits } from '../data/wits';
import { ScoreDots } from '../viz/score-dots';

/** The scores a strain is rated on as a whole, as the form asks them. */
export const STRAIN_ASKS: { key: 'overall' | 'taste' | 'aroma'; ask: string }[] = [
  { key: 'overall', ask: 'overall' },
  { key: 'taste', ask: 'taste' },
  { key: 'aroma', ask: 'aroma' },
];

/** A strain's rating as a line: "overall 4, taste 5 · again". */
export function ratingLine(r?: StrainRating): string {
  if (!r) {
    return '';
  }
  const scores = STRAIN_ASKS.flatMap((a) => (r[a.key] ? [`${a.ask} ${String(r[a.key])}`] : [])).join(', ');
  const again = r.again === 'yes' ? 'again' : r.again === 'no' ? 'not again' : '';
  return [scores, again].filter((x) => x).join(' · ');
}

/**
 * A strain rated as a whole, apart from any one session: overall, its taste
 * and its aroma, each 1 to 5 or left out — tapping a score again clears it —
 * whether you would have it again, and a line about it. Saving a new rating
 * replaces the one before; clearing takes it away.
 */
@Component({
  selector: 'w-strain-rating',
  imports: [FormsModule, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, ScoreDots],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="dims">
      @for (a of asks; track a.key) {
        <span class="ask" [class.lead]="a.key === 'overall'">{{ a.ask }}</span>
        <w-score-dots [label]="a.ask" [value]="draft()[a.key] ?? null" (valueChange)="set(a.key, $event)"
          [style.--tone]="a.key === 'overall' ? 'var(--w-accent)' : 'var(--w-primary)'" />
      }
      <span class="ask">again?</span>
      <div class="again" role="radiogroup" aria-label="Would you have it again">
        <button type="button" role="radio" [attr.aria-checked]="draft().again === 'yes'" [class.on]="draft().again === 'yes'"
          (click)="again('yes')"><mat-icon>thumb_up</mat-icon>yes</button>
        <button type="button" role="radio" [attr.aria-checked]="draft().again === 'no'" [class.on]="draft().again === 'no'"
          (click)="again('no')"><mat-icon>thumb_down</mat-icon>no</button>
      </div>
    </div>
    <mat-form-field appearance="outline" subscriptSizing="dynamic">
      <mat-label>A line about it</mat-label>
      <input matInput [ngModel]="draft().note ?? ''" (ngModelChange)="note($event)" (keydown.enter)="save()" />
    </mat-form-field>
    <div class="actions">
      <button mat-flat-button (click)="save()" [disabled]="!said() || busy()"><mat-icon>done</mat-icon>Save</button>
      @if (rating()) {
        <button mat-button (click)="clear()" [disabled]="busy()">Take the rating away</button>
      }
    </div>
  `,
  styles: `
    :host { display: grid; gap: 12px; }
    .dims { display: grid; grid-template-columns: auto 1fr; gap: 10px 16px; align-items: center; }
    .ask { font-size: 13px; color: var(--w-dim); }
    .ask.lead { color: var(--mat-sys-on-surface); font-weight: 600; }
    .again { display: flex; gap: 8px; }
    .again button { display: inline-flex; align-items: center; gap: 4px; padding: 3px 12px 3px 8px; border-radius: 99px;
      border: 1px solid color-mix(in srgb, var(--w-fg) 18%, transparent); background: transparent; color: var(--w-dim);
      font: 500 12px var(--w-font); cursor: pointer; }
    .again button mat-icon { font-size: 16px; width: 16px; height: 16px; }
    .again button.on { border-color: var(--w-primary); color: var(--mat-sys-on-surface);
      background: color-mix(in srgb, var(--w-primary) 18%, transparent); }
    .actions { display: flex; flex-wrap: wrap; gap: 8px; }
  `,
})
export class StrainRatingForm {
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  readonly slug = input.required<string>();
  /** The rating it has, to start from. */
  readonly rating = input<StrainRating | undefined>(undefined);
  /** Emits once a rating is saved or taken away. */
  readonly saved = output();

  protected readonly asks = STRAIN_ASKS;
  protected readonly draft = signal<StrainRating>({});
  protected readonly busy = signal(false);

  constructor() {
    // Each time the jar's rating changes under it, the form starts from it again.
    effect(() => {
      this.draft.set({ ...(this.rating() ?? {}) });
    });
  }

  /** Whether the draft says anything: a score, again, or a line. */
  protected said(): boolean {
    const d = this.draft();
    return [d.overall, d.taste, d.aroma].some((x) => !!x) || d.again !== undefined || !!d.note?.trim();
  }

  protected set(key: 'overall' | 'taste' | 'aroma', value: number | null): void {
    this.draft.update((d) => ({ ...d, [key]: value ?? undefined }));
  }

  protected again(value: 'yes' | 'no'): void {
    this.draft.update((d) => ({ ...d, again: d.again === value ? undefined : value }));
  }

  protected note(value: string): void {
    this.draft.update((d) => ({ ...d, note: value }));
  }

  protected async save(): Promise<void> {
    if (!this.said()) {
      return;
    }
    const { overall, taste, aroma, again, note } = this.draft();
    await this.write({ overall, taste, aroma, again, note });
  }

  protected async clear(): Promise<void> {
    await this.write(null);
  }

  private async write(rating: StrainRating | null): Promise<void> {
    this.busy.set(true);
    try {
      const out = await this.wits.rateStrain(this.slug(), rating);
      this.snack.open(out.message, undefined, { duration: 3000 });
      this.saved.emit();
    } catch (e) {
      this.snack.open((e as Error).message, 'OK', { duration: 6000 });
    } finally {
      this.busy.set(false);
    }
  }
}

/** What the rating dialog is opened on. */
export interface RateStrainData {
  slug: string;
  name: string;
  rating?: StrainRating;
}

/** The rating form in a dialog, for a card on the Shelf. */
@Component({
  selector: 'w-rate-strain-dialog',
  imports: [MatDialogModule, StrainRatingForm],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h2 mat-dialog-title>Rate {{ data.name }}</h2>
    <mat-dialog-content>
      <p class="dim">The strain as a whole, apart from any one session.</p>
      <w-strain-rating [slug]="data.slug" [rating]="data.rating" (saved)="ref.close(true)" />
    </mat-dialog-content>
  `,
  styles: `
    .dim { color: var(--w-dim); margin: 0 0 12px; font-size: 13px; }
  `,
})
export class RateStrainDialog {
  protected readonly data = inject<RateStrainData>(MAT_DIALOG_DATA);
  protected readonly ref = inject<MatDialogRef<RateStrainDialog, boolean>>(MatDialogRef);
}
