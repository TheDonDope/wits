import { ScrollingModule } from '@angular/cdk/scrolling';
import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatChipsModule } from '@angular/material/chips';
import { MatDialog } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { firstValueFrom } from 'rxjs';

import { Entry } from '../data/models';
import { Wits } from '../data/wits';
import { ConfirmDialog } from './confirm';

/** How each kind of entry looks: its icon, its colour, its verb. */
export const KINDS: Record<Entry['type'], { icon: string; tone: string; verb: string }> = {
  purchase: { icon: 'local_pharmacy', tone: 'var(--w-storage)', verb: 'picked up' },
  grind: { icon: 'grain', tone: 'var(--w-stash)', verb: 'ground' },
  sesh: { icon: 'air', tone: 'var(--w-sesh)', verb: 'session' },
  'avb-collect': { icon: 'savings', tone: 'var(--w-avb)', verb: 'collected AVB' },
  'avb-use': { icon: 'bakery_dining', tone: 'var(--w-avb)', verb: 'used AVB' },
  adjust: { icon: 'tune', tone: 'var(--w-dim)', verb: 'adjusted' },
};

type Row = { day: string; entry?: undefined } | { day?: undefined; entry: Entry };

@Component({
  selector: 'w-journal',
  imports: [
    DatePipe,
    DecimalPipe,
    FormsModule,
    MatButtonModule,
    MatChipsModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatTooltipModule,
    ScrollingModule,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="glass head">
      <div>
        <span class="eyebrow">the journal</span>
        <h1 class="num">{{ shown() | number }} <span class="dim">of {{ wits.journal.value().total | number }} entries</span></h1>
      </div>
      <mat-form-field appearance="outline" subscriptSizing="dynamic" class="search">
        <mat-icon matPrefix>search</mat-icon>
        <input matInput placeholder="Product" [ngModel]="query()" (ngModelChange)="query.set($event)" />
      </mat-form-field>
      <mat-chip-listbox [value]="kind()" (change)="kind.set($event.value ?? '')" aria-label="Kind of entry">
        <mat-chip-option value="">all</mat-chip-option>
        @for (k of kinds; track k) {
          <mat-chip-option [value]="k">{{ KINDS[k].verb }}</mat-chip-option>
        }
      </mat-chip-listbox>
    </section>

    <cdk-virtual-scroll-viewport itemSize="56" class="glass list">
      <div *cdkVirtualFor="let r of rows(); trackBy: track" class="row">
        @if (r.day) {
          <div class="day">
            <span>{{ r.day | date: 'EEEE, d MMMM y' }}</span>
            @if (isAhead(r.day)) { <span class="scope ahead">upcoming</span> }
          </div>
        } @else if (r.entry; as e) {
          <div class="entry" [class.corrected]="e.corrected" [class.fix]="e.reverts">
            <div class="rail" [style.--tone]="KINDS[e.type].tone">
              <mat-icon>{{ KINDS[e.type].icon }}</mat-icon>
            </div>
            <span class="verb">{{ KINDS[e.type].verb }}</span>
            <span class="g num">{{ e.grams | number: '1.2-2' }}<small> g</small></span>
            <div class="amount"><i [style.width.%]="share(e)" [style.background]="KINDS[e.type].tone"></i></div>
            <span class="what">
              {{ e.product_name }}
              @if (e.device) { <span class="dim"> · {{ e.device }}{{ e.temperature ? ' · ' + e.temperature + '°C' : '' }}</span> }
              @if (e.note) { <span class="dim"> · {{ e.note }}</span> }
            </span>
            <span class="hash dim" [matTooltip]="e.hash">{{ e.hash.slice(0, 7) }}</span>
            @if (!e.reverts && e.type !== 'adjust' && !e.corrected) {
              <button mat-icon-button (click)="undo(e)" matTooltip="Undo — records a correction">
                <mat-icon>undo</mat-icon>
              </button>
            } @else {
              <span class="spacer"></span>
            }
          </div>
        }
      </div>
    </cdk-virtual-scroll-viewport>
  `,
  styles: `
    :host { display: grid; gap: 20px; grid-template-rows: auto 1fr; height: calc(100vh - 150px); }
    .head { padding: 18px 24px; display: flex; flex-wrap: wrap; gap: 18px; align-items: center; justify-content: space-between; }
    h1 { font: 600 28px var(--w-num); margin: 4px 0 0; }
    h1 .dim { font: 400 15px 'Inter Variable'; }
    .search { width: 220px; }
    .list { height: 100%; }
    .row { height: 56px; box-sizing: border-box; padding: 0 18px; }
    .day { height: 100%; display: flex; align-items: flex-end; gap: 10px; padding-bottom: 8px;
      font: 600 12px 'Inter Variable'; letter-spacing: .08em; text-transform: uppercase; color: var(--w-violet); }
    .ahead { color: var(--w-avb); }
    .entry { height: 100%; display: grid; grid-template-columns: 36px 120px 84px 90px 1fr auto 40px; gap: 14px; align-items: center;
      border-radius: 14px; padding: 0 8px; transition: background .2s; }
    .entry:hover { background: rgba(255,255,255,0.04); }
    .entry.corrected .g, .entry.corrected .what { text-decoration: line-through; opacity: .5; }
    .entry.fix { opacity: .7; }
    .rail { width: 32px; height: 32px; border-radius: 10px; display: grid; place-items: center;
      background: color-mix(in srgb, var(--tone) 18%, transparent); color: var(--tone); }
    .rail mat-icon { font-size: 18px; width: 18px; height: 18px; }
    .verb { font-weight: 600; }
    .g { font-size: 17px; font-weight: 600; text-align: right; }
    .g small { color: var(--w-dim); font-size: 11px; }
    .amount { height: 6px; background: rgba(255,255,255,0.05); border-radius: 99px; overflow: hidden; }
    .amount i { display: block; height: 100%; border-radius: 99px; opacity: .85; }
    .what { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .hash { font: 500 11px var(--w-mono); }
    @media (max-width: 860px) { .entry { grid-template-columns: 36px 1fr 70px 40px; } .verb, .amount, .hash { display: none; } }
  `,
})
export class Journal {
  protected readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);
  protected readonly KINDS = KINDS;
  protected readonly kinds = Object.keys(KINDS) as Entry['type'][];
  protected readonly kind = signal('');
  protected readonly query = signal('');

  private readonly filtered = computed(() => {
    const k = this.kind();
    const q = this.query().trim().toLowerCase();
    return this.wits.journal.value().entries.filter(
      (e) =>
        (!k || e.type === k) &&
        (!q || (e.product_name ?? '').toLowerCase().includes(q) || (e.product ?? '').includes(q)),
    );
  });

  protected readonly shown = computed(() => this.filtered().length);
  /** The largest entry of each kind: a grind's bar is measured against grinds. */
  private readonly most = computed(() => {
    const m = new Map<string, number>();
    for (const e of this.filtered()) {
      m.set(e.type, Math.max(m.get(e.type) ?? 0, e.grams));
    }
    return m;
  });

  protected share(e: Entry): number {
    return (e.grams / (this.most().get(e.type) || 1)) * 100;
  }

  protected readonly rows = computed<Row[]>(() => {
    const out: Row[] = [];
    let last = '';
    for (const e of this.filtered()) {
      const day = e.occurred_at.slice(0, 10);
      if (day !== last) {
        out.push({ day });
        last = day;
      }
      out.push({ entry: e });
    }
    return out;
  });

  protected track = (_: number, r: Row) => r.entry?.hash ?? `day-${r.day}`;

  protected isAhead(day: string): boolean {
    return day > new Date().toISOString().slice(0, 10);
  }

  protected async undo(e: Entry): Promise<void> {
    const ok = await firstValueFrom(
      this.dialog
        .open(ConfirmDialog, {
          data: {
            title: `Undo this ${KINDS[e.type].verb}?`,
            body: `${e.grams.toFixed(2)} g of ${e.product_name ?? 'AVB'} on ${new Date(e.occurred_at).toLocaleDateString()}. Nothing is deleted: a correction moves the grams back, and the journal keeps both.`,
            yes: 'Undo it',
          },
        })
        .afterClosed(),
    );
    if (!ok) {
      return;
    }
    try {
      const w = await this.wits.undo(e.hash);
      this.snack.open(w.message, undefined, { duration: 3500 });
    } catch (err) {
      this.snack.open((err as Error).message, 'OK', { duration: 6000 });
    }
  }
}
