import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule } from '@angular/material/dialog';

export interface Confirm {
  title: string;
  body: string;
  yes: string;
}

/** Asks before something is recorded that cannot be taken back, only corrected. */
@Component({
  selector: 'w-confirm',
  imports: [MatButtonModule, MatDialogModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h2 mat-dialog-title>{{ data.title }}</h2>
    <mat-dialog-content>{{ data.body }}</mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button [mat-dialog-close]="false" cdkFocusInitial>Keep it</button>
      <button mat-flat-button [mat-dialog-close]="true">{{ data.yes }}</button>
    </mat-dialog-actions>
  `,
})
export class ConfirmDialog {
  protected readonly data = inject<Confirm>(MAT_DIALOG_DATA);
}
