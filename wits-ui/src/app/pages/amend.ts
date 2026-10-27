import { DatePipe, DecimalPipe } from '@angular/common';
import { HttpErrorResponse, httpResource } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatSnackBar } from '@angular/material/snack-bar';

import { Entry, Preview } from '../data/models';
import { Wits } from '../data/wits';
import { FlowPreview, flowOf } from '../inputs/flow';
import { GramsInput } from '../inputs/grams';

/**
 * An entry's amount corrected, as `wits amend` corrects it: undone and
 * recorded again with the amount it should have had, the journal keeping
 * all three. What it does to the jar is the server's rehearsal, and so is a
 * refusal — a jar gives no more than it held.
 */
@Component({
  selector: 'w-amend',
  imports: [DatePipe, DecimalPipe, FlowPreview, GramsInput, MatButtonModule, MatDialogModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h2 mat-dialog-title>Amend this {{ data.verb }}</h2>
    <mat-dialog-content>
      <div class="form">
        <p class="dim">{{ e.grams | number: '1.2-2' }} g of {{ e.product_name ?? 'AVB' }} on {{ e.occurred_at | date: 'd MMMM y, HH:mm' }}.
          Nothing is deleted: it is undone, and recorded again as it should have been.</p>
        <w-grams label="should have been" [value]="grams()" (valueChange)="grams.set($event)" />
        <label class="w-field"><span class="eyebrow">why</span>
          <span class="w-box"><input [value]="note()" (input)="note.set(text($event))" placeholder="misread the scale — optional" /></span></label>
        @if (refusal()) {
          <p class="bad">{{ refusal() }}</p>
        } @else if (flow().length) {
          <w-flow [lines]="flow()" />
        }
      </div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Keep it</button>
      <button mat-flat-button [disabled]="!changed() || !!refusal() || busy()" (click)="amend()">Amend it</button>
    </mat-dialog-actions>
  `,
  styles: `
    .form { display: grid; gap: 16px; padding-top: 6px; width: min(460px, calc(94vw - 52px)); }
    .dim { color: var(--w-dim); margin: 0; font-size: 13px; }
    .eyebrow { font-size: 11px; }
    .bad { color: var(--w-warn); margin: 0; }
  `,
})
export class AmendDialog {
  protected readonly data = inject<{ entry: Entry; verb: string }>(MAT_DIALOG_DATA);
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  private readonly ref = inject(MatDialogRef<AmendDialog>);
  protected readonly e = this.data.entry;
  protected readonly grams = signal<number | null>(this.e.grams);
  protected readonly note = signal('');
  protected readonly busy = signal(false);
  protected readonly changed = computed(() => (this.grams() ?? 0) > 0 && Math.abs((this.grams() ?? 0) - this.e.grams) >= 0.005);

  /** The amendment as the server would record it, rehearsed as the amount is typed. */
  private readonly rehearsal = httpResource<Preview>(() => {
    this.wits.version();
    return this.changed()
      ? { url: '/api/amend?dry=1', method: 'POST', body: { hash: this.e.hash, grams: this.grams(), note: this.note() } }
      : undefined;
  });
  protected readonly refusal = computed(() => {
    const err = this.rehearsal.error() as HttpErrorResponse | undefined;
    return (err?.error as { error?: string } | null)?.error ?? '';
  });
  protected readonly flow = computed(() => (this.rehearsal.hasValue() && !this.refusal() ? flowOf(this.rehearsal.value()) : []));

  protected text(e: Event): string {
    return (e.target as HTMLInputElement).value;
  }

  protected async amend(): Promise<void> {
    this.busy.set(true);
    try {
      const out = await this.wits.amend(this.e.hash, this.grams() ?? 0, this.note().trim());
      this.snack.open(out.message, undefined, { duration: 3500 });
      this.ref.close(true);
    } catch (err) {
      this.snack.open((err as Error).message, 'OK', { duration: 6000 });
    } finally {
      this.busy.set(false);
    }
  }
}
