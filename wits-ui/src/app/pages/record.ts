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
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSliderModule } from '@angular/material/slider';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTabsModule } from '@angular/material/tabs';

import { readInvoice } from '../data/invoice';
import { readLabel } from '../data/label';
import { FillLine, InvoiceReading, LabelReading, Written } from '../data/models';
import { Wits } from '../data/wits';

/** The VAT added to a price a gram typed in: Germany's, which medical cannabis pays in full. */
const VAT = 0.19;

/**
 * One strain of a fill, as the form holds it. A strain read off an invoice
 * keeps the invoice's line total until its figures are changed, and its
 * prices include VAT, as an invoice's do: none is ever added to them.
 */
interface Row {
  id: number;
  name: string;
  grams: number | null;
  perGram: number | null;
  /** The line's total in cents, as the invoice prints it. */
  total?: number;
  /** Read off an invoice: its prices are with VAT. */
  inclVAT?: boolean;
  /** The name as the invoice printed it, when it was matched to a jar's. */
  printed?: string;
  /** The invoice's number, written down beside the price. */
  invoice?: string;
}

let nextRow = 0;

/** Whether a strain has anything typed or read into it. */
function started(r: Row): boolean {
  return r.name.trim() !== '' || (r.grams ?? 0) > 0 || (r.perGram ?? 0) > 0;
}

function blank(): Row {
  return { id: ++nextRow, name: '', grams: null, perGram: null };
}

