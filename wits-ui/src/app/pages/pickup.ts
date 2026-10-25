import { CurrencyPipe, DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnDestroy, OnInit, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';

import { addsUp, pictureOf, readInvoice } from '../data/invoice';
import { InvoiceReading, PriceLine, UnpricedEntry } from '../data/models';
import { Wits } from '../data/wits';
import { Row, VAT, rowCents } from './fill-form';

/** One pickup's purchases without a price: the strains of one prescription. */
export interface Pickup {
  fill: number;
  start: string;
  grams: number;
  lines: UnpricedEntry[];
}

/** What a purchase is priced at on the card: a price a gram typed, or an invoice's line, which includes VAT. */
export interface Priced {
  perGram: number | null;
  /** The invoice's line total, in cents, until the price a gram is changed. */
  total?: number;
  inclVAT?: boolean;
  note?: string;
}

/** An invoice read on the device: what it says, and its first page as a picture beside the figures. */
export interface Scan {
  file: string;
  picture: string;
  pdf: string;
  reading: InvoiceReading;
}

/** Groups the purchases without a price by the fill they came in, newest fill first. */
export function byFill(entries: UnpricedEntry[]): Pickup[] {
  const out = new Map<number, Pickup>();
  for (const e of entries) {
    const p = out.get(e.fill) ?? { fill: e.fill, start: e.fill_start, grams: 0, lines: [] };
    p.lines.push(e);
    p.grams = Math.round((p.grams + e.grams) * 100) / 100;
    out.set(e.fill, p);
  }
  return [...out.values()].sort((a, b) => b.fill - a.fill);
}

function row(l: UnpricedEntry, p: Priced | undefined): Row {
  return { id: 0, name: '', grams: l.grams, perGram: p?.perGram ?? null, total: p?.total, inclVAT: p?.inclVAT };
}

/**
 * A fill's prices in cents: every line by rowCents, as the fill form and the
 * server (lineCents) work it, and the fill's total — `typed` the prices typed
 * in before VAT, `vat` what is added to them, the invoice's lines as printed.
 */
export function fillCents(lines: UnpricedEntry[], priced: Partial<Record<string, Priced>>, vat: boolean):
  { lines: Record<string, number>; typed: number; vat: number; invoice: number; total: number } {
  const out = { lines: {} as Record<string, number>, typed: 0, vat: 0, invoice: 0, total: 0 };
  for (const l of lines) {
    const r = row(l, priced[l.hash]);
    const cents = rowCents(r, vat);
    out.lines[l.hash] = cents;
    out.total += cents;
    if (r.inclVAT) {
      out.invoice += cents;
    } else {
      out.typed += rowCents(r, false);
      out.vat += cents - rowCents(r, false);
    }
  }
  return out;
}

/**
 * The fill as the API takes it: an invoice's line by its total, one typed by
 * its price a gram, and — when the fill has no price to be had — every line
 * left without one marked so, to be asked about no more.
 */
export function priceLines(lines: UnpricedEntry[], priced: Partial<Record<string, Priced>>, vat: boolean, giveUp = false): PriceLine[] {
  return lines.map((l) => {
    const p = priced[l.hash];
    if (p?.inclVAT && p.perGram) {
      return { purchase: l.hash, total: (rowCents(row(l, p), vat) / 100).toFixed(2), note: p.note };
    }
    if (p?.perGram) {
      return { purchase: l.hash, per_gram: String(p.perGram) };
    }
    return giveUp ? { purchase: l.hash, unknown: true } : { purchase: l.hash, per_gram: '' };
  });
}

/**
 * The prices an invoice gives a fill's purchases, by the match the server
 * found or, without one, by product; and what on it fits none of them.
 */
export function fromInvoice(f: Pickup, reading: InvoiceReading, byMatch: boolean): { priced: Partial<Record<string, Priced>>; notes: string[] } {
  const priced: Partial<Record<string, Priced>> = {};
  const notes: string[] = [];
  const ours = new Map(f.lines.map((l) => [l.hash, l]));
  const m = byMatch && reading.match?.fill === f.fill ? reading.match : undefined;
  reading.lines.forEach((line, i) => {
    const hash = m ? m.lines[i] : f.lines.find((l) => l.product === line.slug)?.hash;
    const entry = hash ? ours.get(hash) : undefined;
    if (!entry) {
      notes.push(`${line.printed} is not in this fill`);
      return;
    }
    if (Math.abs(entry.grams - line.grams) > 0.005) {
      notes.push(`the invoice says ${line.grams} g of ${line.name}, the journal ${entry.grams} g; its total is written down as printed`);
    }
    priced[entry.hash] = { perGram: line.per_gram_cents / 100, total: line.total_cents, inclVAT: true,
      note: reading.number ? `invoice ${reading.number}` : undefined };
  });
  return { priced, notes };
}

/**
 * One pickup on Costs, priced together: a price a gram for each strain, VAT
 * on top when it is due, or the prices read off its invoice — on this device,
 * the invoice's first page shown beside them. An invoice dated a few days off
 * the pickup is asked about; one that is another fill's says whose it is.
 */
@Component({
  selector: 'w-pickup',
  imports: [CurrencyPipe, DatePipe, DecimalPipe, FormsModule, MatButtonModule, MatCheckboxModule, MatFormFieldModule, MatIconModule,
    MatInputModule, MatSlideToggleModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @let f = pickup();
    <div class="split" [class.with]="applied()">
      <div class="pickup">
        <div class="head">
          <span class="count num">{{ position() }}</span>
          <div class="what">
            <strong>Fill {{ f.fill }}</strong>
            <span class="dim">picked up {{ f.start | date: 'd MMMM y' }} · {{ f.lines.length }} {{ f.lines.length === 1 ? 'strain' : 'strains' }}, {{ f.grams | number: '1.2-2' }} g</span>
          </div>
          <button mat-stroked-button type="button" class="scan" (click)="pdf.click()" [disabled]="reading()">
            <mat-icon>receipt_long</mat-icon>{{ reading() ? 'Reading…' : 'Scan invoice' }}
          </button>
          <input #pdf type="file" accept="application/pdf,.pdf" hidden (change)="read(pdf)" />
        </div>

        @if (scan(); as s) {
          @switch (ask()) {
            @case ('confirm') {
              <div class="ask" role="status">
                <mat-icon>help</mat-icon>
                <p>The invoice{{ s.reading.number ? ' ' + s.reading.number : '' }} is dated {{ s.reading.date | date: 'd MMMM' }};
                  Fill {{ f.fill }} was picked up {{ f.start | date: 'd MMMM' }},
                  {{ s.reading.match?.days }} {{ s.reading.match?.days === 1 ? 'day' : 'days' }} apart. Is it this fill's invoice?</p>
                <div class="choose">
                  <button mat-flat-button type="button" (click)="apply(true)">Yes, take its prices</button>
                  <button mat-button type="button" (click)="dismiss()">No</button>
                </div>
              </div>
            }
            @case ('other') {
              <div class="ask" role="status">
                <mat-icon>swap_horiz</mat-icon>
                <p>This invoice is Fill {{ s.reading.match?.fill }}'s, picked up {{ s.reading.match?.start | date: 'd MMMM y' }}{{
                  s.reading.match?.priced ? ', which already has its prices' : '' }}.</p>
                <div class="choose">
                  @if (fills().includes(s.reading.match?.fill ?? 0)) {
                    <button mat-flat-button type="button" (click)="handOver()">Go to Fill {{ s.reading.match?.fill }}</button>
                  }
                  <button mat-button type="button" (click)="apply(false)">Use it here anyway</button>
                  <button mat-button type="button" (click)="dismiss()">Dismiss</button>
                </div>
              </div>
            }
            @case ('none') {
              <div class="ask" role="status">
                <mat-icon>help</mat-icon>
                <p>No fill within two weeks of {{ s.reading.date ? (s.reading.date | date: 'd MMMM y') : 'the invoice' }} holds its strains.</p>
                <div class="choose">
                  <button mat-button type="button" (click)="apply(false)">Use it here anyway</button>
                  <button mat-button type="button" (click)="dismiss()">Dismiss</button>
                </div>
              </div>
            }
          }
        }

        <div class="lines">
          @for (l of f.lines; track l.hash) {
            <div class="line">
              <div class="strain">
                <strong>{{ l.product_name }}</strong>
                <span class="dim num">{{ l.grams | number: '1.2-2' }} g{{ priced()[l.hash]?.inclVAT ? ' · from the invoice, with VAT' : '' }}</span>
              </div>
              <mat-form-field appearance="outline" class="rate" subscriptSizing="dynamic">
                <mat-label>A gram</mat-label>
                <input matInput type="number" min="0" step="0.01" inputmode="decimal" [ngModel]="priced()[l.hash]?.perGram ?? null"
                  (ngModelChange)="rate(l.hash, $event)" (keydown.enter)="save()" />
                <span matTextSuffix>€</span>
              </mat-form-field>
              <span class="total num">{{ sums().lines[l.hash] ? (sums().lines[l.hash] / 100 | currency: 'EUR') : '—' }}</span>
            </div>
          }
        </div>
        @for (n of notes(); track n) {
          <p class="note dim"><mat-icon>info</mat-icon>{{ n }}</p>
        }

        <div class="foot">
          <div class="vat">
            <mat-slide-toggle [checked]="vat()" (change)="vat.set($event.checked)">Add {{ vatRate }}% VAT (Germany)</mat-slide-toggle>
            @if (sums().invoice) {
              <span class="dim">The invoice’s prices include VAT already; it is added only to prices typed in.</span>
            }
          </div>
          <dl>
            @if (vat() && sums().typed) {
              <div><dt>Typed in, before VAT</dt><dd class="num">{{ sums().typed / 100 | currency: 'EUR' }}</dd></div>
              <div><dt>VAT {{ vatRate }}%</dt><dd class="num">{{ sums().vat / 100 | currency: 'EUR' }}</dd></div>
            }
            @if (sums().invoice) {
              <div><dt>From the invoice, with VAT</dt><dd class="num">{{ sums().invoice / 100 | currency: 'EUR' }}</dd></div>
            }
            <div class="sum"><dt>The fill</dt><dd class="num">{{ sums().total / 100 | currency: 'EUR' }}</dd></div>
          </dl>
        </div>
        @if (error()) {
          <p class="bad">{{ error() }}</p>
        }
        <div class="actions">
          <mat-checkbox class="give-up" [checked]="giveUp()" (change)="giveUp.set($event.checked)">
            No price to be had{{ sums().total ? ' for the rest' : '' }} — stop asking
          </mat-checkbox>
          <button mat-flat-button type="button" [disabled]="busy() || (!sums().total && !giveUp())" (click)="save()">
            {{ giveUp() && !sums().total ? 'Mark as unpriced' : 'Save the fill' }}
          </button>
          <button mat-button type="button" (click)="skipped.emit()">Skip</button>
        </div>
      </div>

      @if (applied() && scan(); as s) {
        <figure class="invoice">
          <a [href]="s.pdf" target="_blank" rel="noopener" aria-label="Open the invoice">
            <img [src]="s.picture" alt="The invoice's first page" />
          </a>
          <figcaption class="dim">
            {{ s.file }}{{ s.reading.date ? ' · ' + (s.reading.date | date: 'd MMM y') : '' }} ·
            {{ s.reading.sum_cents / 100 | currency: 'EUR' }} of flower
            @if (!addsUp(s.reading)) {
              — but it says {{ s.reading.gross_cents / 100 | currency: 'EUR' }} in all. Check the lines.
            }
            @for (o of s.reading.other; track $index) {
              <br />Left out, not flower by the gram: {{ o.name }}, {{ o.total_cents / 100 | currency: 'EUR' }}.
            }
          </figcaption>
        </figure>
      }
    </div>
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: block; }
    .split { display: grid; grid-template-columns: minmax(0, 1fr); gap: 22px; }
    .split.with { grid-template-columns: minmax(0, 1fr) minmax(0, 420px); }
    .pickup { display: grid; gap: 14px; align-content: start; }
    .head { display: flex; flex-wrap: wrap; gap: 10px 14px; align-items: center; }
    .what { display: grid; flex: 1 1 200px; }
    .count { color: var(--w-dim); min-width: 64px; }
    .ask { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 6px 12px; align-items: start; padding: 12px 14px;
      border-radius: var(--w-radius-dialog); background: color-mix(in srgb, var(--w-primary) 10%, transparent);
      border: 1px solid color-mix(in srgb, var(--w-primary) 30%, transparent); }
    .ask p { margin: 0; }
    .ask mat-icon { color: var(--w-primary); }
    .choose { grid-column: 2; display: flex; flex-wrap: wrap; gap: 8px; }
    .lines { display: grid; }
    .line { display: grid; grid-template-columns: minmax(0, 1fr) 150px 110px; gap: 14px; align-items: center;
      padding: 10px 0; border-top: 1px solid var(--w-glass-edge); }
    .strain { display: grid; }
    .total { text-align: right; }
    .note { display: flex; gap: 8px; align-items: center; margin: 0; font-size: 13px; }
    .note mat-icon { font-size: 18px; width: 18px; height: 18px; }
    .foot { display: flex; flex-wrap: wrap; gap: 10px 24px; justify-content: space-between; align-items: flex-start;
      padding-top: 12px; border-top: 1px solid var(--w-glass-edge); }
    .vat { display: grid; gap: 4px; max-width: 320px; }
    .vat .dim { font-size: 12px; }
    dl { margin: 0; display: grid; gap: 4px; min-width: 240px; }
    dl div { display: flex; justify-content: space-between; gap: 18px; }
    dt { color: var(--w-dim); }
    dd { margin: 0; }
    .sum { font-weight: 600; font-size: 16px; }
    .sum dt { color: inherit; }
    .bad { color: var(--w-warn); margin: 0; }
    .actions { display: flex; flex-wrap: wrap; gap: 10px; justify-content: flex-end; align-items: center; }
    .give-up { margin-right: auto; }
    .invoice { margin: 0; display: grid; gap: 8px; align-content: start; }
    .invoice img { display: block; width: 100%; border-radius: var(--w-radius-dialog); border: 1px solid var(--w-glass-edge);
      background: #fff; }
    .invoice figcaption { font-size: 12px; }
    @include bp.phone {
      .split.with { grid-template-columns: minmax(0, 1fr); }
      .line { grid-template-columns: minmax(0, 1fr) auto; }
      .strain { grid-column: 1 / -1; }
      .rate { width: 150px; }
      dl { min-width: 0; width: 100%; }
      .scan { width: 100%; }
    }
  `,
})
export class PickupCard implements OnInit, OnDestroy {
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  protected readonly vatRate = VAT;
  protected readonly addsUp = addsUp;

  readonly pickup = input.required<Pickup>();
  /** Where the walk is, as "1 of 2". */
  readonly position = input('');
  /** The fills in the walk, to offer going to the one an invoice is for. */
  readonly fills = input<number[]>([]);
  /** An invoice read on another fill's card that is this fill's. */
  readonly handed = input<Scan | null>(null);
  readonly saved = output();
  readonly skipped = output();
  /** An invoice read here that is another fill's, handed to that fill's card. */
  readonly jump = output<{ fill: number; scan: Scan }>();

  protected readonly priced = signal<Partial<Record<string, Priced>>>({});
  /** Whether VAT is added to the prices typed in. On: a German pharmacy quotes them before it. */
  protected readonly vat = signal(true);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly reading = signal(false);
  protected readonly scan = signal<Scan | null>(null);
  protected readonly ask = signal<'confirm' | 'other' | 'none' | null>(null);
  protected readonly applied = signal(false);
  protected readonly notes = signal<string[]>([]);
  /** Whether what is left without a price has none to be had, and is asked about no more. */
  protected readonly giveUp = signal(false);
  protected readonly sums = computed(() => fillCents(this.pickup().lines, this.priced(), this.vat()));
  private handedOver = false;

  ngOnInit(): void {
    const scan = this.handed();
    if (scan) {
      this.scan.set(scan);
      this.apply(true);
    }
  }

  ngOnDestroy(): void {
    if (!this.handedOver) {
      this.release(this.scan());
    }
  }

  protected rate(hash: string, perGram: number | null): void {
    // A price changed by hand is no longer the invoice's total; it keeps the invoice's VAT.
    this.priced.update((all) => ({ ...all, [hash]: { ...all[hash], perGram, total: undefined } }));
  }

  /** Reads the invoice picked, and fills the prices from it, asking first when its date is a few days off. */
  protected async read(input: HTMLInputElement): Promise<void> {
    const file = input.files?.[0];
    input.value = '';
    if (!file) {
      return;
    }
    this.reading.set(true);
    this.error.set('');
    try {
      const [text, picture] = await Promise.all([readInvoice(file), pictureOf(file)]);
      const reading = await this.wits.invoice(text);
      this.release(this.scan());
      this.scan.set({ file: file.name, picture, pdf: URL.createObjectURL(file), reading });
      this.applied.set(false);
      const m = reading.match;
      if (m?.fill === this.pickup().fill) {
        if (m.days === 0) {
          this.apply(true);
        } else {
          this.ask.set('confirm');
        }
      } else {
        this.ask.set(m ? 'other' : 'none');
      }
    } catch (e) {
      this.error.set(`Not read — ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      this.reading.set(false);
    }
  }

  /** Takes the invoice's prices: by the server's match, or by product when it matched another fill or none. */
  protected apply(byMatch: boolean): void {
    const s = this.scan();
    if (!s) {
      return;
    }
    const { priced, notes } = fromInvoice(this.pickup(), s.reading, byMatch);
    this.priced.update((all) => ({ ...all, ...priced }));
    this.notes.set(Object.keys(priced).length ? notes : ['none of the invoice’s strains are in this fill', ...notes]);
    this.applied.set(Object.keys(priced).length > 0);
    this.ask.set(null);
  }

  protected handOver(): void {
    const s = this.scan();
    const fill = s?.reading.match?.fill;
    if (s && fill) {
      this.handedOver = true;
      this.jump.emit({ fill, scan: s });
    }
  }

  protected dismiss(): void {
    this.release(this.scan());
    this.scan.set(null);
    this.ask.set(null);
    this.applied.set(false);
  }

  protected async save(): Promise<void> {
    if (this.busy() || (!this.sums().total && !this.giveUp())) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const seller = this.applied() ? (this.scan()?.reading.seller ?? '') : '';
      const out = await this.wits.priceFill(priceLines(this.pickup().lines, this.priced(), this.vat(), this.giveUp()), this.vat(), seller);
      this.snack.open(out.message, undefined, { duration: 3000 });
      this.saved.emit();
    } catch (e) {
      this.error.set(e instanceof Error ? e.message : String(e));
    } finally {
      this.busy.set(false);
    }
  }

  private release(s: Scan | null): void {
    if (s) {
      URL.revokeObjectURL(s.picture);
      URL.revokeObjectURL(s.pdf);
    }
  }
}
