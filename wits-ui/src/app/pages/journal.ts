import { CdkVirtualScrollViewport, ScrollingModule } from '@angular/cdk/scrolling';
import { DatePipe, DecimalPipe } from '@angular/common';
import { afterRenderEffect, ChangeDetectionStrategy, Component, computed, DestroyRef, ElementRef, inject, signal, viewChild } from '@angular/core';
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
import { Binding, Keys } from '../data/keys';
import { Wits } from '../data/wits';
import { AmendDialog } from './amend';
import { Confirm, ConfirmDialog } from './confirm';

/** How each kind of entry looks: its icon, its colour, its verb. */
export const KINDS: Record<Entry['type'], { icon: string; tone: string; verb: string }> = {
  purchase: { icon: 'local_pharmacy', tone: 'var(--w-storage)', verb: 'picked up' },
  grind: { icon: 'grain', tone: 'var(--w-stash)', verb: 'ground' },
  sesh: { icon: 'air', tone: 'var(--w-sesh)', verb: 'session' },
  'avb-collect': { icon: 'savings', tone: 'var(--w-avb)', verb: 'collected AVB' },
  'avb-use': { icon: 'bakery_dining', tone: 'var(--w-avb)', verb: 'used AVB' },
  adjust: { icon: 'tune', tone: 'var(--w-dim)', verb: 'adjusted' },
  merge: { icon: 'merge', tone: 'var(--w-dim)', verb: 'merged another device' },
};

/** What each account is called on screen, and its colour. */
const ACCOUNTS: Record<string, { name: string; tone: string }> = {
  external: { name: 'outside', tone: 'var(--w-dim)' },
  storage: { name: 'sealed', tone: 'var(--w-storage)' },
  stash: { name: 'ground', tone: 'var(--w-stash)' },
  consumed: { name: 'vaped', tone: 'var(--w-sesh)' },
  avb: { name: 'AVB', tone: 'var(--w-avb)' },
};

/** A day's heading: what was ground and vaped on it, of the entries shown. */
interface DayRow {
  day: string;
  ground: number;
  vaped: number;
  count: number;
  entry?: undefined;
}
type Row = DayRow | { day?: undefined; entry: Entry };