/**
 * Records an entry: a grind, a session, a fill, a weighing, or AVB collected or used. Every check is
 * the recorder's, on the server — what this form refuses is what the command
 * line would refuse, said in the same words. A fill can be read off a
 * photograph of its pharmacy label (data/label.ts), or off the pharmacy's
 * invoices, one or several, on this device (data/invoice.ts); it holds one
 * strain or several, each with its price a gram, and VAT added when the
 * prices are given before it.
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
    MatSlideToggleModule,
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
          <div class="form">
            <div class="label">
              <button mat-stroked-button type="button" (click)="pdf.click()" [disabled]="reading() !== null">
                <mat-icon>receipt_long</mat-icon>Read invoices
              </button>
              <input #pdf type="file" accept="application/pdf,.pdf" multiple hidden (change)="readInvoices(pdf)" />
              <button mat-stroked-button type="button" (click)="photo.click()" [disabled]="reading() !== null">
                <mat-icon>photo_camera</mat-icon>Read the label
              </button>
              <input #photo type="file" accept="image/*" capture="environment" hidden (change)="readPhoto(photo)" />
              @if (reading() !== null) {
                <span class="dim num">reading… {{ reading() | percent }}</span>
              } @else {
                <span class="dim">Read on this device; nothing is sent but its text.</span>
              }
            </div>
            @for (inv of invoices(); track $index) {
              <div class="read">
                <mat-icon>{{ inv.reading.gross_cents && inv.reading.sum_cents !== inv.reading.gross_cents ? 'error' : 'receipt_long' }}</mat-icon>
                <div>
                  <p><b>{{ inv.file }}</b>{{ inv.reading.number ? ' · ' + inv.reading.number : '' }}{{ inv.reading.date ? ' · ' + inv.reading.date : '' }}</p>
                  <p class="dim">
                    {{ inv.reading.lines.length }} {{ inv.reading.lines.length === 1 ? 'strain' : 'strains' }},
                    {{ euros(inv.reading.sum_cents) }}
                    @if (inv.reading.gross_cents) {
                      @if (inv.reading.sum_cents === inv.reading.gross_cents) {
                        — the invoice's total with {{ inv.reading.rate || 19 }}% VAT, to the cent.
                      } @else {
                        — but the invoice says {{ euros(inv.reading.gross_cents) }} with VAT. Check the lines.
                      }
                    }
                  </p>
                </div>
              </div>
            }
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

            @for (l of rows(); track l.id; let i = $index) {
              <div class="strain">
                <mat-form-field appearance="outline" class="name" subscriptSizing="dynamic">
                  <mat-label>{{ rows().length > 1 ? 'Strain ' + (i + 1) : 'Product, as on the label' }}</mat-label>
                  <input matInput [ngModel]="l.name" (ngModelChange)="edit(l.id, { name: $event })" placeholder="Enua 22/1 Wedding Cake" />
                  @if (l.printed && l.printed !== l.name) {
                    <mat-hint>On the invoice: {{ l.printed }}</mat-hint>
                  }
                </mat-form-field>
                <mat-form-field appearance="outline" class="g" subscriptSizing="dynamic">
                  <mat-label>Amount</mat-label>
                  <input matInput type="number" min="0.01" step="0.01" [ngModel]="l.grams" (ngModelChange)="edit(l.id, { grams: $event, total: undefined })" />
                  <span matTextSuffix>g</span>
                </mat-form-field>
                <mat-form-field appearance="outline" class="pg" subscriptSizing="dynamic">
                  <mat-label>Price a gram</mat-label>
                  <input matInput type="number" min="0" step="0.01" [ngModel]="l.perGram" (ngModelChange)="edit(l.id, { perGram: $event, total: undefined })" />
                  <span matTextSuffix>€</span>
                </mat-form-field>
                <span class="total num" [class.dim]="!rowCents(l)">{{ rowCents(l) ? euros(rowCents(l)) : '—' }}</span>
                <button mat-icon-button type="button" class="x" (click)="remove(l.id)" [disabled]="rows().length === 1" [attr.aria-label]="'Remove strain ' + (i + 1)">
                  <mat-icon>close</mat-icon>
                </button>
              </div>
            }
            <div class="more">
              <button mat-button type="button" (click)="add()"><mat-icon>add</mat-icon>Add a strain</button>
            </div>

            <div class="vat">
              <mat-slide-toggle [checked]="vat()" (change)="vat.set($event.checked)">Add 19% VAT (Germany)</mat-slide-toggle>
              @if (fromInvoice()) {
                <span class="dim">The invoice’s prices include VAT already; it is added only to prices typed in.</span>
              }
            </div>
            <dl class="sum">
              @if (vat() && typedCents()) {
                <div><dt>{{ fromInvoice() ? 'Typed in, before VAT' : 'Before VAT' }}</dt><dd class="num">{{ euros(typedCents()) }}</dd></div>
                <div><dt>VAT 19%</dt><dd class="num">{{ euros(vatCents()) }}</dd></div>
              }
              <div class="all"><dt>The fill</dt><dd class="num">{{ totalGrams() | number: '1.0-2' }} g · {{ totalCents() ? euros(totalCents()) : 'no price' }}</dd></div>
            </dl>

            <mat-form-field appearance="outline">
              <mat-label>When</mat-label>
              <input matInput [matDatepicker]="fillPicker" [ngModel]="date()" (ngModelChange)="date.set($event)" placeholder="now" />
              <mat-datepicker-toggle matIconSuffix [for]="fillPicker" />
              <mat-datepicker #fillPicker />
              @if (invoiceDates().length > 1) {
                <mat-hint>The invoices are dated {{ invoiceDates().join(', ') }}: a fill is one pickup, on the day set here.</mat-hint>
              }
            </mat-form-field>
          </div>
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
    @media (width <= 640px) {
      .tab { flex-direction: column; gap: 2px; font-size: 11px; line-height: 1.1; }
    }
    .label { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin-bottom: 14px; }
    .label mat-icon { margin-right: 6px; }
    .read { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 10px; border: 1px solid var(--w-glass-edge);
      border-radius: 12px; padding: 10px 12px; margin-bottom: 14px; }
    .read p { margin: 0 0 4px; }
    /* A strain is a card: its name across with the way to remove it, its figures under it. */
    .strain { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(76px, auto) 40px;
      grid-template-areas: 'name name name x' 'g pg total total'; gap: 4px 10px; align-items: center;
      padding: 12px 8px 8px 12px; margin-bottom: 10px; border: 1px solid var(--w-glass-edge); border-radius: 14px; }
    .strain .name { grid-area: name; }
    .strain .g { grid-area: g; }
    .strain .pg { grid-area: pg; }
    .strain .total { grid-area: total; text-align: right; font-size: 14px; padding-right: 8px; }
    .strain .x { grid-area: x; }
    /* A phone: the line's total on a line of its own. */
    @media (width <= 640px) {
      .strain { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 40px; grid-template-areas: 'name name x' 'g pg pg' 'total total total'; }
    }
    .more { margin: -2px 0 8px; }
    .vat { display: flex; flex-wrap: wrap; gap: 6px 14px; align-items: center; margin: 4px 0 6px; }
    .vat .dim { font-size: 12px; }
    .sum { display: grid; gap: 2px; margin: 4px 0 14px; padding: 10px 12px; border: 1px solid var(--w-glass-edge); border-radius: 12px; }
    .sum div { display: flex; justify-content: space-between; gap: 12px; font-size: 13px; color: var(--w-dim); }
    .sum dd { margin: 0; }
    .sum .all { color: var(--mat-sys-on-surface); font-weight: 600; font-size: 15px; }
    .read pre { white-space: pre-wrap; font: 12px var(--w-mono); max-height: 160px; overflow: auto; margin: 6px 0 0; }
  `,
})
export class RecordDialog {
  protected readonly wits = inject(Wits);
  private readonly ref = inject(MatDialogRef<RecordDialog>);
  private readonly snack = inject(MatSnackBar);

  protected readonly tab = signal(inject<{ tab?: number } | null>(MAT_DIALOG_DATA, { optional: true })?.tab ?? 0);
  protected readonly product = signal('');
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
  /** The strains of the fill being recorded. */
  protected readonly rows = signal<Row[]>([blank()]);
  /** Whether VAT is added to the prices a gram typed in. On: a German pharmacy's prices are quoted before it. */
  protected readonly vat = signal(true);
  /** The invoices read, each with what the API made of it. */
  protected readonly invoices = signal<{ file: string; reading: InvoiceReading }[]>([]);
  protected readonly fromInvoice = computed(() => this.rows().some((r) => r.inclVAT));
  protected readonly invoiceDates = computed(() => [
    ...new Set(this.invoices().flatMap((i) => (i.reading.date ? [i.reading.date] : []))),
  ]);
  /** The prices typed in, before VAT, in cents. */
  protected readonly typedCents = computed(() =>
    this.rows().reduce((sum, r) => sum + (r.inclVAT ? 0 : Math.round((r.grams ?? 0) * (r.perGram ?? 0) * 100)), 0),
  );
  protected readonly totalCents = computed(() => this.rows().reduce((sum, r) => sum + this.rowCents(r), 0));
  protected readonly vatCents = computed(() =>
    this.rows().reduce((sum, r) => sum + (r.inclVAT ? 0 : this.rowCents(r)), 0) - this.typedCents(),
  );
  protected readonly totalGrams = computed(() => this.rows().reduce((sum, r) => sum + (r.grams ?? 0), 0));

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

  /** What a strain costs, in cents: the invoice's total, or its grams at its price, VAT added when it is due. */
  protected rowCents(r: Row): number {
    if (r.total !== undefined) {
      return r.total;
    }
    const net = (r.grams ?? 0) * (r.perGram ?? 0) * 100;
    return Math.round(this.vat() && !r.inclVAT ? net * (1 + VAT) : net);
  }

  protected euros(cents: number): string {
    return (cents / 100).toLocaleString('de-DE', { style: 'currency', currency: 'EUR' });
  }

  protected edit(id: number, patch: Partial<Row>): void {
    this.rows.update((rs) => rs.map((r) => (r.id === id ? { ...r, ...patch } : r)));
  }

  protected add(): void {
    this.rows.update((rs) => [...rs, blank()]);
  }

  protected remove(id: number): void {
    this.rows.update((rs) => (rs.length > 1 ? rs.filter((r) => r.id !== id) : rs));
  }

  /** Strains with something in them; the blank ones the form starts with go. */
  private filled(): Row[] {
    return this.rows().filter(started);
  }

  /** Reads the invoices picked, one after the other, and adds their strains to the fill. */
  protected async readInvoices(input: HTMLInputElement): Promise<void> {
    const files = [...(input.files ?? [])];
    input.value = '';
    if (!files.length) {
      return;
    }
    this.error.set('');
    this.reading.set(0);
    const failed: string[] = [];
    for (const [i, file] of files.entries()) {
      try {
        const reading = await this.wits.invoice(await readInvoice(file));
        this.invoices.update((all) => [...all, { file: file.name, reading }]);
        const read: Row[] = reading.lines.map((l) => ({
          id: ++nextRow,
          name: l.name,
          grams: l.grams,
          perGram: l.per_gram_cents / 100,
          total: l.total_cents,
          inclVAT: true,
          printed: l.printed,
          invoice: reading.number,
        }));
        this.rows.update((rs) => [...rs.filter(started), ...read]);
      } catch (err) {
        failed.push(`${file.name}: ${(err as Error).message}`);
      }
      this.reading.set((i + 1) / files.length);
    }
    // A fill is one pickup: dated the day of the latest invoice, which the field can change.
    const latest = this.invoiceDates().sort().at(-1);
    if (latest) {
      this.date.set(new Date(`${latest}T12:00`));
    }
    this.error.set(failed.length ? `Not read — ${failed.join('; ')}` : '');
    this.reading.set(null);
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
      if (reading.name || reading.grams) {
        // Into the first strain still blank, or a new one.
        const empty = this.rows().find((r) => !r.name.trim() && !r.grams);
        const patch = { name: reading.name, grams: reading.grams ?? null };
        if (empty) {
          this.edit(empty.id, patch);
        } else {
          this.rows.update((rs) => [...rs, { ...blank(), ...patch }]);
        }
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

  /**
   * The strains as the API takes them. A strain off an invoice goes with its
   * total, worked out here when its figures were changed, since its prices
   * include VAT; one typed in goes with its price a gram, VAT added there.
   */
  private fillLines(): FillLine[] {
    return this.filled().map((r) => ({
      name: r.name.trim(),
      grams: r.grams ?? 0,
      total: r.inclVAT ? (this.rowCents(r) / 100).toFixed(2) : undefined,
      per_gram: !r.inclVAT && r.perGram ? String(r.perGram) : undefined,
      note: r.invoice ? `invoice ${r.invoice}` : undefined,
    }));
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
        return this.wits.fill({ lines: this.fillLines(), vat: this.vat(), date });
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
