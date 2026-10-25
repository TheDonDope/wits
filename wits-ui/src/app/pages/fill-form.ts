import { DecimalPipe, PercentPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, model, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { provideNativeDateAdapter } from '@angular/material/core';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';

import { readInvoice } from '../data/invoice';
import { readLabel } from '../data/label';
import { FillLine, InvoiceReading, LabelReading } from '../data/models';
import { Wits } from '../data/wits';

/** The VAT added to a price a gram typed in, in percent: Germany's, which medical cannabis pays in full. */
const VAT = 19;

/**
 * One strain of a fill, as the form holds it. A strain read off an invoice
 * keeps the invoice's line total until its figures are changed, and its
 * prices include VAT, as an invoice's do: none is ever added to them.
 */
export interface Row {
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

function blank(): Row {
  return { id: ++nextRow, name: '', grams: null, perGram: null };
}

/** Whether a strain has anything typed or read into it. */
function started(r: Row): boolean {
  return r.name.trim() !== '' || (r.grams ?? 0) > 0 || (r.perGram ?? 0) > 0;
}

/**
 * What a strain costs, in cents: the invoice's total, or its grams at its
 * price, VAT added when it is due. Worked as the server's lineCents (pkg/api/
 * fill.go) works it — the price a gram in cents first, then the grams, then the
 * VAT, rounded once — so the total on the form is the one written down.
 */
export function rowCents(r: Row, vat: boolean): number {
  if (r.total !== undefined) {
    return r.total;
  }
  const cents = Math.round((r.perGram ?? 0) * 100) * (r.grams ?? 0);
  return Math.round(vat && !r.inclVAT ? cents * (1 + VAT / 100) : cents);
}

/**
 * A fill on Record's Fill tab: one strain or several, each with its grams and
 * its price a gram, read off the pharmacy's invoices (data/invoice.ts) or a
 * photograph of its label (data/label.ts), on this device, or typed in. VAT
 * is added to the prices typed in when the switch says so. The dialog records
 * what `request()` gives it.
 */
@Component({
  selector: 'w-fill-form',
  providers: [provideNativeDateAdapter()],
  imports: [
    DecimalPipe,
    FormsModule,
    PercentPipe,
    MatButtonModule,
    MatDatepickerModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSlideToggleModule,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
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
        <span class="total num" [class.dim]="!cents(l)">{{ cents(l) ? euros(cents(l)) : '—' }}</span>
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
      <input matInput [matDatepicker]="picker" [ngModel]="date()" (ngModelChange)="date.set($event)" placeholder="now" />
      <mat-datepicker-toggle matIconSuffix [for]="picker" />
      <mat-datepicker #picker />
      @if (invoiceDates().length > 1) {
        <mat-hint>The invoices are dated {{ invoiceDates().join(', ') }}: a fill is one pickup, on the day set here.</mat-hint>
      }
    </mat-form-field>
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; gap: 4px; }
    mat-icon { vertical-align: middle; }
    .label { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin-bottom: 14px; }
    .label mat-icon { margin-right: 6px; }
    .read { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 10px; border: 1px solid var(--w-glass-edge);
      border-radius: 12px; padding: 10px 12px; margin-bottom: 14px; }
    .read p { margin: 0 0 4px; }
    .read pre { white-space: pre-wrap; font: 12px var(--w-mono); max-height: 160px; overflow: auto; margin: 6px 0 0; }
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
    @include bp.phone {
      .strain { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 40px; grid-template-areas: 'name name x' 'g pg pg' 'total total total'; }
    }
    .more { margin: -2px 0 8px; }
    .vat { display: flex; flex-wrap: wrap; gap: 6px 14px; align-items: center; margin: 4px 0 6px; }
    .vat .dim { font-size: 12px; }
    .sum { display: grid; gap: 2px; margin: 4px 0 14px; padding: 10px 12px; border: 1px solid var(--w-glass-edge); border-radius: 12px; }
    .sum div { display: flex; justify-content: space-between; gap: 12px; font-size: 13px; color: var(--w-dim); }
    .sum dd { margin: 0; }
    .sum .all { color: var(--mat-sys-on-surface); font-weight: 600; font-size: 15px; }
  `,
})
export class FillForm {
  private readonly wits = inject(Wits);

  /** The day picked up: the dialog's, set from the latest invoice read. */
  readonly date = model<Date | null>(null);
  /** What went wrong reading an invoice or a label, said where the dialog says its errors. */
  readonly error = model('');

  /** How far a label or the invoices are read, 0 to 1, while they are; null otherwise. */
  protected readonly reading = signal<number | null>(null);
  /** The label last read: what the API made of it, and the text it was made of. */
  protected readonly read = signal<{ reading: LabelReading; text: string } | null>(null);
  /** The strains of the fill being recorded. */
  protected readonly rows = signal<Row[]>([blank()]);
  /** Whether VAT is added to the prices a gram typed in. On: a German pharmacy's prices are quoted before it. */
  readonly vat = signal(true);
  /** The invoices read, each with what the API made of it. */
  protected readonly invoices = signal<{ file: string; reading: InvoiceReading }[]>([]);
  protected readonly fromInvoice = computed(() => this.rows().some((r) => r.inclVAT));
  protected readonly invoiceDates = computed(() =>
    [...new Set(this.invoices().flatMap((i) => (i.reading.date ? [i.reading.date] : [])))].sort(),
  );
  /** The prices typed in, before VAT, in cents. */
  protected readonly typedCents = computed(() =>
    this.rows().reduce((sum, r) => sum + (r.inclVAT ? 0 : rowCents(r, false)), 0),
  );
  protected readonly totalCents = computed(() => this.rows().reduce((sum, r) => sum + this.cents(r), 0));
  protected readonly vatCents = computed(() =>
    this.rows().reduce((sum, r) => sum + (r.inclVAT ? 0 : this.cents(r)), 0) - this.typedCents(),
  );
  protected readonly totalGrams = computed(() => this.rows().reduce((sum, r) => sum + (r.grams ?? 0), 0));

  protected cents(r: Row): number {
    return rowCents(r, this.vat());
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

  /**
   * The fill as the API takes it. A strain off an invoice goes with its
   * total, worked out here when its figures were changed, since its prices
   * include VAT; one typed in goes with its price a gram, VAT added there.
   * The blank strains the form starts with go.
   */
  request(): { lines: FillLine[]; vat: boolean } {
    const lines = this.rows()
      .filter(started)
      .map((r) => ({
        name: r.name.trim(),
        grams: r.grams ?? 0,
        total: r.inclVAT ? (this.cents(r) / 100).toFixed(2) : undefined,
        per_gram: !r.inclVAT && r.perGram ? String(r.perGram) : undefined,
        note: r.invoice ? `invoice ${r.invoice}` : undefined,
      }));
    return { lines, vat: this.vat() };
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
    const latest = this.invoiceDates().at(-1);
    if (latest) {
      this.date.set(new Date(`${latest}T12:00`));
    }
    this.error.set(failed.length ? `Not read — ${failed.join('; ')}` : '');
    this.reading.set(null);
  }

  /** Reads the photograph just taken or picked, and fills a strain with what the label says. */
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
        const empty = this.rows().find((r) => !started(r));
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
}
