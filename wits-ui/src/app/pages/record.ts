import { DecimalPipe, NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal, viewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { provideNativeDateAdapter } from '@angular/material/core';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MAT_DIALOG_DATA, MatDialog, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSliderModule } from '@angular/material/slider';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTabsModule } from '@angular/material/tabs';

import { Written } from '../data/models';
import { Wits } from '../data/wits';
import { FillForm } from './fill-form';

/** What the dialog opens on: a tab, and the jar or the device already picked. */
export interface RecordData {
  tab?: number;
  product?: string;
  device?: string;
}

/** The record dialog's tabs, in order, as a link names them. */
export const RECORD_TABS = ['grind', 'session', 'fill', 'weigh', 'avb'];

/** Opens the record dialog: on the shell's Record, a key, a link, or a thing forgotten. */
export function openRecord(dialog: MatDialog, data: RecordData = {}): void {
  dialog.open(RecordDialog, {
    autoFocus: 'first-tabbable',
    panelClass: 'record-panel',
    // Five tabs want more than Material's 560 px; a phone still gets the screen less a margin.
    maxWidth: 'min(660px, 94vw)',
    data: { ...data, tab: Math.max(data.tab ?? 0, 0) },
  });
}

/**
 * Records an entry: a grind, a session, a fill, a weighing, or AVB collected or used. Every check is
 * the recorder's, on the server — what this form refuses is what the command
 * line would refuse, said in the same words. A fill, its strains, its prices
 * and the invoices and labels it is read off, is a form of its own (FillForm).
 */
@Component({
  selector: 'w-record',
  providers: [provideNativeDateAdapter()],
  imports: [
    DecimalPipe,
    FillForm,
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
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>grain</mat-icon><span>Grind</span></span></ng-template>
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
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>air</mat-icon><span>Session</span></span></ng-template>
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
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>local_pharmacy</mat-icon><span>Fill</span></span></ng-template>
          <w-fill-form class="form" [(date)]="date" [(error)]="error" />
        </mat-tab>
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>scale</mat-icon><span>Weigh</span></span></ng-template>
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
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>recycling</mat-icon><span>AVB</span></span></ng-template>
          <div class="form">
            <mat-form-field appearance="outline">
              <mat-label>What happened</mat-label>
              <mat-select [ngModel]="avbMode()" (ngModelChange)="avbMode.set($event)">
                <mat-option value="collect">Emptied a device — AVB weighed out</mat-option>
                <mat-option value="use">Used AVB — butter, a tincture</mat-option>
              </mat-select>
            </mat-form-field>
            @if (avbMode() === 'collect') {
              <mat-form-field appearance="outline" subscriptSizing="dynamic" class="hinted">
                <mat-label>The device emptied</mat-label>
                <mat-select [ngModel]="emptied()" (ngModelChange)="device.set($event)">
                  @for (w of wits.avb.value()?.waiting ?? []; track w.device) {
                    <mat-option [value]="w.device">{{ deviceName(w.device) }}
                      <span class="dim">· {{ w.grams | number: '1.2-2' }} g in {{ w.sessions }} {{ w.sessions === 1 ? 'session' : 'sessions' }}</span>
                    </mat-option>
                  }
                </mat-select>
                <mat-hint>From its sessions since it was last emptied.</mat-hint>
              </mat-form-field>
            } @else {
              <p class="dim">{{ wits.avb.value()?.held ?? 0 | number: '1.2-2' }} g in the AVB jar.</p>
            }
            <ng-container *ngTemplateOutlet="amount" />
            <mat-form-field appearance="outline">
              <mat-label>Note</mat-label>
              <input matInput [ngModel]="note()" (ngModelChange)="note.set($event)" />
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
    @use 'breakpoints' as bp;
    /* The dialog is at most 94vw: less its padding and its border, so the form never pushes past it. */
    .form { display: grid; gap: 4px; padding-top: 18px; width: min(580px, calc(94vw - 52px)); }
    .pair { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
    .temp { display: grid; grid-template-columns: auto 1fr auto; gap: 12px; align-items: center; margin-bottom: 12px; }
    .error { display: flex; gap: 8px; align-items: center; color: var(--w-warn); background: color-mix(in srgb, var(--w-warn) 8%, transparent);
      border: 1px solid color-mix(in srgb, var(--w-warn) 25%, transparent); border-radius: 12px; padding: 10px 12px; margin-top: 8px; }
    mat-icon { vertical-align: middle; }
    /* A hint that wraps on a phone takes its room, rather than running into the field under it. */
    .hinted { margin-bottom: 18px; }
    .tab { display: inline-flex; align-items: center; gap: 6px; }
    /* A phone: the five tabs side by side, each its icon over its name, none paged out of sight. */
    @include bp.phone {
      .tab { flex-direction: column; gap: 2px; font-size: 11px; line-height: 1.1; }
    }
  `,
})
export class RecordDialog {
  protected readonly wits = inject(Wits);
  private readonly ref = inject(MatDialogRef<RecordDialog>);
  private readonly snack = inject(MatSnackBar);

  private readonly data = inject<RecordData | null>(MAT_DIALOG_DATA, { optional: true });
  protected readonly tab = signal(this.data?.tab ?? 0);
  protected readonly product = signal(this.data?.product ?? '');
  protected readonly grams = signal<number | null>(null);
  protected readonly date = signal<Date | null>(null);
  protected readonly device = signal(this.data?.device ?? '');
  protected readonly temp = signal(185);
  protected readonly note = signal('');
  protected readonly account = signal('storage');
  protected readonly avbMode = signal<'collect' | 'use'>('collect');
  protected readonly error = signal('');
  protected readonly busy = signal(false);
  /** The Fill tab's form, which says what a fill records. */
  private readonly fill = viewChild(FillForm);

  protected readonly sealed = computed(() => this.wits.jars.value().filter((j) => j.storage > 0));
  protected readonly stashed = computed(() => this.wits.jars.value().filter((j) => j.stash > 0));
  /** The device emptied: the one picked, or the only one with sessions waiting. */
  protected readonly emptied = computed(() => {
    const waiting = this.wits.avb.value()?.waiting ?? [];
    return this.device() || (waiting.length === 1 ? waiting[0].device : '');
  });
  protected readonly range = computed(() => {
    const d = this.wits.devices.value().find((x) => x.slug === this.device());
    // The API leaves a range out when the device has none.
    return d ? { min: d.min_temp ?? 120, max: d.max_temp ?? 230 } : null;
  });

  /** A device's name, or what a session that named none is called. */
  protected deviceName(slug: string): string {
    return slug ? (this.wits.devices.value().find((d) => d.slug === slug)?.name ?? slug) : 'no device named';
  }

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

  private send(): Promise<Written | { message: string }> {
    const grams = this.grams() ?? 0;
    const day = this.date();
    const date = day ? isoDay(day) : undefined;
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
      case 2: {
        const fill = this.fill();
        return fill ? this.wits.fill({ ...fill.request(), date }) : Promise.reject(new Error('the fill form is not open'));
      }
      case 4:
        return this.avbMode() === 'collect'
          ? this.wits.collectAVB({ grams, device: this.emptied(), note: this.note() || undefined, date })
          : this.wits.useAVB({ grams, note: this.note() || undefined, date });
      default:
        return this.wits.weigh({ product: this.product(), account: this.account(), weighed: grams });
    }
  }
}

function isoDay(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}