/** One week of the scrubber: its grams, and the row its newest day starts at. */
interface Week {
  start: string;
  ground: number;
  vaped: number;
  row: number;
}

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
        <input #search matInput placeholder="Product  /" [ngModel]="query()" (ngModelChange)="query.set($event); cursor.set(-1)" />
      </mat-form-field>
      <mat-chip-listbox [value]="kind()" (change)="kind.set($event.value ?? ''); cursor.set(-1)" aria-label="Kind of entry">
        <mat-chip-option value="">all</mat-chip-option>
        @for (k of kinds; track k) {
          <mat-chip-option [value]="k">{{ KINDS[k].verb }}</mat-chip-option>
        }
      </mat-chip-listbox>
    </section>

    @if (weeks().length > 1) {
      <nav class="glass scrub" aria-label="The journal week by week: pick a week to go to it">
        <div class="bars">
          @for (w of weeks(); track w.start) {
            <button type="button" class="week" [class.here]="w.start === here()" (click)="go(w)"
              [attr.aria-label]="'Week of ' + (w.start | date: 'd MMMM y') + ': ' + (w.ground | number: '1.2-2') + ' g ground, ' + (w.vaped | number: '1.2-2') + ' g vaped'">
              <i class="v" [style.height.%]="(w.vaped / fullest()) * 100"></i>
              <i class="g" [style.height.%]="(w.ground / fullest()) * 100"></i>
            </button>
          }
        </div>
        <div class="ends dim"><span>{{ weeks().at(-1)?.start | date: 'MMM y' }}</span>
          <span class="key"><i class="g"></i>ground <i class="v"></i>vaped</span><span>{{ weeks()[0].start | date: 'MMM y' }}</span></div>
      </nav>
    }

    <cdk-virtual-scroll-viewport itemSize="56" class="glass list" (scrolledIndexChange)="top.set($event)">
      <div *cdkVirtualFor="let r of rows(); trackBy: track; let i = index" class="row" [class.cur]="i === cursor()">
        @if (r.day) {
          <div class="day">
            <div class="when"><span class="dnum num">{{ r.day | date: 'd' }}</span>
              <span class="dname">{{ r.day | date: 'EEEE' }}<br /><span class="dim">{{ r.day | date: 'MMMM y' }}</span></span></div>
            <div class="rail node"><i></i></div>
            <div class="sum">
              <span class="dim num">
                @if (r.ground) { <b class="tg">{{ r.ground | number: '1.2-2' }} g</b> ground }
                @if (r.ground && r.vaped) { · }
                @if (r.vaped) { <b class="tv">{{ r.vaped | number: '1.2-2' }} g</b> vaped }
                @if (!r.ground && !r.vaped) { {{ r.count }} {{ r.count === 1 ? 'entry' : 'entries' }} }
              </span>
              @if (r.ground || r.vaped) {
                <span class="split"><i class="g" [style.flex-grow]="r.ground"></i><i class="v" [style.flex-grow]="r.vaped"></i></span>
              }
            </div>
          </div>
        } @else if (r.entry; as e) {
          <div class="entry" [class.corrected]="e.corrected" [class.fix]="e.reverts" [style.--tone]="KINDS[e.type].tone">
            <span class="time dim num">{{ e.occurred_at | date: 'HH:mm' }}</span>
            <div class="rail"><span class="bead"><mat-icon>{{ KINDS[e.type].icon }}</mat-icon></span></div>
            <div class="what">
              <span class="line"><b>{{ KINDS[e.type].verb }}</b> {{ e.product_name }}
                @if (e.reverts) { <span class="dim">· undoes {{ e.reverts.slice(0, 7) }}</span> }</span>
              <span class="sub dim">
                <span class="flow" [attr.aria-label]="'from ' + account(e.from).name + ' to ' + account(e.to).name">
                  <i [style.background]="account(e.from).tone"></i>{{ account(e.from).name }}<mat-icon>arrow_forward</mat-icon><i
                    [style.background]="account(e.to).tone"></i>{{ account(e.to).name }}
                </span>
                @if (e.device) { · {{ takenWith(e) }} }
                @if (e.note) { · {{ e.note }} }
              </span>
            </div>
            <span class="g num" [style.--w.%]="share(e)">{{ e.grams | number: '1.2-2' }}<small> g</small></span>
            <span class="hash dim" [matTooltip]="e.hash">{{ e.hash.slice(0, 7) }}</span>
            @if (correctable(e)) {
              <span class="acts">
                <button mat-icon-button (click)="amend(e)" matTooltip="Amend the amount — undoes it and records it again"
                  aria-label="Amend the amount"><mat-icon>edit</mat-icon></button>
                <button mat-icon-button (click)="undo(e)" matTooltip="Undo — records a correction" aria-label="Undo"><mat-icon>undo</mat-icon></button>
              </span>
            } @else {
              <span class="spacer"></span>
            }
          </div>
        }
      </div>
    </cdk-virtual-scroll-viewport>
  `,
  styles: `
    :host { display: grid; gap: 20px; grid-template-rows: auto 1fr; height: calc(100dvh - var(--w-chrome)); }
    .head { padding: 18px 24px; display: flex; flex-wrap: wrap; gap: 18px; align-items: center; justify-content: space-between; }
    h1 { font: 600 28px var(--w-num); margin: 4px 0 0; }
    h1 .dim { font: 400 15px var(--w-font); }
    .search { width: 220px; }
    .list { height: 100%; }
    :host { grid-template-rows: auto auto 1fr; }
    .row { height: 56px; box-sizing: border-box; padding: 0 18px; }
    /* The rail runs through every row, so the days and their entries read as one line through time. */
    .rail { position: relative; height: 100%; display: grid; place-items: center; }
    .rail::before { content: ''; position: absolute; top: 0; bottom: 0; left: 50%; width: 2px; transform: translateX(-50%);
      background: color-mix(in srgb, var(--w-fg) 10%, transparent); }
    .bead { position: relative; width: 30px; height: 30px; border-radius: 50%; display: grid; place-items: center;
      background: color-mix(in srgb, var(--tone) 32%, var(--w-bg)); color: color-mix(in srgb, var(--tone) 70%, var(--w-fg));
      border: 1px solid color-mix(in srgb, var(--tone) 60%, transparent);
      box-shadow: 0 0 0 3px var(--w-bg), 0 0 12px color-mix(in srgb, var(--tone) 35%, transparent); }
    .bead mat-icon { font-size: 16px; width: 16px; height: 16px; }
    .node i { position: relative; width: 12px; height: 12px; border-radius: 50%; background: var(--w-primary);
      box-shadow: 0 0 0 4px var(--w-bg), 0 0 10px var(--w-primary); }
    .day { height: 100%; display: grid; grid-template-columns: 96px 36px minmax(0, 1fr); gap: 14px; align-items: center; }
    .when { display: flex; align-items: center; gap: 8px; justify-content: flex-end; }
    .dnum { font-size: 26px; font-weight: 600; color: var(--w-primary); line-height: 1; }
    .dname { font-size: 11px; line-height: 1.25; font-weight: 600; }
    .sum { display: grid; gap: 4px; max-width: 420px; font-size: 12px; }
    .sum b { font-weight: 600; }
    .tg { color: var(--w-stash); }
    .tv { color: var(--w-sesh); }
    .split { display: flex; gap: 2px; height: 5px; border-radius: 99px; overflow: hidden; }
    .split .g, .scrub .g { background: var(--w-stash); }
    .split .v, .scrub .v { background: var(--w-sesh); }
    .entry { height: 100%; display: grid; grid-template-columns: 96px 36px minmax(0, 1fr) 96px 64px 84px; gap: 14px; align-items: center;
      border-radius: 14px; transition: background .2s; }
    .entry:hover { background: color-mix(in srgb, var(--w-fg) 4%, transparent); }
    .row.cur .entry { background: color-mix(in srgb, var(--w-primary) 14%, transparent); box-shadow: inset 3px 0 0 var(--w-primary); }
    .entry.corrected .g, .entry.corrected .line { text-decoration: line-through; opacity: .5; }
    .entry.fix { opacity: .7; }
    .time { text-align: right; font-size: 12px; }
    .what { display: grid; min-width: 0; }
    .line, .sub { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .line b { font-weight: 600; }
    .sub { font-size: 12px; }
    .flow { display: inline-flex; align-items: center; gap: 4px; }
    .flow i { width: 7px; height: 7px; border-radius: 2px; }
    .flow mat-icon { font-size: 12px; width: 12px; height: 12px; }
    .g { position: relative; font-size: 17px; font-weight: 600; text-align: right; }
    .g::after { content: ''; position: absolute; right: 0; bottom: -4px; height: 3px; width: var(--w); border-radius: 99px;
      background: var(--tone); opacity: .7; }
    .g small { color: var(--w-dim); font-size: 11px; }
    .hash { font: 500 11px var(--w-mono); }
    .acts { display: flex; justify-content: flex-end; }
    .scrub { padding: 12px 18px 8px; display: grid; gap: 6px; }
    .bars { display: flex; align-items: flex-end; gap: 1px; height: 46px; flex-direction: row-reverse; }
    .week { flex: 1 1 0; min-width: 0; height: 100%; display: flex; flex-direction: column-reverse; justify-content: flex-start; gap: 1px;
      padding: 0; border: 0; border-radius: 2px; background: none; cursor: pointer; }
    .week i { display: block; width: 100%; border-radius: 1px; opacity: .55; }
    .week:hover i, .week.here i { opacity: 1; }
    .week.here { box-shadow: 0 2px 0 var(--w-primary); background: color-mix(in srgb, var(--w-primary) 12%, transparent); }
    .ends { display: flex; justify-content: space-between; font-size: 11px; }
    .key { display: inline-flex; align-items: center; gap: 4px; }
    .key i { width: 8px; height: 8px; border-radius: 2px; display: inline-block; margin-left: 6px; }
    @media (max-width: 860px) {
      .entry { grid-template-columns: 40px 30px minmax(0, 1fr) 64px 64px; gap: 8px; }
      .acts button { width: 32px; height: 32px; padding: 4px; }
      .day { grid-template-columns: 70px 30px minmax(0, 1fr); gap: 8px; }
      .dname, .hash { display: none; }
      .row { padding: 0 8px; }
      .bead { width: 26px; height: 26px; }
    }
    @media (prefers-reduced-motion: reduce) { .entry { transition: none; } }
  `,
})
export class Journal {
  /** What a session was taken with: the device, its mode, temperature and timer, and what heated it at which setting. */
  protected takenWith(e: Entry): string {
    const name = (slug: string) => this.wits.devices.value().find((d) => d.slug === slug)?.name ?? slug;
    const parts = [
      name(e.device ?? ''),
      e.mode ?? '',
      e.temperature ? `${e.temperature} °C` : '',
      e.timer ? `${e.timer} s` : '',
      e.heater ? `heated with ${name(e.heater)}${e.setting ? ' at ' + e.setting : ''}` : '',
    ];
    return parts.filter((p) => p).join(' · ');
  }

  protected readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);
  protected readonly KINDS = KINDS;
  /** The kinds to filter by: a merge stands in no list. */
  protected readonly kinds = (Object.keys(KINDS) as Entry['type'][]).filter((k) => k !== 'merge');
  /** The first row in sight, which the scrubber marks. */
  protected readonly top = signal(0);
  protected readonly kind = signal('');
  protected readonly query = signal('');
  /** The row the keys act on; -1 is none. */
  protected readonly cursor = signal(-1);
  private readonly viewport = viewChild.required(CdkVirtualScrollViewport);
  private readonly search = viewChild.required<ElementRef<HTMLInputElement>>('search');

  constructor() {
    inject(Keys).register('Journal', () => this.keys(), inject(DestroyRef));
    // The scrubber comes in above the list once the journal is read, and the
    // list measures how tall it is only when told: told after every render
    // that changed the weeks, it fills what it has.
    afterRenderEffect(() => {
      this.weeks();
      this.viewport().checkViewportSize();
    });
  }

  private keys(): Binding[] {
    return [
      { keys: ['j', 'ArrowDown'], label: 'j/k', help: 'move', run: () => {
        this.move(1);
      } },
      { keys: ['k', 'ArrowUp'], label: 'k', help: 'up', quiet: true, run: () => {
        this.move(-1);
      } },
      { keys: ['e'], help: 'amend', run: () => {
        const e = this.rows().at(this.cursor())?.entry;
        if (e && this.correctable(e)) {
          this.amend(e);
        }
      } },
      { keys: ['d'], help: 'undo', run: () => {
        const e = this.rows().at(this.cursor())?.entry;
        if (e && this.correctable(e)) {
          void this.undo(e);
        }
      } },
      { keys: ['f'], help: this.kind() ? `filter: ${KINDS[this.kind() as Entry['type']].verb}` : 'filter', run: () => {
        const order = ['', ...this.kinds];
        this.kind.set(order[(order.indexOf(this.kind()) + 1) % order.length]);
        this.cursor.set(-1);
      } },
      { keys: ['/'], help: 'search', run: () => {
        this.search().nativeElement.focus();
      } },
    ];
  }

  /** Moves the cursor to the next entry that way, passing over day headings. */
  private move(delta: 1 | -1): void {
    const rows = this.rows();
    let i = this.cursor();
    do {
      i += delta;
    } while (i >= 0 && i < rows.length && !rows[i].entry);
    if (i < 0 || i >= rows.length) {
      return;
    }
    this.cursor.set(i);
    const v = this.viewport();
    const first = Math.floor(v.measureScrollOffset() / 56);
    const fit = Math.floor(v.getViewportSize() / 56);
    if (i < first + 1 || i >= first + fit - 1) {
      v.scrollToIndex(Math.max(i - Math.floor(fit / 2), 0));
    }
  }

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
    return (e.grams / Math.max(this.most().get(e.type) ?? 0, 0.01)) * 100;
  }

  protected readonly rows = computed<Row[]>(() => {
    const out: Row[] = [];
    let head: DayRow | undefined;
    for (const e of this.filtered()) {
      const day = e.occurred_at.slice(0, 10);
      if (day !== head?.day) {
        head = { day, ground: 0, vaped: 0, count: 0 };
        out.push(head);
      }
      head.count++;
      // A correction undoes what it names; the day's grams are what stands.
      if (!e.corrected && !e.reverts) {
        if (e.type === 'grind') {
          head.ground = Math.round((head.ground + e.grams) * 100) / 100;
        } else if (e.type === 'sesh') {
          head.vaped = Math.round((head.vaped + e.grams) * 100) / 100;
        }
      }
    }
    return out.length ? this.withEntries(out) : out;
  });

  /** The day headings with their entries under them, in the order shown. */
  private withEntries(days: Row[]): Row[] {
    const out: Row[] = [];
    let i = 0;
    for (const e of this.filtered()) {
      if (days[i]?.day === e.occurred_at.slice(0, 10)) {
        out.push(days[i]);
        i++;
      }
      out.push({ entry: e });
    }
    return out;
  }

  /** The weeks the rows cover, newest first, each starting on its Monday. */
  protected readonly weeks = computed<Week[]>(() => {
    const out: Week[] = [];
    this.rows().forEach((r, i) => {
      if (!r.day) {
        return;
      }
      const start = monday(r.day);
      let w = out.at(-1);
      if (w?.start !== start) {
        w = { start, ground: 0, vaped: 0, row: i };
        out.push(w);
      }
      w.ground += r.ground;
      w.vaped += r.vaped;
    });
    return out;
  });
  /** The fullest week's grams, which the scrubber's bars are measured against. */
  protected readonly fullest = computed(() => Math.max(0.01, ...this.weeks().map((w) => w.ground + w.vaped)));
  /** The week the first row in sight is in. */
  protected readonly here = computed(() => {
    const rows = this.rows();
    for (let i = Math.min(this.top(), rows.length - 1); i >= 0; i--) {
      const r = rows[i];
      if (r.day) {
        return monday(r.day);
      }
    }
    return '';
  });

  protected go(w: Week): void {
    this.viewport().scrollToIndex(w.row);
  }

  protected account(name: string): { name: string; tone: string } {
    return ACCOUNTS[name] ?? { name, tone: 'var(--w-dim)' };
  }

  protected track = (_: number, r: Row) => r.entry?.hash ?? `day-${r.day}`;

  /** Whether an entry can be amended or undone: one that stands, and is no correction itself. */
  protected correctable(e: Entry): boolean {
    return !e.reverts && e.type !== 'adjust' && e.type !== 'merge' && !e.corrected;
  }

  protected amend(e: Entry): void {
    this.dialog.open(AmendDialog, { data: { entry: e, verb: KINDS[e.type].verb }, autoFocus: 'first-tabbable' });
  }

  protected async undo(e: Entry): Promise<void> {
    const ok = await firstValueFrom(
      this.dialog
        .open<ConfirmDialog, Confirm, boolean>(ConfirmDialog, {
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

/** The Monday of a day's week, as YYYY-MM-DD. */
function monday(day: string): string {
  const d = new Date(`${day}T12:00:00`);
  d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
  const p = (n: number) => String(n).padStart(2, '0');
  return `${String(d.getFullYear())}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}
