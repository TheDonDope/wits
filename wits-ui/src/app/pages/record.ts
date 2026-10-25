import { DecimalPipe, NgTemplateOutlet, PercentPipe } from '@angular/common';
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

import { readLabel } from '../data/label';
import { LabelReading, Written } from '../data/models';
import { Wits } from '../data/wits';

/**
 * Records an entry: a grind, a session, a fill, a weighing, or AVB collected or used. Every check is
 * the recorder's, on the server — what this form refuses is what the command
 * line would refuse, said in the same words. A fill can be read off a
 * photograph of its pharmacy label, on this device (data/label.ts).
 */
@Component({
  selector: 'w-record',
  providers: [provideNativeDateAdapter()],
  imports: [
    DecimalPipe,
    FormsModule,
    NgTemplateOutlet,
    PercentPipe,
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
            <div class="label">
              <button mat-stroked-button type="button" (click)="photo.click()" [disabled]="reading() !== null">
                <mat-icon>photo_camera</mat-icon>Read the label
              </button>
              <input #photo type="file" accept="image/*" capture="environment" hidden (change)="readPhoto(photo)" />
              @if (reading() !== null) {
                <span class="dim num">reading… {{ reading() | percent }}</span>
              } @else {
                <span class="dim">Read on this device; the photograph goes nowhere.</span>
              }
            </div>
            @if (read(); as r) {
              <div class="read">
                <mat-icon>{{ r.reading.name ? 'fact_check' : 'help' }}</mat-icon>
                <div>
                  @if (r.reading.slug) {
                    <p><b>{{ r.reading.name }}</b> — already a jar.</p>
                  } @else if (r.reading.name) {
                    <p><b>{{ r.reading.name }}</b> — a new product. Check the name against the label.</p>
                  } @else if (r.reading.thc) {
                    <p>No product line found; its THC and CBD make it {{ r.reading.thc }}/{{ r.reading.cbd }}. Type the name in.</p>
                  } @else {
                    <p>Nothing on the photograph reads as a label. Type it in, or try a sharper photograph.</p>
                  }
                  <p class="dim">{{ r.reading.grams ? (r.reading.grams | number: '1.2-2') + ' g, as dispensed.' : 'No amount found.' }}</p>
                  <details>
                    <summary class="dim">What it read</summary>
                    <pre>{{ r.text }}</pre>
                  </details>
                </div>
              </div>
            }
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
        <mat-tab><ng-template mat-tab-label><mat-icon>recycling</mat-icon>&nbsp;AVB</ng-template>
          <div class="form">
            <mat-form-field appearance="outline">
              <mat-label>What happened</mat-label>
              <mat-select [ngModel]="avbMode()" (ngModelChange)="avbMode.set($event)">
                <mat-option value="collect">Emptied a device — AVB weighed out</mat-option>
                <mat-option value="use">Used AVB — butter, a tincture</mat-option>
              </mat-select>
            </mat-form-field>
            @if (avbMode() === 'collect') {
              <mat-form-field appearance="outline">
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
    /* The dialog is at most 94vw: less its padding and its border, so the form never pushes past it. */
    .form { display: grid; gap: 4px; padding-top: 18px; width: min(580px, calc(94vw - 52px)); }
    .pair { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
    .temp { display: grid; grid-template-columns: auto 1fr auto; gap: 12px; align-items: center; margin-bottom: 12px; }
    .error { display: flex; gap: 8px; align-items: center; color: var(--w-warn); background: rgba(255,122,138,.08);
      border: 1px solid rgba(255,122,138,.25); border-radius: 12px; padding: 10px 12px; margin-top: 8px; }
    mat-icon { vertical-align: middle; }
    .label { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin-bottom: 14px; }
    .label mat-icon { margin-right: 6px; }
    .read { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 10px; border: 1px solid var(--w-glass-edge);
      border-radius: 12px; padding: 10px 12px; margin-bottom: 14px; }
    .read p { margin: 0 0 4px; }
    .read pre { white-space: pre-wrap; font: 12px var(--w-mono); max-height: 160px; overflow: auto; margin: 6px 0 0; }
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
  protected readonly avbMode = signal<'collect' | 'use'>('collect');
  protected readonly error = signal('');
  protected readonly busy = signal(false);
  /** How far the label is read, 0 to 1, while it is; null otherwise. */
  protected readonly reading = signal<number | null>(null);
  /** The label last read: what the API made of it, and the text it was made of. */
  protected readonly read = signal<{ reading: LabelReading; text: string } | null>(null);

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

  /** Reads the photograph just taken or picked, and fills the form with what the label says. */
  protected async readPhoto(input: HTMLInputElement): Promise<void> {
    const photo = input.files?.[0];
    input.value = '';
    if (!photo) {
      return;
    }
    this.error.set('');
    this.read.set(null);
    this.reading.set(0);
    try {
      const text = await readLabel(photo, (done) => {
        this.reading.set(done);
      });
      const reading = await this.wits.label(text);
      this.read.set({ reading, text: text.trim() });
      if (reading.name) {
        this.name.set(reading.name);
      }
      if (reading.grams) {
        this.grams.set(reading.grams);
      }
    } catch (err) {
      this.error.set(`The label could not be read: ${(err as Error).message}`);
    } finally {
      this.reading.set(null);
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
      case 2:
        return this.wits.buy({ name: this.name(), grams, date });
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
