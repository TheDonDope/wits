import { DecimalPipe, PercentPipe } from '@angular/common';
import { HttpErrorResponse, httpResource } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, computed, inject, model, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { addsUp, readInvoice } from '../data/invoice';
import { readLabel } from '../data/label';
import { FillLine, InvoiceReading, LabelReading, Preview } from '../data/models';
import { Wits } from '../data/wits';
import { Choice, ChoiceInput } from '../inputs/choice';
import { WhenInput } from '../inputs/when';
import { GlassJar } from '../viz/glass-jar';
import { tone } from '../viz/tones';

/** The VAT added to a price a gram typed in, in percent: Germany's, which medical cannabis pays in full. */
export const VAT = 19;

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

/** An invoice read on the form, and what became of it. */
interface Read {
  file: string;
  reading: InvoiceReading;
  state: 'new' | 'matched' | 'ask' | 'priced';
}

let nextRow = 0;

/** The amounts that come up most, four at most, smallest first. */
function sizes(grams: number[]): number[] {
  const count = new Map<number, number>();
  for (const g of grams) {
    count.set(g, (count.get(g) ?? 0) + 1);
  }
  return [...count.entries()].sort((a, b) => b[1] - a[1] || a[0] - b[0]).slice(0, 4).map(([g]) => g).sort((a, b) => a - b);
}

function blank(): Row {
  return { id: ++nextRow, name: '', grams: null, perGram: null };
}

/** Whether a strain has anything typed or read into it. */
function started(r: Row): boolean {
  return r.name.trim() !== '' || (r.grams ?? 0) > 0 || (r.perGram ?? 0) > 0;
}

/** Whether a strain is whole enough to be priced: a name and some grams. */
function whole(r: Row): boolean {
  return r.name.trim() !== '' && (r.grams ?? 0) > 0;
}

