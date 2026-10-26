import { CurrencyPipe, DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, model, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';

import { leftOut, readInvoice } from '../data/invoice';
import { InvoiceReading, PriceLine } from '../data/models';
import { Wits } from '../data/wits';

/** One invoice of a batch: the file, what it says, and whether its fill is to be priced from it. */
export interface Read {
  file: File;
  reading?: InvoiceReading;
  error?: string;
  ticked: boolean;
}

/** How many of an invoice's lines its match found a purchase for. */
export function found(r: Read): number {
  return r.reading?.match?.lines.filter((h) => h).length ?? 0;
}

/**
 * Whether an invoice's fill is ticked to begin with: it matched a fill, with
 * a purchase for at least one line, and not every one of those has its price.
 */
export function tickable(r: Read): boolean {
  const m = r.reading?.match;
  return !!m && found(r) > 0 && m.priced < found(r);
}

/** The lines a fill is priced at from its invoice: each matched line at the invoice's total for it. */
export function invoiceLines(reading: InvoiceReading): PriceLine[] {
  const m = reading.match;
  if (!m) {
    return [];
  }
  const note = reading.number ? `invoice ${reading.number}` : undefined;
  return reading.lines.flatMap((l, i) => (m.lines[i] ? [{ purchase: m.lines[i], total: (l.total_cents / 100).toFixed(2), note }] : []));
}

/** Newest invoice first, as the pickups are; one that was not read, last. */
export function newestFirst(a: Read, b: Read): number {
  return (b.reading?.date ?? '').localeCompare(a.reading?.date ?? '');
}

/**
 * Many invoices at once — picked together, or a whole folder — read on this
 * device one after the other, each set against the fill it is for. What
 * matched is ticked to be priced, the days between invoice and pickup said
 * so they can be checked; what is already priced, or matches no fill, is
 * not. One can be reviewed on its pickup's card, its first page beside it.
 */
@Component({
  selector: 'w-invoice-batch',
  imports: [CurrencyPipe, DatePipe, MatButtonModule, MatCheckboxModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="pick">
      <button mat-stroked-button type="button" (click)="many.click()" [disabled]="reading() !== null">
        <mat-icon>library_add</mat-icon>Add invoices
      </button>
      <input #many type="file" accept="application/pdf,.pdf" multiple hidden (change)="read(many)" />
      <button mat-stroked-button type="button" (click)="folder.click()" [disabled]="reading() !== null">
        <mat-icon>folder_open</mat-icon>Pick a folder
      </button>
      <input #folder type="file" webkitdirectory multiple hidden (change)="read(folder)" />
      @if (reading(); as r) {
        <span class="dim num">reading {{ r.done }} of {{ r.of }}…</span>
      } @else if (!reads().length) {
        <span class="dim">Every PDF at once; each is read on this device and set against its pickup.</span>
      }
    </div>

    @if (reads().length) {
      <div class="rows">
        @for (r of reads(); track $index; let i = $index) {
          <div class="row" [class.off]="!r.reading?.match">
            <mat-checkbox [checked]="r.ticked" [disabled]="!r.reading?.match || !found(r)" (change)="tick(i, $event.checked)"
              [aria-label]="r.file.name" />
            <div class="what">
              <strong>{{ r.reading?.date ? (r.reading?.date | date: 'd MMM y') : r.file.name }}</strong>
              <span class="dim">{{ r.reading?.number ?? r.file.name }}</span>
            </div>
            <div class="to">
              @if (r.error) {
                <span class="bad">Not read — {{ r.error }}</span>
              } @else if (r.reading?.match; as m) {
                <span>Prescription {{ m.fill }}, picked up {{ m.start | date: 'd MMM y' }}</span>
                <span class="dim">
                  @if (m.days) { <span class="far">{{ m.days }} {{ m.days === 1 ? 'day' : 'days' }} apart</span> · } @else { the same day · }
                  {{ found(r) }} of {{ r.reading?.lines?.length }} {{ r.reading?.lines?.length === 1 ? 'strain' : 'strains' }}
                  @if (m.priced) { · {{ m.priced === found(r) ? 'already priced' : m.priced + ' already priced' }} }
                  @if (twice().has(m.fill)) { · one of {{ twice().get(m.fill) }} invoices for it }
                </span>
              } @else {
                <span class="dim">No prescription within two weeks holds its strains</span>
              }
            </div>
            <div class="sum">
              <span class="num">{{ (r.reading?.sum_cents ?? 0) / 100 | currency: 'EUR' }}</span>
              @if (r.reading && left(r.reading)) {
                <span class="dim small">+ {{ left(r.reading) / 100 | currency: 'EUR' }} left out</span>
              }
            </div>
            <button mat-icon-button type="button" [disabled]="!r.reading?.match" (click)="review.emit(r)"
              [attr.aria-label]="'Review ' + r.file.name + ' on its pickup'"><mat-icon>visibility</mat-icon></button>
          </div>
        }
      </div>
      <div class="go">
        <span class="dim">{{ ticked().length }} of {{ reads().length }} ticked
          @if (far()) { · <span class="far">{{ far() }} dated some days off their pickup — check them</span> }</span>
        <button mat-button type="button" (click)="reads.set([])">Clear the list</button>
        <button mat-flat-button type="button" [disabled]="!ticked().length || busy()" (click)="price()">
          Price {{ fills() }} {{ fills() === 1 ? 'prescription' : 'prescriptions' }} from {{ ticked().length }} {{ ticked().length === 1 ? 'invoice' : 'invoices' }}
        </button>
      </div>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; gap: 12px; }
    .pick { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
    .pick .dim { font-size: 13px; }
    .rows { display: grid; max-height: 460px; overflow: auto; }
    .row { display: grid; grid-template-columns: auto minmax(110px, 0.6fr) minmax(0, 1.4fr) auto auto; gap: 12px; align-items: center;
      padding: 6px 0; border-top: 1px solid var(--w-glass-edge); }
    .row.off { opacity: 0.7; }
    .what, .to, .sum { display: grid; min-width: 0; }
    .sum { text-align: right; }
    .dim { color: var(--w-dim); }
    .small { font-size: 11px; }
    .far { color: var(--w-avb); }
    .bad { color: var(--w-warn); }
    .go { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; justify-content: flex-end; }
    .go > .dim { margin-right: auto; font-size: 13px; }
    @include bp.phone {
      .row { grid-template-columns: auto minmax(0, 1fr) auto; }
      .what { grid-column: 2; }
      .to { grid-column: 2 / -1; grid-row: 2; }
      .sum { grid-column: 3; grid-row: 1; }
      .row > button { display: none; }
    }
  `,
})
export class InvoiceBatch {
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  protected readonly found = found;
  protected readonly left = leftOut;

  /** Called when fills were priced from the batch, so the walk can read what is left. */
  readonly priced = output();
  /** An invoice to look at on its pickup's card. */
  readonly review = output<Read>();
  /** The batch, kept by the walk so it outlives a card. */
  readonly reads = model<Read[]>([]);

  protected readonly reading = signal<{ done: number; of: number } | null>(null);
  protected readonly busy = signal(false);
  protected readonly ticked = computed(() => this.reads().filter((r) => r.ticked));
  protected readonly fills = computed(() => new Set(this.ticked().map((r) => r.reading?.match?.fill)).size);
  protected readonly far = computed(() => this.ticked().filter((r) => (r.reading?.match?.days ?? 0) > 0).length);
  /** Fills that more than one invoice in the batch is for, with how many. */
  protected readonly twice = computed(() => {
    const n = new Map<number, number>();
    for (const r of this.reads()) {
      const f = r.reading?.match?.fill;
      if (f) {
        n.set(f, (n.get(f) ?? 0) + 1);
      }
    }
    return new Map([...n].filter(([, c]) => c > 1));
  });

  protected tick(i: number, on: boolean): void {
    this.reads.update((all) => all.map((r, n) => (n === i ? { ...r, ticked: on } : r)));
  }

  /** Reads the PDFs picked, or every PDF in the folder picked, one after the other. */
  protected async read(input: HTMLInputElement): Promise<void> {
    const files = [...(input.files ?? [])].filter((f) => /\.pdf$/i.test(f.name) || f.type === 'application/pdf');
    input.value = '';
    if (!files.length) {
      return;
    }
    const out: Read[] = [];
    this.reading.set({ done: 0, of: files.length });
    for (const [i, file] of files.entries()) {
      try {
        const reading = await this.wits.invoice(await readInvoice(file));
        const r: Read = { file, reading, ticked: false };
        out.push({ ...r, ticked: tickable(r) });
      } catch (e) {
        out.push({ file, error: e instanceof Error ? e.message : String(e), ticked: false });
      }
      this.reading.set({ done: i + 1, of: files.length });
    }
    this.reads.set(out.sort(newestFirst));
    this.reading.set(null);
  }

  /** Prices every ticked invoice's fill from it, oldest first, so a later invoice for one fill stands over an earlier. */
  protected async price(): Promise<void> {
    this.busy.set(true);
    let done = 0;
    const failed: string[] = [];
    const ok = new Set<Read>();
    for (const r of [...this.ticked()].reverse()) {
      const reading = r.reading;
      if (!reading) {
        continue;
      }
      try {
        await this.wits.priceFill(invoiceLines(reading), false, reading.seller ?? '');
        ok.add(r);
        done++;
      } catch (e) {
        failed.push(`${r.file.name}: ${e instanceof Error ? e.message : String(e)}`);
      }
    }
    this.busy.set(false);
    // What was priced says so, and is ticked no more; what failed stays ticked to try again.
    this.reads.update((all) => all.map((r) => (ok.has(r) && r.reading?.match
      ? { ...r, ticked: false, reading: { ...r.reading, match: { ...r.reading.match, priced: found(r) } } }
      : r)));
    this.snack.open(
      `${done} ${done === 1 ? 'invoice' : 'invoices'} priced${failed.length ? `; not priced — ${failed.join('; ')}` : ''}`,
      failed.length ? 'OK' : undefined,
      { duration: failed.length ? 8000 : 4000 },
    );
    this.priced.emit();
  }
}
