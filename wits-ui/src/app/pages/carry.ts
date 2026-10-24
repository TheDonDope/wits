import { DecimalPipe } from '@angular/common';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialogModule } from '@angular/material/dialog';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
import { firstValueFrom } from 'rxjs';

/** A signed statement of what is held, as the API makes it. */
interface Carried {
  statement: string;
  url: string;
  fingerprint: string;
  qr: string;
  items: { name: string; sealed: number; ground: number; avb: number; fills: { day: string; grams: number }[] }[];
}

/**
 * What is carried, signed: the holdings against the fills that dispensed them,
 * and a QR code that opens a page checking the signature in the browser. The
 * statement rides after the #, which a browser never sends to a server.
 */
@Component({
  selector: 'w-carry',
  imports: [DecimalPipe, MatButtonModule, MatDialogModule, MatSlideToggleModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h2 mat-dialog-title>What I carry</h2>
    <mat-dialog-content>
      @if (error()) {
        <p class="bad">{{ error() }}</p>
      } @else if (carried(); as c) {
        <div class="qr" [innerHTML]="qr()"></div>
        <table>
          <thead><tr><th>Product</th><th class="n">Sealed</th><th class="n">Ground</th><th>Dispensed</th></tr></thead>
          <tbody>
            @for (it of c.items; track it.name) {
              <tr><td>{{ it.name }}</td><td class="n">{{ it.sealed | number: '1.2-2' }} g</td>
                <td class="n">{{ it.ground | number: '1.2-2' }} g</td>
                <td>@for (f of it.fills; track f.day) { {{ f.grams | number: '1.2-2' }} g on {{ f.day }}<br> }</td></tr>
            }
          </tbody>
        </table>
        <p class="dim">Signed by <code>{{ c.fingerprint }}</code>. Scanned, it opens a page that checks the signature in the
          browser; nothing in it is sent to any server.</p>
      }
      <mat-slide-toggle [checked]="all()" (change)="load($event.checked)">every jar holding anything, not only the shelf</mat-slide-toggle>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Close</button>
    </mat-dialog-actions>
  `,
  styles: `
    .qr { width: min(320px, 70vw); margin: 0 auto 12px; background: #fff; border-radius: 12px; overflow: hidden; }
    .qr :is(svg) { display: block; width: 100%; height: auto; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th { text-align: left; font-weight: 500; color: var(--w-dim); padding: 4px; }
    td { padding: 4px; border-top: 1px solid var(--w-glass-edge); vertical-align: top; }
    .n { text-align: right; white-space: nowrap; }
    .dim { color: var(--w-dim); font-size: 12px; }
    .bad { color: var(--w-warn); }
    code { overflow-wrap: anywhere; }
  `,
})
export class CarryDialog {
  private readonly http = inject(HttpClient);
  private readonly sanitizer = inject(DomSanitizer);
  protected readonly carried = signal<Carried | null>(null);
  protected readonly qr = signal<SafeHtml>('');
  protected readonly all = signal(false);
  protected readonly error = signal('');

  constructor() {
    void this.load(false);
  }

  protected async load(all: boolean): Promise<void> {
    this.all.set(all);
    this.error.set('');
    try {
      const c = await firstValueFrom(this.http.get<Carried>(`/api/carry${all ? '?all=1' : ''}`));
      // The QR code is SVG wits drew itself, of rectangles alone.
      this.qr.set(this.sanitizer.bypassSecurityTrustHtml(c.qr));
      this.carried.set(c);
    } catch (e) {
      const body = (e as HttpErrorResponse).error as { error?: string } | null;
      this.error.set(body?.error ?? 'the statement could not be made');
    }
  }
}
