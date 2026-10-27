import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatSnackBar } from '@angular/material/snack-bar';

import { Jar, ProductLabel } from '../data/models';
import { Wits } from '../data/wits';

/**
 * What a product's label says, corrected, as `wits describe --name` corrects
 * it: its name, maker, cultivar, THC and CBD. The slug its entries name it by
 * stays, and so do its terpenes and its rating, which are written elsewhere.
 * The recorder refuses a product with no name, or a share past 100 %.
 */
@Component({
  selector: 'w-describe',
  imports: [MatButtonModule, MatDialogModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h2 mat-dialog-title>Correct the label</h2>
    <mat-dialog-content>
      <div class="form">
        <label class="w-field name"><span class="eyebrow">name</span>
          <input [value]="l().name" (input)="set('name', text($event))" required /></label>
        <div class="pair">
          <label class="w-field"><span class="eyebrow">maker</span>
            <span class="w-box"><input [value]="l().manufacturer" (input)="set('manufacturer', text($event))" /></span></label>
          <label class="w-field"><span class="eyebrow">cultivar</span>
            <span class="w-box"><input [value]="l().cultivar" (input)="set('cultivar', text($event))" /></span></label>
          <label class="w-field"><span class="eyebrow">THC, %</span>
            <span class="w-box"><input type="number" inputmode="decimal" min="0" max="100" step="0.1" [value]="l().thc || ''"
              (input)="set('thc', +text($event))" /></span></label>
          <label class="w-field"><span class="eyebrow">CBD, %</span>
            <span class="w-box"><input type="number" inputmode="decimal" min="0" max="100" step="0.1" [value]="l().cbd || ''"
              (input)="set('cbd', +text($event))" /></span></label>
        </div>
        <p class="dim">Its entries keep naming it <code>{{ data.slug }}</code>; its terpenes and rating stay as they are.</p>
        @if (error()) { <p class="bad">{{ error() }}</p> }
      </div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Cancel</button>
      <button mat-flat-button [disabled]="busy() || !l().name.trim()" (click)="save()">Save</button>
    </mat-dialog-actions>
  `,
  styles: `
    @use 'breakpoints' as bp;
    .form { display: grid; gap: 16px; padding-top: 6px; width: min(480px, calc(94vw - 52px)); }
    .eyebrow { font-size: 11px; }
    .name input { width: 100%; border: 0; border-bottom: 1px solid var(--w-glass-edge); background: none; color: inherit; outline: 0;
      font: 600 22px var(--w-head); padding: 4px 0 8px; }
    .name input:focus { border-bottom-color: var(--w-primary); }
    .pair { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 16px; }
    .dim { color: var(--w-dim); margin: 0; font-size: 12px; }
    .bad { color: var(--w-warn); margin: 0; }
    @include bp.phone { .pair { grid-template-columns: minmax(0, 1fr); } }
  `,
})
export class DescribeDialog {
  protected readonly data = inject<Jar>(MAT_DIALOG_DATA);
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  private readonly ref = inject(MatDialogRef<DescribeDialog>);
  protected readonly l = signal<ProductLabel>({
    name: this.data.name,
    manufacturer: this.data.manufacturer ?? '',
    cultivar: this.data.cultivar ?? '',
    thc: this.data.thc ?? 0,
    cbd: this.data.cbd ?? 0,
  });
  protected readonly busy = signal(false);
  protected readonly error = signal('');

  protected text(e: Event): string {
    return (e.target as HTMLInputElement).value;
  }

  protected set<K extends keyof ProductLabel>(k: K, v: ProductLabel[K]): void {
    this.l.update((l) => ({ ...l, [k]: v }));
  }

  protected async save(): Promise<void> {
    this.busy.set(true);
    this.error.set('');
    try {
      const out = await this.wits.describe(this.data.slug, this.l());
      this.snack.open(out.message, undefined, { duration: 3000 });
      this.ref.close(true);
    } catch (e) {
      this.error.set(e instanceof Error ? e.message : String(e));
    } finally {
      this.busy.set(false);
    }
  }
}
