import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Router, RouterLink } from '@angular/router';

import { openDemo } from '../data/demo';
import { CATEGORIES } from '../data/devices';
import { Wits } from '../data/wits';
import { CountUp } from '../viz/count-up';
import { FillRing } from '../viz/fill-ring';
import { GlassJar } from '../viz/glass-jar';
import { Heatmap } from '../viz/heatmap';
import { ProjectionChart } from '../viz/projection-chart';
import { DeviceForm } from './device-form';
import { openRecord } from './record';
import { TodayCard } from './today';

@Component({
  selector: 'w-dashboard',
  imports: [
    CountUp,
    MatButtonModule,
    TodayCard,
    DatePipe,
    DecimalPipe,
    FillRing,
    GlassJar,
    Heatmap,
    MatIconModule,
    MatProgressSpinnerModule,
    MatTooltipModule,
    ProjectionChart,
    RouterLink,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (wits.overview.value(); as o) {
      @if (o.fill; as f) {
        <section class="hero glass">
          <div class="ring">
            <w-fill-ring [dispensed]="f.dispensed" [sealed]="f.remaining" [stash]="ground().stash"
              [avb]="ground().avb" [carried]="f.carried.grams" />
          </div>
          <div class="facts">
            <div class="eyebrow">Prescription {{ f.number }} · <span class="scope">the prescription</span></div>
            <h1>
              Day <w-count [value]="f.day" [decimals]="0" />
              <span class="dim">since {{ f.start | date: 'd MMMM' }}</span>
            </h1>
            <div class="stats">
              <div class="stat">
                <span class="eyebrow">days' worth</span>
                <span class="v num"><w-count [value]="f.days_worth" [decimals]="0" /></span>
                <span class="dim">at {{ f.per_active_day | number: '1.2-2' }} g a day</span>
              </div>
              <div class="stat">
                <span class="eyebrow">still sealed</span>
                <span class="v num"><w-count [value]="f.remaining_pct * 100" [decimals]="0" />%</span>
                <span class="dim">of this prescription</span>
              </div>
            </div>
            <div class="legend">
              <span><i style="background: var(--w-storage)"></i>sealed</span>
              <span><i style="background: var(--w-stash)"></i>ground</span>
              <span><i style="background: var(--w-avb)"></i>already vaped</span>
              @if (f.carried.grams > 0) {
                <span><i style="background: var(--w-primary)"></i>older jars</span>
              }
            </div>
            @if (f.carried.grams > 0) {
              <p class="carried">
                <mat-icon>history</mat-icon>
                + {{ f.carried.grams | number: '1.2-2' }} g in {{ f.carried.jars }} older
                {{ f.carried.jars === 1 ? 'jar' : 'jars' }}, from {{ f.carried.cycles }} earlier
                {{ f.carried.cycles === 1 ? 'prescription' : 'prescriptions' }} still open
              </p>
            }
          </div>
        </section>

        <section class="shelf">
          @for (t of tiles(); track t.label) {
            <div class="tile glass" [style.--tone]="t.tone">
              <span class="eyebrow">{{ t.label }} <span class="scope">the shelf</span></span>
              <span class="tile-v num"><w-count [value]="t.grams" /><small> g</small></span>
              <div class="bar"><div [style.width.%]="t.share"></div></div>
              @if (t.avb && lastYield(); as y) {
                <span class="yield">last emptying gave back {{ y.ratio * 100 | number: '1.0-0' }}% —
                  {{ y.collected | number: '1.2-2' }} g of {{ y.from | number: '1.2-2' }} g</span>
              }
            </div>
          }
        </section>

        <w-today />

        <section class="jars glass">
          <header>
            <span class="eyebrow">{{ o.fills.length > 1 ? 'every open prescription, jar by jar' : 'this prescription, jar by jar' }}</span>
            <span class="scope" matTooltip="Each prescription's own grams: what it dispensed, and what of that is still sealed">the prescription</span>
          </header>
          @for (g of o.fills; track g.number; let first = $first) {
            <div class="group" [class.older]="!first">
              @if (o.fills.length > 1) {
                <div class="group-head">
                  <strong>Prescription {{ g.number }}</strong>
                  <span class="dim">{{ first ? 'in progress' : 'older' }} · since {{ g.start | date: 'd MMMM' }} · day {{ g.day }}</span>
                  <span class="num">{{ g.remaining | number: '1.2-2' }} g <span class="dim">of {{ g.dispensed | number: '1.0-0' }} still sealed ·
                    {{ g.remaining_pct * 100 | number: '1.0-0' }}%</span></span>
                </div>
              }
              <div class="row">
                @for (p of g.products; track p.slug) {
                  <a class="figure" [routerLink]="['/shelf/jars', p.slug]" [attr.aria-label]="'Open ' + p.name">
                    <w-jar [capacity]="p.dispensed"
                      [layers]="[
                        { grams: p.sealed, color: 'var(--w-storage)', label: 'sealed' },
                        { grams: p.stash, color: 'var(--w-stash)', label: 'ground' },
                        { grams: p.avb, color: 'var(--w-avb)', label: 'already vaped' },
                      ]" />
                    <span class="caption">
                      <strong>{{ p.name }}</strong>
                      <span class="num">{{ p.sealed | number: '1.2-2' }} g <span class="dim">of {{ p.dispensed | number: '1.0-0' }}</span></span>
                    </span>
                  </a>
                }
              </div>
            </div>
          }
        </section>

        <section class="projection glass">
          <header>
            <div>
              <span class="eyebrow">supply projection</span>
              <h2>
                @if (f.shelf_projection.empty_on; as e) {
                  Empty around <span class="warn">{{ e | date: 'd MMMM' }}</span>
                } @else {
                  No rate to project from yet
                }
              </h2>
              @if (f.refill; as r) {
                <p class="refill" [class.warn]="r.overdue">
                  <mat-icon>event</mat-icon>
                  @if (r.overdue) {
                    Order the next prescription now — {{ r.order_by | date: 'd MMMM' }} has passed
                  } @else {
                    Order the next prescription by <b>{{ r.order_by | date: 'EEE d MMMM' }}</b>
                  }
                  <span class="dim">· {{ r.lead_days }} {{ r.lead_days === 1 ? 'day' : 'days' }} for the pharmacy</span>
                </p>
              }
            </div>
            <span class="scope" matTooltip="Everything standing when this prescription arrived, less every grind since">the shelf</span>
          </header>
          <w-projection [history]="f.shelf_projection.history" [projected]="f.shelf_projection.projected" />
        </section>
      } @else if (fresh()) {
        <section class="welcome glass">
          <div class="hello">
            <span class="eyebrow">welcome to wits</span>
            <h1>Every gram, from the pharmacy to the vaporizer</h1>
            <p class="dim">wits keeps track of what your prescriptions held, what you ground and vaped, and what is left —
              and works out how long it lasts and when to order the next. Nothing leaves this device unless you send it.</p>
          </div>
          <div class="ways">
            <button class="way" (click)="record(2)">
              <mat-icon>local_pharmacy</mat-icon>
              <strong>Record your first prescription</strong>
              <span class="dim">What the pharmacy handed you: each strain and its grams. A photo of the label or the invoice fills it in.</span>
            </button>
            <button class="way" (click)="addGear()">
              <mat-icon>air</mat-icon>
              <strong>Add your gear</strong>
              <span class="dim">The vaporizers, grinders, heaters and glass you already have, so a session can say what it was had with.</span>
              @if (gear().length) {
                <span class="gear">
                  @for (d of gear(); track d.slug) {
                    <span class="chip"><mat-icon>{{ icon(d.category) }}</mat-icon>{{ d.name }}</span>
                  }
                  <span class="dim">Add another</span>
                </span>
              }
            </button>
            <a class="way" routerLink="/repository">
              <mat-icon>table_view</mat-icon>
              <strong>Bring a spreadsheet across</strong>
              <span class="dim">Years of tracking in a workbook, each sheet a prescription. Read and checked before anything is written.</span>
            </a>
            <button class="way" (click)="demo()" [disabled]="seeding()">
              <mat-icon>science</mat-icon>
              <strong>Look around first</strong>
              <span class="dim">Three years of made-up use beside yours, so every page has something on it. Yours stays as it is.</span>
            </button>
          </div>
          <a class="guide" routerLink="/guide"><mat-icon>menu_book</mat-icon>The user guide shows every page, and what it is for</a>
        </section>
      } @else {
        <section class="empty glass">
          <mat-icon>inventory_2</mat-icon>
          <h1>Nothing on the shelf</h1>
          <p class="dim">Every jar is empty. When the next prescription comes, record it.</p>
          <button mat-flat-button (click)="record(2)"><mat-icon>local_pharmacy</mat-icon>Record a prescription</button>
        </section>
      }

      @if (wits.days.value().length) {
        <section class="rhythm">
          <div class="glass pad">
            <span class="eyebrow">grinding · last 20 weeks</span>
            <w-heatmap [days]="wits.days.value()" metric="ground" [weeks]="20" tone="var(--w-stash)" />
          </div>
          <div class="glass pad">
            <span class="eyebrow">sessions · last 20 weeks</span>
            <w-heatmap [days]="wits.days.value()" metric="seshed" [weeks]="20" tone="var(--w-sesh)" />
          </div>
          @if (!o.chain_ok) {
            <div class="glass pad">
              <span class="eyebrow">the journal</span>
              <div class="chain bad"><mat-icon>gpp_bad</mat-icon>the journal was changed outside wits — run wits fsck</div>
            </div>
          }
        </section>
      }
    } @else if (wits.overview.error()) {
      <section class="empty glass">
        <mat-icon>cloud_off</mat-icon>
        <h1>wits-server is not answering</h1>
        <p class="dim">Start it in your repository: <code>wits-server</code></p>
      </section>
    } @else {
      <div class="loading"><mat-spinner diameter="42" /></div>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .hero { display: grid; grid-template-columns: minmax(220px, 340px) 1fr; gap: 32px; padding: 28px 32px; align-items: center; }
    @media (max-width: 760px) { .hero { grid-template-columns: 1fr; justify-items: center; text-align: center; } }
    h1 { font: 600 34px/1.1 var(--w-num); margin: 8px 0 18px; letter-spacing: -0.03em; }
    h1 .dim { font: 400 16px var(--w-font); margin-left: 10px; letter-spacing: 0; }
    h2 { font: 600 22px var(--w-num); margin: 6px 0 0; }
    .warn { color: var(--w-warn); }
    .stats { display: flex; gap: 36px; flex-wrap: wrap; }
    .stat { display: grid; gap: 2px; }
    .stat .v { font-size: 40px; font-weight: 600; line-height: 1.05; }
    .legend { display: flex; gap: 16px; margin-top: 22px; flex-wrap: wrap; font-size: 12px; color: var(--w-dim); }
    .legend i { width: 10px; height: 10px; border-radius: 3px; display: inline-block; margin-right: 6px; }
    .carried { display: flex; gap: 8px; align-items: center; color: var(--w-primary); margin: 16px 0 0; font-size: 13px; }
    .shelf { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; }
    .tile { padding: 18px 22px; display: grid; gap: 6px; }
    .tile-v { font-size: 30px; font-weight: 600; color: var(--tone); }
    .tile-v small { font-size: 14px; color: var(--w-dim); }
    .bar { height: 6px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); overflow: hidden; }
    .bar div { height: 100%; background: var(--tone); border-radius: 99px; box-shadow: 0 0 12px var(--tone);
      transition: width 1s cubic-bezier(.2,.8,.2,1); }
    .jars, .projection { padding: 22px 26px; }
    .refill { display: flex; align-items: center; gap: 6px; margin: 6px 0 0; color: var(--mat-sys-on-surface); }
    .refill mat-icon { font-size: 18px; width: 18px; height: 18px; color: var(--w-accent); }
    .refill .dim { color: var(--w-dim); }
    .yield { display: block; margin-top: 8px; font-size: 12px; color: var(--w-dim); }
    header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
    .group { display: grid; gap: 14px; }
    .group + .group { margin-top: 18px; padding-top: 16px; border-top: 1px solid var(--w-glass-edge); }
    .group-head { display: flex; flex-wrap: wrap; gap: 4px 14px; align-items: baseline; font-size: 13px; }
    .group-head strong { font: 600 15px var(--w-num); }
    .dim { color: var(--w-dim); }
    .row { display: flex; gap: 28px; flex-wrap: wrap; justify-content: space-around; }
    .older .row { justify-content: flex-start; }
    .figure { margin: 0; display: grid; justify-items: center; gap: 10px; width: 150px; color: inherit; text-decoration: none;
      border-radius: 14px; padding: 6px 0; transition: transform .25s, background .25s; }
    .figure:hover, .figure:focus-visible { transform: translateY(-3px); background: color-mix(in srgb, var(--w-primary) 8%, transparent); outline: none; }
    .figure w-jar { width: 110px; height: 154px; }
    .older .figure w-jar { width: 80px; height: 112px; }
    .caption { display: grid; text-align: center; gap: 2px; font-size: 13px; }
    .rhythm { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
    .pad { padding: 18px 22px; display: grid; gap: 12px; align-content: start; }
    .chain { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--w-stash); margin-top: auto; }
    .chain.bad { color: var(--w-warn); }
    .empty { padding: 48px; display: grid; justify-items: center; text-align: center; }
    .empty mat-icon { font-size: 48px; width: 48px; height: 48px; color: var(--w-primary); }
    .empty button mat-icon { font-size: 20px; width: 20px; height: 20px; color: inherit; }
    .welcome { display: grid; gap: 24px; padding: 36px 40px;
      background: radial-gradient(90% 120% at 0% 0%, color-mix(in srgb, var(--w-primary) 16%, transparent), transparent 60%), var(--w-glass); }
    .hello { display: grid; gap: 10px; max-width: 64ch; }
    .hello h1 { margin: 0; font: 600 clamp(26px, 3.2vw, 38px)/1.15 var(--w-head); }
    .hello p { margin: 0; line-height: 1.6; }
    .ways { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 16px; }
    .way { display: grid; gap: 8px; align-content: start; padding: 20px; border-radius: calc(var(--w-radius) * 0.8); text-align: left;
      border: 1px solid var(--w-glass-edge); background: color-mix(in srgb, var(--w-fg) 3%, transparent); color: inherit;
      font: inherit; text-decoration: none; cursor: pointer; transition: border-color .2s, transform .2s; }
    .way:hover, .way:focus-visible { border-color: var(--w-primary); transform: translateY(-2px); }
    .way:disabled { opacity: .6; cursor: progress; }
    .way mat-icon { font-size: 30px; width: 30px; height: 30px; color: var(--w-primary); }
    .way strong { font: 600 17px var(--w-head); }
    .way .dim { font-size: 13px; line-height: 1.5; }
    .gear { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; margin-top: 4px; }
    .gear .chip { display: inline-flex; align-items: center; gap: 4px; padding: 2px 10px 2px 6px; border-radius: 99px; font-size: 12px;
      background: color-mix(in srgb, var(--w-primary) 14%, transparent); }
    .gear .chip mat-icon { font-size: 16px; width: 16px; height: 16px; }
    .gear .dim { font-size: 12px; }
    .guide { display: inline-flex; gap: 8px; align-items: center; color: var(--w-primary); text-decoration: none; font-size: 14px; }
    @include bp.phone {
      .welcome { padding: 22px 18px; }
      .ways { grid-template-columns: minmax(0, 1fr); }
    }
    .loading { display: grid; place-items: center; height: 50vh; }
    /* A phone: three jars to a row, every fill's, so two fills are not a screen each. */
    @include bp.phone {
      .jars { padding: 18px 16px; }
      .row, .older .row { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px 8px; }
      .figure { width: auto; gap: 6px; }
      .figure w-jar { width: 60px; height: 84px; }
      .older .figure w-jar { width: 48px; height: 67px; }
      .caption { font-size: 11px; }
      /* One line of words that wraps, not four columns of them. */
      .refill { display: block; }
      .refill mat-icon { vertical-align: -3px; margin-right: 4px; }
      .refill .dim { margin-left: 4px; }
      .projection { padding: 18px 16px; }
    }
  `,
})
export class Dashboard {
  protected readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);
  private readonly snack = inject(MatSnackBar);
  /** A repository with nothing in its journal: the welcome, not an empty shelf. */
  protected readonly fresh = computed(() => this.wits.place.value()?.entries === 0);
  protected readonly seeding = signal(false);

  /** The gear added so far, while the journal is still empty. */
  protected readonly gear = computed(() => this.wits.devices.value().filter((d) => !d.retired));

  protected record(tab: number): void {
    openRecord(this.dialog, { tab });
  }

  protected addGear(): void {
    this.dialog.open(DeviceForm, { maxWidth: 'min(720px, 94vw)', autoFocus: 'first-tabbable' });
  }

  protected icon(category: string): string {
    return CATEGORIES.find((c) => c.id === category)?.icon ?? 'devices';
  }

  protected async demo(): Promise<void> {
    this.seeding.set(true);
    try {
      await openDemo(this.wits, this.router, this.snack);
    } finally {
      this.seeding.set(false);
    }
  }

  /** The fill's ground grams, split as its rows split them. */
  protected readonly ground = computed(() => {
    const ps = this.wits.overview.value()?.fill?.products ?? [];
    return {
      stash: ps.reduce((s, p) => s + p.stash, 0),
      avb: ps.reduce((s, p) => s + p.avb, 0),
    };
  });

  /** The newest collection's yield, for the AVB tile. */
  protected readonly lastYield = computed(() => this.wits.avb.value()?.yields.at(-1));

  protected readonly tiles = computed(() => {
    const s = this.wits.overview.value()?.shelf;
    if (!s) {
      return [];
    }
    const top = Math.max(s.storage, s.stash, s.avb, 1);
    return [
      { label: 'sealed', grams: s.storage, tone: 'var(--w-storage)', share: (s.storage / top) * 100, avb: false },
      { label: 'ground, ready to use', grams: s.stash, tone: 'var(--w-stash)', share: (s.stash / top) * 100, avb: false },
      { label: 'already vaped · AVB', grams: s.avb, tone: 'var(--w-avb)', share: (s.avb / top) * 100, avb: true },
    ];
  });
}