/** A strain as the API takes it: an invoice's total as it was printed, or the price a gram typed in, said to have its VAT or not. */
export function lineOf(r: Row): FillLine {
  return {
    name: r.name.trim(),
    grams: r.grams ?? 0,
    total: r.total !== undefined ? (r.total / 100).toFixed(2) : undefined,
    per_gram: r.total === undefined && r.perGram ? String(r.perGram) : undefined,
    with_vat: r.inclVAT && r.total === undefined ? true : undefined,
    note: r.invoice ? `invoice ${r.invoice}` : undefined,
  };
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
  imports: [ChoiceInput, DecimalPipe, GlassJar, PercentPipe, MatButtonModule, MatIconModule, WhenInput],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="readers">
      <button type="button" class="reader" [class.over]="over()" [disabled]="reading() !== null" (click)="pdf.click()"
        (dragover)="drag($event, true)" (dragleave)="drag($event, false)" (drop)="dropped($event)">
        <mat-icon>receipt_long</mat-icon>
        <strong>Read invoices</strong>
        <span class="dim">The pharmacy's PDFs — pick them, or drop them here. Every strain and price, read.</span>
      </button>
      <input #pdf type="file" accept="application/pdf,.pdf" multiple hidden (change)="readInvoices(pdf)" />
      <button type="button" class="reader" [disabled]="reading() !== null" (click)="photo.click()">
        <mat-icon>photo_camera</mat-icon>
        <strong>Read the label</strong>
        <span class="dim">A photo of the jar's label: the product and its grams.</span>
      </button>
      <input #photo type="file" accept="image/*" capture="environment" hidden (change)="readPhoto(photo)" />
    </div>
    @if (reading() !== null) {
      <div class="progress" role="progressbar" [attr.aria-valuenow]="(reading() ?? 0) * 100" aria-valuemin="0" aria-valuemax="100">
        <div [style.width.%]="(reading() ?? 0) * 100"></div><span class="dim num">reading… {{ reading() | percent }}</span>
      </div>
    } @else {
      <p class="dim local"><mat-icon>lock</mat-icon>Read on this device; nothing is sent but its text.</p>
    }
    @for (inv of invoices(); track $index) {
      <div class="read">
        <mat-icon>{{ addsUp(inv.reading) ? 'receipt_long' : 'error' }}</mat-icon>
        <div>
          <p><b>{{ inv.file }}</b>{{ inv.reading.number ? ' · ' + inv.reading.number : '' }}{{ inv.reading.date ? ' · ' + inv.reading.date : '' }}</p>
          <p class="dim">
            {{ inv.reading.lines.length }} {{ inv.reading.lines.length === 1 ? 'strain' : 'strains' }},
            {{ euros(inv.reading.sum_cents) }}
            @if (inv.reading.gross_cents) {
              @if (addsUp(inv.reading)) {
                — {{ inv.reading.other.length ? 'with what was left out, ' : '' }}the invoice's total with {{ inv.reading.rate || 19 }}% VAT, to the cent.
              } @else {
                — but the invoice says {{ euros(inv.reading.gross_cents) }} with VAT. Check the lines.
              }
            }
          </p>
          @for (o of inv.reading.other; track $index) {
            <p class="dim">Left out, not flower by the gram: {{ o.name }}, {{ euros(o.total_cents) }}.</p>
          }
          @if (inv.reading.match; as m) {
            @switch (inv.state) {
              @case ('matched') {
                <p class="known">Already recorded: Prescription {{ m.fill }}, picked up {{ day(m.start) }}{{
                  m.priced ? ', which has its prices already' : '' }}. Its strains are not added again.</p>
              }
              @case ('ask') {
                <p class="known">Prescription {{ m.fill }} was picked up {{ day(m.start) }} with its strains,
                  {{ m.days }} {{ m.days === 1 ? 'day' : 'days' }} from the invoice{{
                  m.priced ? ', and has its prices already' : '' }}. Is it that prescription?</p>
              }
              @case ('priced') {
                <p class="known">Prescription {{ m.fill }} priced from it.</p>
              }
            }
            @if (inv.state === 'matched' || inv.state === 'ask') {
              <div class="choose">
                <button mat-flat-button type="button" [disabled]="pricing()" (click)="priceMatched($index)">
                  {{ inv.state === 'ask' ? 'Yes — price' : 'Price' }} Prescription {{ m.fill }} from it
                </button>
                <button mat-button type="button" (click)="asNew($index)">{{ inv.state === 'ask' ? 'No, a new prescription' : 'Record it as a new prescription' }}</button>
              </div>
            }
          }
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

    <datalist id="fill-jars">
      @for (j of wits.jars.value(); track j.slug) {
        <option [value]="j.name"></option>
      }
    </datalist>
    @for (l of rows(); track l.id; let i = $index) {
      <div class="strain" [style.--tone]="toneOf(i)">
        <w-jar class="glass" [capacity]="capacity()" [layers]="[{ grams: l.grams ?? 0, color: toneOf(i), label: 'dispensed' }]" />
        <div class="body">
          <div class="top">
            <label class="name">
              <span class="eyebrow">{{ rows().length > 1 ? 'strain ' + (i + 1) : 'product, as on the label' }}</span>
              <input list="fill-jars" [value]="l.name" (input)="edit(l.id, { name: text($event) })" placeholder="Enua 22/1 Wedding Cake" />
            </label>
            <button mat-icon-button type="button" class="x" (click)="remove(l.id)" [disabled]="rows().length === 1"
              [attr.aria-label]="'Remove strain ' + (i + 1)"><mat-icon>close</mat-icon></button>
          </div>
          @if (l.name.trim()) {
            <p class="kind" [class.new]="!jarOf(l)">
              <mat-icon>{{ jarOf(l) ? 'inventory_2' : 'fiber_new' }}</mat-icon>
              {{ jarOf(l) ? 'already a jar' : 'a new product' }}
              @if (l.printed && l.printed !== l.name) { <span class="dim">· on the invoice: {{ l.printed }}</span> }
            </p>
          }
          <div class="figures">
            <label class="fig">
              <span class="eyebrow">grams</span>
              <span class="w-box big"><input type="number" inputmode="decimal" min="0.01" step="0.01" placeholder="0"
                [value]="l.grams ?? ''" (input)="number($event, l.id, 'grams')" /><span class="unit">g</span></span>
            </label>
            <label class="fig">
              <span class="eyebrow">a gram{{ l.inclVAT ? ', with VAT' : vat() ? ', before VAT' : '' }}</span>
              <span class="w-box big"><input type="number" inputmode="decimal" min="0" step="0.01" placeholder="0.00"
                [value]="l.perGram ?? ''" (input)="number($event, l.id, 'perGram')" /><span class="unit">€</span></span>
            </label>
            <span class="line num" [class.dim]="!cents(l)">{{ cents(l) ? euros(cents(l)) : '—' }}</span>
          </div>
          <div class="chips">
            @for (g of packsFor(l); track g) {
              <button type="button" class="chip" [class.on]="l.grams === g" (click)="edit(l.id, { grams: g, total: undefined })">{{ g }} g</button>
            }
            @if (lastPaid(l); as p) {
              <button type="button" class="chip paid" [class.on]="l.inclVAT && l.perGram === p / 100" (click)="paid(l.id, p)"
                [attr.aria-label]="'The price a gram paid last time, ' + euros(p) + ' with VAT'">
                <mat-icon>history</mat-icon>last time {{ euros(p) }}/g
              </button>
            }
          </div>
        </div>
      </div>
    }
    <button mat-stroked-button type="button" class="add" (click)="add()"><mat-icon>add</mat-icon>Add a strain</button>

    <w-choice label="the prices typed in" [options]="vatChoices" [value]="vat() ? 'add' : 'with'" (valueChange)="vat.set($event === 'add')" />
    @if (fromInvoice()) {
      <p class="dim note">A price marked with VAT — an invoice’s, or last time’s — has it already; it is added only to the others.</p>
    }
    <div class="receipt">
      <span class="eyebrow">the prescription</span>
      @if (refusal()) {
        <p class="refused"><mat-icon>report</mat-icon>Would be refused: {{ refusal() }}</p>
      }
      @if (totalGrams() > 0) {
        <div class="split" aria-hidden="true">
          @for (l of rows(); track l.id; let i = $index) {
            @if ((l.grams ?? 0) > 0) {
              <div [style.flex-grow]="l.grams" [style.background]="toneOf(i)" [title]="l.name"></div>
            }
          }
        </div>
      }
      @for (l of rows(); track l.id; let i = $index) {
        @if (l.name.trim() || (l.grams ?? 0) > 0) {
          <div class="row"><i [style.background]="toneOf(i)"></i><span class="what">{{ l.name.trim() || 'strain ' + (i + 1) }}</span>
            <span class="num">{{ l.grams ?? 0 | number: '1.0-2' }} g</span><span class="num">{{ cents(l) ? euros(cents(l)) : '—' }}</span></div>
        }
      }
      @if (vat() && typedCents()) {
        <div class="row sub"><i></i><span class="what">{{ fromInvoice() ? 'Typed in, before VAT' : 'Before VAT' }}</span><span></span>
          <span class="num">{{ euros(typedCents()) }}</span></div>
        <div class="row sub"><i></i><span class="what">VAT 19%</span><span></span><span class="num">{{ euros(vatCents()) }}</span></div>
      }
      <div class="row all"><i></i><span class="what">Together</span><span class="num">{{ totalGrams() | number: '1.0-2' }} g</span>
        <span class="num">{{ totalCents() ? euros(totalCents()) : 'no price' }}</span></div>
    </div>

    <w-when [value]="date()" (valueChange)="date.set($event)" />
    @if (invoiceDates().length > 1) {
      <p class="dim note">The invoices are dated {{ invoiceDates().join(', ') }}: a prescription is one pickup, on the day set here.</p>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; gap: 14px; }
    mat-icon { vertical-align: middle; }
    .eyebrow { font-size: 11px; }
    .dim { color: var(--w-dim); }
    .note { margin: -6px 0 0; font-size: 12px; }
    .readers { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
    .reader { display: grid; gap: 4px; justify-items: start; text-align: left; padding: 16px; border-radius: 16px; cursor: pointer;
      border: 1.5px dashed var(--w-glass-edge); background: color-mix(in srgb, var(--w-primary) 5%, transparent); color: inherit;
      font: inherit; transition: border-color .2s, background .2s; }
    .reader:hover:not(:disabled), .reader.over { border-color: var(--w-primary); background: color-mix(in srgb, var(--w-primary) 14%, transparent); }
    .reader:disabled { opacity: .6; cursor: progress; }
    .reader mat-icon { font-size: 28px; width: 28px; height: 28px; color: var(--w-primary); }
    .reader strong { font: 600 15px var(--w-head); }
    .reader .dim { font-size: 12px; line-height: 1.45; }
    .local { display: flex; align-items: center; gap: 6px; margin: -6px 0 0; font-size: 12px; }
    .local mat-icon { font-size: 15px; width: 15px; height: 15px; }
    .progress { position: relative; height: 22px; border-radius: 99px; overflow: hidden; background: color-mix(in srgb, var(--w-fg) 6%, transparent); }
    .progress div { position: absolute; inset: 0 auto 0 0; background: color-mix(in srgb, var(--w-primary) 40%, transparent); transition: width .3s; }
    .progress span { position: relative; display: block; text-align: center; font-size: 12px; line-height: 22px; }
    .read { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 10px; border: 1px solid var(--w-glass-edge);
      border-radius: 14px; padding: 12px 14px; background: color-mix(in srgb, var(--w-fg) 3%, transparent); }
    .read > mat-icon { color: var(--w-primary); }
    .read p { margin: 0 0 4px; }
    .known { color: var(--mat-sys-on-surface); }
    .choose { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 6px; }
    .read pre { white-space: pre-wrap; font: 12px var(--w-mono); max-height: 160px; overflow: auto; margin: 6px 0 0; }
    /* A strain is a card: its jar, filling as its grams are typed, beside its name, its figures and its chips. */
    .strain { display: grid; grid-template-columns: 52px minmax(0, 1fr); gap: 14px; align-items: start; padding: 14px;
      border-radius: 18px; border: 1px solid color-mix(in srgb, var(--tone) 35%, var(--w-glass-edge));
      background: linear-gradient(135deg, color-mix(in srgb, var(--tone) 8%, transparent), transparent 60%); }
    .glass { width: 52px; height: 73px; margin-top: 6px; }
    .body { display: grid; gap: 8px; min-width: 0; }
    .top { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 6px; align-items: start; }
    .name { display: grid; gap: 2px; min-width: 0; }
    .name input { width: 100%; min-width: 0; border: 0; border-bottom: 1px solid var(--w-glass-edge); background: none; color: inherit;
      font: 600 17px var(--w-head); padding: 4px 0 6px; outline: 0; }
    .name input:focus { border-bottom-color: var(--w-primary); }
    .name input::placeholder { color: color-mix(in srgb, var(--w-fg) 25%, transparent); font-weight: 400; }
    .kind { display: flex; align-items: center; gap: 4px; margin: -2px 0 0; font-size: 12px; color: var(--w-good); }
    .kind.new { color: var(--w-accent); }
    .kind mat-icon { font-size: 15px; width: 15px; height: 15px; }
    .figures { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; gap: 10px; align-items: end; }
    .fig { display: grid; gap: 2px; min-width: 0; }
    .big input[type='number'] { font-size: 22px; }
    .line { font-size: 17px; font-weight: 600; padding-bottom: 8px; text-align: right; min-width: 9ch; }
    .chips { display: flex; flex-wrap: wrap; gap: 6px; }
    .chip { display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; border-radius: 999px; border: 1px solid var(--w-glass-edge);
      background: none; color: inherit; font: 500 12px var(--w-num); cursor: pointer; }
    .chip:hover { border-color: var(--w-primary); }
    .chip.on { border-color: var(--w-primary); background: color-mix(in srgb, var(--w-primary) 18%, transparent); }
    .chip mat-icon { font-size: 14px; width: 14px; height: 14px; }
    .add { justify-self: start; }
    .receipt { display: grid; gap: 6px; padding: 14px 16px; border-radius: 4px 4px 14px 14px; border-top: 2px dashed var(--w-glass-edge);
      background: color-mix(in srgb, var(--w-fg) 4%, transparent); font-size: 13px; }
    .split { display: flex; gap: 2px; height: 8px; border-radius: 99px; overflow: hidden; margin: 2px 0 4px; }
    .split div { flex-basis: 0; transition: flex-grow .35s; }
    .receipt .row { display: grid; grid-template-columns: 10px minmax(0, 1fr) auto 10ch; gap: 10px; align-items: center; }
    .receipt i { width: 8px; height: 8px; border-radius: 2px; }
    .receipt .what { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .receipt .num { text-align: right; }
    .receipt .sub { color: var(--w-dim); font-size: 12px; }
    .refused { display: flex; gap: 8px; align-items: center; margin: 0; color: var(--w-warn); }
    .receipt .all { border-top: 1px solid var(--w-glass-edge); padding-top: 8px; margin-top: 2px; font-weight: 600; font-size: 15px; }
    @include bp.phone {
      .reader { padding: 12px; }
      .reader .dim { display: none; }
      .strain { grid-template-columns: 40px minmax(0, 1fr); gap: 10px; padding: 12px; }
      .glass { width: 40px; height: 56px; }
      .figures { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
      .line { grid-column: 1 / -1; padding: 0; }
    }
    @media (prefers-reduced-motion: reduce) { .reader, .progress div, .split div { transition: none; } }
  `,
})
export class FillForm {
  protected readonly wits = inject(Wits);

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
  /**
   * The invoices read, each with what the API made of it, and what became of
   * it: strains for this fill (new); a fill recorded already, the same day
   * (matched) or a few days off (ask); or that fill priced from it (priced).
   */
  protected readonly invoices = signal<Read[]>([]);
  /** Whether a fill already recorded is being priced from its invoice. */
  protected readonly pricing = signal(false);
  protected readonly fromInvoice = computed(() => this.rows().some((r) => r.inclVAT));
  protected readonly invoiceDates = computed(() =>
    [...new Set(this.invoices().flatMap((i) => (i.state === 'new' && i.reading.date ? [i.reading.date] : [])))].sort(),
  );
  /**
   * The fill as it would be recorded, rehearsed by the server as it is typed
   * (?dry=1): each whole strain priced as the recorder prices it — VAT added
   * where it is due, rounded once — so every figure on the form is the one
   * written down, and none is worked out here.
   */
  private readonly rehearsal = httpResource<Preview>(() => {
    const lines = this.rows().filter(whole).map(lineOf);
    this.wits.version();
    return lines.length ? { url: '/api/fill?dry=1', method: 'POST', body: { lines, vat: this.vat() } } : undefined;
  });
  /** Each whole strain's price and its VAT, in cents, by the strain. */
  private readonly priced = computed(() => {
    const out = new Map<number, { cents: number; vat: number }>();
    const lines = this.rehearsal.hasValue() ? (this.rehearsal.value().lines ?? []) : [];
    this.rows().filter(whole).forEach((r, i) => {
      const l = lines.at(i);
      if (l) {
        out.set(r.id, { cents: l.cents, vat: l.vat_cents });
      }
    });
    return out;
  });
  /** Why the fill would be refused, as the recorder says it. */
  protected readonly refusal = computed(() => {
    const err = this.rehearsal.error() as HttpErrorResponse | undefined;
    return (err?.error as { error?: string } | null)?.error ?? '';
  });
  protected readonly totalCents = computed(() => [...this.priced().values()].reduce((sum, p) => sum + p.cents, 0));
  protected readonly vatCents = computed(() => [...this.priced().values()].reduce((sum, p) => sum + p.vat, 0));
  /** The prices the VAT was added to, before it. */
  protected readonly typedCents = computed(() =>
    [...this.priced().values()].reduce((sum, p) => sum + (p.vat ? p.cents - p.vat : 0), 0));
  protected readonly totalGrams = computed(() => this.rows().reduce((sum, r) => sum + (r.grams ?? 0), 0));

  protected readonly vatChoices: Choice[] = [
    { value: 'add', label: 'are before VAT', icon: 'add_circle', hint: '19% added' },
    { value: 'with', label: 'include VAT', icon: 'check_circle' },
  ];
  /** Whether invoices are being dragged over their reader. */
  protected readonly over = signal(false);
  /** The jars on the shelf by their names, to tell a jar already known from a new product. */
  private readonly known = computed(() => new Map(this.wits.jars.value().map((j) => [j.name.toLowerCase(), j.slug])));
  /** Every purchase that stands, newest first. */
  private readonly purchases = computed(() =>
    this.wits.journal.value().entries.filter((e) => e.type === 'purchase' && !e.corrected));
  /** The sizes bought most, four of them, smallest first: what a pharmacy packs. */
  private readonly packs = computed(() => sizes(this.purchases().map((e) => e.grams)));
  /** Every strain's jar, drawn against the largest, and 20 g at least, so a 5 g jar reads as one. */
  protected readonly capacity = computed(() => Math.max(20, ...this.rows().map((r) => r.grams ?? 0)));

  protected toneOf(i: number): string {
    return tone(i);
  }

  /** The jar a strain already is, by its name. */
  protected jarOf(r: Row): string | undefined {
    return this.known().get(r.name.trim().toLowerCase());
  }

  /** The sizes this jar was bought in, or the ones bought most when it is new. */
  protected packsFor(r: Row): number[] {
    const slug = this.jarOf(r);
    const own = slug ? sizes(this.purchases().filter((e) => e.product === slug).map((e) => e.grams)) : [];
    return own.length ? own : this.packs();
  }

  /** What a gram of this jar cost the last time, in cents with VAT. */
  protected lastPaid(r: Row): number | undefined {
    const slug = this.jarOf(r);
    return slug ? this.wits.costs.value()?.bought.filter((b) => b.product === slug).at(-1)?.per_gram : undefined;
  }

  /** The price a gram paid last time, which has its VAT in it already. */
  protected paid(id: number, cents: number): void {
    this.edit(id, { perGram: cents / 100, inclVAT: true, total: undefined });
  }

  protected text(e: Event): string {
    return (e.target as HTMLInputElement).value;
  }

  /** A figure typed; half a number — "1." on its way to "1.5" — keeps what was there. */
  protected number(e: Event, id: number, field: 'grams' | 'perGram'): void {
    const el = e.target as HTMLInputElement;
    if (el.validity.badInput) {
      return;
    }
    const v = el.value === '' ? null : Number(el.value);
    this.edit(id, field === 'grams' ? { grams: v, total: undefined } : { perGram: v, total: undefined });
  }

  protected drag(e: DragEvent, over: boolean): void {
    e.preventDefault();
    this.over.set(over);
  }

  /** Invoices dropped on their reader are read like those picked; anything not a PDF is passed over. */
  protected async dropped(e: DragEvent): Promise<void> {
    e.preventDefault();
    this.over.set(false);
    const files = [...(e.dataTransfer?.files ?? [])].filter((f) => f.type === 'application/pdf' || f.name.toLowerCase().endsWith('.pdf'));
    await this.readFiles(files);
  }

  protected cents(r: Row): number {
    return this.priced().get(r.id)?.cents ?? 0;
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
   * The fill as the API takes it: a strain off an invoice with its total,
   * one typed in with its price a gram, said to have its VAT or not, and the
   * server pricing both. The blank strains the form starts with go.
   */
  request(): { lines: FillLine[]; vat: boolean; seller?: string } {
    const lines = this.rows().filter(started).map(lineOf);
    // The pharmacy, when the invoices read for this fill name one and the same.
    const sellers = new Set(this.invoices().filter((i) => i.state === 'new').map((i) => i.reading.seller ?? ''));
    const seller = sellers.size === 1 ? [...sellers][0] : '';
    return { lines, vat: this.vat(), seller: seller || undefined };
  }

  /** Reads the invoices picked, one after the other, and adds their strains to the fill. */
  protected async readInvoices(input: HTMLInputElement): Promise<void> {
    const files = [...(input.files ?? [])];
    input.value = '';
    await this.readFiles(files);
  }

  /** Reads invoices, one after the other, and adds their strains to the fill. */
  private async readFiles(files: File[]): Promise<void> {
    if (!files.length) {
      return;
    }
    this.error.set('');
    this.reading.set(0);
    const failed: string[] = [];
    for (const [i, file] of files.entries()) {
      try {
        const reading = await this.wits.invoice(await readInvoice(file));
        // A fill recorded already is priced from its invoice, not recorded again.
        const state = !reading.match ? 'new' : reading.match.days === 0 ? 'matched' : 'ask';
        this.invoices.update((all) => [...all, { file: file.name, reading, state }]);
        if (state === 'new') {
          this.addRows(reading);
        }
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

  /** An invoice's strains, added to the fill as rows. */
  private addRows(reading: InvoiceReading): void {
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
  }

  protected readonly addsUp = addsUp;

  protected day(iso: string): string {
    return new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'long' });
  }

  /** Prices the fill an invoice is for from it, each strain at the invoice's total for it. */
  protected async priceMatched(i: number): Promise<void> {
    const inv = this.invoices()[i];
    const m = inv.reading.match;
    if (!m) {
      return;
    }
    this.pricing.set(true);
    this.error.set('');
    try {
      const lines = inv.reading.lines.flatMap((l, n) => (m.lines[n]
        ? [{ purchase: m.lines[n], total: (l.total_cents / 100).toFixed(2), note: inv.reading.number ? `invoice ${inv.reading.number}` : undefined }]
        : []));
      await this.wits.priceFill(lines, false, inv.reading.seller ?? '');
      this.setState(i, 'priced');
    } catch (e) {
      this.error.set(e instanceof Error ? e.message : String(e));
    } finally {
      this.pricing.set(false);
    }
  }

  /** Not the fill it matched: its strains go on this one after all. */
  protected asNew(i: number): void {
    this.addRows(this.invoices()[i].reading);
    this.setState(i, 'new');
    const latest = this.invoiceDates().at(-1);
    if (latest) {
      this.date.set(new Date(`${latest}T12:00`));
    }
  }

  private setState(i: number, state: Read['state']): void {
    this.invoices.update((all) => all.map((inv, n) => (n === i ? { ...inv, state } : inv)));
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
