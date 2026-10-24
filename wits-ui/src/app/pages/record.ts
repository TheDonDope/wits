import { DecimalPipe, NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { provideNativeDateAdapter } from '@angular/material/core';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSliderModule } from '@angular/material/slider';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTabsModule } from '@angular/material/tabs';

import { Written } from '../data/models';
import { Wits } from '../data/wits';

/**
 * Records an entry: a grind, a session, a fill or a weighing. Every check is
 * the recorder's, on the server — what this form refuses is what the command
 * line would refuse, said in the same words.
 */
@Component({
  selector: 'w-record',
  providers: [provideNativeDateAdapter()],
  imports: [
    DecimalPipe,
    FormsModule,
    NgTemplateOutlet,
    MatButtonModule,
    MatDatepickerModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatSliderModule,
    MatTabsModule,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h2 mat-dialog-title>Record</h2>
    <mat-dialog-content>
      <mat-tab-group [selectedIndex]="tab()" (selectedIndexChange)="tab.set($event); error.set('')" animationDuration="200ms">
        <mat-tab><ng-template mat-tab-label><mat-icon>grain</mat-icon>&nbsp;Grind</ng-template>
          <div class="form">
            <mat-form-field appearance="outline">
              <mat-label>From the jar</mat-label>
              <mat-select [ngModel]="product()" (ngModelChange)="product.set($event)">
                @for (j of sealed(); track j.slug) {
                  <mat-option [value]="j.slug">{{ j.name }} <span class="dim">· {{ j.storage | number: '1.2-2' }} g sealed</span></mat-option>
                }
              </mat-select>
            </mat-form-field>
            <ng-container *ngTemplateOutlet="amount" />
          </div>
        </mat-tab>
        <mat-tab><ng-template mat-tab-label><mat-icon>air</mat-icon>&nbsp;Session</ng-template>
          <div class="form">
            <mat-form-field appearance="outline">
              <mat-label>From the stash</mat-label>
              <mat-select [ngModel]="product()" (ngModelChange)="product.set($event)">
                @for (j of stashed(); track j.slug) {
                  <mat-option [value]="j.slug">{{ j.name }} <span class="dim">· {{ j.stash | number: '1.2-2' }} g ground</span></mat-option>
                }
              </mat-select>
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>Device</mat-label>
              <mat-select [ngModel]="device()" (ngModelChange)="pickDevice($event)">
                <mat-option value="">none</mat-option>
                @for (d of wits.devices.value(); track d.slug) {
                  <mat-option [value]="d.slug">{{ d.name }}</mat-option>
                }
              </mat-select>
            </mat-form-field>
            @if (range(); as r) {
              <div class="temp">
                <span class="eyebrow">temperature</span>
                <mat-slider [min]="r.min" [max]="r.max" step="1" discrete>
                  <input matSliderThumb [ngModel]="temp()" (ngModelChange)="temp.set($event)" />
                </mat-slider>
                <span class="num">{{ temp() }} °C</span>
              </div>
            }
            <ng-container *ngTemplateOutlet="amount" />
            <mat-form-field appearance="outline">
              <mat-label>Note</mat-label>
              <input matInput [ngModel]="note()" (ngModelChange)="note.set($event)" />
            </mat-form-field>
          </div>
        </mat-tab>
        <mat-tab><ng-template mat-tab-label><mat-icon>local_pharmacy</mat-icon>&nbsp;Fill</ng-template>
          <div class="form">
            <mat-form-field appearance="outline">
              <mat-label>Product, as on the label</mat-label>
              <input matInput [ngModel]="name()" (ngModelChange)="name.set($event)" placeholder="Enua 22/1 Wedding Cake" />
              <mat-hint>A handle or the full name; a new name adds the product.</mat-hint>
            </mat-form-field>
            <ng-container *ngTemplateOutlet="amount" />
          </div>
        </mat-tab>
        <mat-tab><ng-template mat-tab-label><mat-icon>scale</mat-icon>&nbsp;Weigh</ng-template>
          <div class="form">
            <mat-form-field appearance="outline">
              <mat-label>Jar</mat-label>
              <mat-select [ngModel]="product()" (ngModelChange)="product.set($event)">
                @for (j of wits.jars.value(); track j.slug) {
                  <mat-option [value]="j.slug">{{ j.name }}</mat-option>
                }
              </mat-select>
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>Account</mat-label>
              <mat-select [ngModel]="account()" (ngModelChange)="account.set($event)">
                <mat-option value="storage">Storage — still sealed</mat-option>
                <mat-option value="stash">The stash</mat-option>
                <mat-option value="avb">The AVB jar</mat-option>
              </mat-select>
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>On the scale</mat-label>
              <input matInput type="number" min="0" step="0.01" [ngModel]="grams()" (ngModelChange)="grams.set($event)" />
              <span matTextSuffix>g</span>
            </mat-form-field>
          </div>
        </mat-tab>
      </mat-tab-group>

      <ng-template #amount>
        <div class="pair">
          <mat-form-field appearance="outline">
            <mat-label>Amount</mat-label>
            <input matInput type="number" min="0.01" step="0.01" [ngModel]="grams()" (ngModelChange)="grams.set($event)" />
            <span matTextSuffix>g</span>
          </mat-form-field>
          <mat-form-field appearance="outline">
            <mat-label>When</mat-label>
            <input matInput [matDatepicker]="picker" [ngModel]="date()" (ngModelChange)="date.set($event)" placeholder="now" />
            <mat-datepicker-toggle matIconSuffix [for]="picker" />
            <mat-datepicker #picker />
          </mat-form-field>
        </div>
      </ng-template>

      @if (error()) {
        <div class="error"><mat-icon>block</mat-icon>{{ error() }}</div>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Cancel</button>
      <button mat-flat-button (click)="save()" [disabled]="busy()">Record</button>
    </mat-dialog-actions>
  `,
  styles: `
    .form { display: grid; gap: 4px; padding-top: 18px; min-width: min(460px, 80vw); }
    .pair { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
    .temp { display: grid; grid-template-columns: auto 1fr auto; gap: 12px; align-items: center; margin-bottom: 12px; }
    .error { display: flex; gap: 8px; align-items: center; color: var(--w-warn); background: rgba(255,122,138,.08);
      border: 1px solid rgba(255,122,138,.25); border-radius: 12px; padding: 10px 12px; margin-top: 8px; }
    mat-icon { vertical-align: middle; }
  `,
})
export class RecordDialog {
  protected readonly wits = inject(Wits);
  private readonly ref = inject(MatDialogRef<RecordDialog>);
  private readonly snack = inject(MatSnackBar);

  protected readonly tab = signal(inject<{ tab?: number } | null>(MAT_DIALOG_DATA, { optional: true })?.tab ?? 0);
  protected readonly product = signal('');
  protected readonly name = signal('');
  protected readonly grams = signal<number | null>(null);
  protected readonly date = signal<Date | null>(null);
  protected readonly device = signal('');
  protected readonly temp = signal(185);
  protected readonly note = signal('');
  protected readonly account = signal('storage');
  protected readonly error = signal('');
  protected readonly busy = signal(false);

  protected readonly sealed = computed(() => this.wits.jars.value().filter((j) => j.storage > 0));
  protected readonly stashed = computed(() => this.wits.jars.value().filter((j) => j.stash > 0));
  protected readonly range = computed(() => {
    const d = this.wits.devices.value().find((x) => x.slug === this.device());
    return d ? { min: d.min_temp || 120, max: d.max_temp || 230 } : null;
  });

  protected pickDevice(slug: string): void {
    this.device.set(slug);
    const d = this.wits.devices.value().find((x) => x.slug === slug);
    if (d?.default_temp) {
      this.temp.set(d.default_temp);
    }
  }

  protected async save(): Promise<void> {
    this.error.set('');
    this.busy.set(true);
    try {
      const w = await this.send();
      this.snack.open(w.message, undefined, { duration: 3500 });
      this.ref.close(w);
    } catch (err) {
      this.error.set((err as Error).message);
    } finally {
      this.busy.set(false);
    }
  }

  private send(): Promise<Written> {
    const grams = Number(this.grams() ?? 0);
    const date = this.date() ? isoDay(this.date()!) : undefined;
    switch (this.tab()) {
      case 0:
        return this.wits.grind({ product: this.product(), grams, date });
      case 1:
        return this.wits.sesh({
          product: this.product(),
          grams,
          device: this.device() || undefined,
          temperature: this.device() ? this.temp() : undefined,
          note: this.note() || undefined,
          date,
        });
      case 2:
        return this.wits.buy({ name: this.name(), grams, date });
      default:
        return this.wits.weigh({ product: this.product(), account: this.account(), weighed: grams });
    }
  }
}

function isoDay(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}
