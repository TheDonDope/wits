import { DatePipe, DecimalPipe } from '@angular/common';
import { httpResource } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';

import { DayToday, Entry } from '../data/models';
import { Wits } from '../data/wits';
import { CountUp } from '../viz/count-up';
import { DayClock, Moment } from '../viz/day-clock';
import { Donut, Slice } from '../viz/donut';
import { tone } from '../viz/tones';
import { openRecord } from './record';

/**
 * The day so far, on the dashboard, as wits today says it: the THC and CBD
 * in what went through a device — grams × the strain's per cent × 10 — summed
 * and against a usual day, by strain and by device as rings, the sessions on
 * a clock of the day, and each session in a row with its share. What was in
 * the material, not what reached the body.
 */
@Component({
  selector: 'w-today',
  imports: [CountUp, DatePipe, DayClock, DecimalPipe, Donut, MatButtonModule, MatIconModule, MatTooltipModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (day.value(); as d) {
      <section class="glass today">
        <header>
          <span class="eyebrow">today <span class="dim">· {{ d.day | date: 'EEEE, d MMMM' }}</span> <span class="scope">the day</span></span>
          <span class="dim note" matTooltip="A vaporizer releases part of what is in the material, and the body takes in part of that">
            <mat-icon>info</mat-icon>in the material, not what reached you
          </span>
        </header>
        @if (again(); as l) {
          <button mat-flat-button class="again" (click)="repeat(l)" [disabled]="busy()"
            [matTooltip]="'Records ' + l.grams + ' g of ' + l.product_name + ' now, as last time'">
            <mat-icon>replay</mat-icon>Same as last time
            <span class="what">{{ l.grams | number: '1.2-2' }} g · {{ l.product_name }}{{ deviceAt(l) }}</span>
          </button>
        }

        @if (d.sessions.length) {
          <div class="top">
            <div class="sum">
              <span class="v num"><w-count [value]="d.thc_mg" [decimals]="0" />&thinsp;<small>mg THC</small></span>
              <span class="dim">in {{ d.seshed | number: '1.2-2' }} g through {{ d.sessions.length }}
                {{ d.sessions.length === 1 ? 'session' : 'sessions' }} · <b>{{ d.cbd_mg | number: '1.0-1' }} mg CBD</b></span>
              @if (d.usual.days) {
                <div class="vs">
                  <div class="row"><span>today</span><i [style.width.%]="share(d.thc_mg, d)" class="now"></i><b class="num">{{ d.thc_mg | number: '1.0-0' }} mg</b></div>
                  <div class="row"><span>usual</span><i [style.width.%]="share(d.usual.thc_mg, d)"></i><b class="num">{{ d.usual.thc_mg | number: '1.0-0' }} mg</b></div>
                  <small class="dim">a usual day of the last 30 with a session, over {{ d.usual.days }} days</small>
                </div>
              }
              @if (d.unknown) {
                <p class="warn">{{ d.unknown }} {{ d.unknown === 1 ? 'session' : 'sessions' }} of a strain with no THC written down:
                  not counted.</p>
              }
              <div class="chips">
                @if (d.ground) { <span><mat-icon>grain</mat-icon>{{ d.ground | number: '1.2-2' }} g ground</span> }
                @for (p of d.picked; track p.name) { <span><mat-icon>local_pharmacy</mat-icon>{{ p.grams }} g {{ p.name }} picked up</span> }
                @if (d.avb_collected) { <span><mat-icon>savings</mat-icon>{{ d.avb_collected | number: '1.2-2' }} g AVB collected</span> }
              </div>
            </div>
            <figure>
              <w-donut [slices]="byStrain()" [center]="(d.thc_mg | number: '1.0-0') ?? ''" unit="mg THC" />
              <figcaption>by strain</figcaption>
              <ul class="legend">@for (x of byStrain(); track x.label) { <li><i [style.background]="x.color"></i><span>{{ x.label }}</span><b class="num">{{ x.value | number: '1.0-0' }}</b></li> }</ul>
            </figure>
            <figure>
              <w-donut [slices]="byDevice()" [center]="(d.seshed | number: '1.2-2') ?? ''" unit="grams" />
              <figcaption>by device</figcaption>
              <ul class="legend">@for (x of byDevice(); track x.label) { <li><i [style.background]="x.color"></i><span>{{ x.label }}</span><b class="num">{{ x.value | number: '1.2-2' }}</b></li> }</ul>
            </figure>
            <figure>
              <w-day-clock [moments]="moments()" [now]="now()" />
              <figcaption>through the day</figcaption>
            </figure>
          </div>

          <div class="split" role="img" [attr.aria-label]="'THC by session'">
            @for (s of d.sessions; track s.hash) {
              <i [style.flex-grow]="s.thc_mg || 0.001" [style.background]="colour(s.name)" [matTooltip]="(s.at | date: 'HH:mm') + ' · ' + (s.thc_mg | number: '1.0-1') + ' mg'"></i>
            }
          </div>
          <ol class="sessions">
            @for (s of d.sessions; track s.hash) {
              <li [style.--tone]="colour(s.name)">
                <span class="time num">{{ s.at | date: 'HH:mm' }}</span>
                <span class="what"><b>{{ s.name }}</b>
                  <small>{{ s.device || 'no device named' }}{{ s.temperature ? ' · ' + s.temperature + ' °C' : '' }}{{ s.mode ? ' · ' + s.mode : '' }}</small></span>
                <span class="g num">{{ s.grams | number: '1.2-2' }} g</span>
                @if (s.known) {
                  <span class="mg"><b class="num">{{ s.thc_mg | number: '1.1-1' }} mg</b><small>{{ s.thc_pct }} % THC · {{ s.cbd_mg | number: '1.1-1' }} mg CBD</small></span>
                } @else {
                  <span class="mg dim"><b>?</b><small>no THC written down</small></span>
                }
                <span class="pct num">{{ share(s.thc_mg, d, d.thc_mg) | number: '1.0-0' }} %</span>
                <span class="stars" [attr.aria-label]="s.overall ? s.overall + ' of 5' : 'not rated'">{{ s.overall ? '★'.repeat(s.overall) : '—' }}</span>
              </li>
            }
          </ol>
        } @else {
          <div class="none">
            <mat-icon>wb_twilight</mat-icon>
            <div>
              <strong>No session yet today.</strong>
              @if (d.usual.days) {
                <span class="dim">A usual day of the last 30: {{ d.usual.seshed | number: '1.2-2' }} g, {{ d.usual.thc_mg | number: '1.0-0' }} mg THC.</span>
              }
              @if (d.ground) { <span class="dim">{{ d.ground | number: '1.2-2' }} g ground.</span> }
            </div>
            <button mat-stroked-button (click)="record()"><mat-icon>add</mat-icon>Record a session</button>
          </div>
        }
      </section>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: block; }
    .today { display: grid; gap: 18px; padding: 22px 26px;
      background: radial-gradient(80% 120% at 0% 0%, color-mix(in srgb, var(--w-sesh) 12%, transparent), transparent 60%), var(--w-glass); }
    header { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; }
    .eyebrow { display: flex; gap: 6px; align-items: center; }
    .again { justify-self: start; max-width: 100%; }
    .again .what { margin-left: 8px; font-weight: 400; opacity: .8; }
    @include bp.phone { .again .what { display: none; } }
    .note { display: inline-flex; gap: 4px; align-items: center; font-size: 12px; cursor: help; }
    .note mat-icon { font-size: 16px; width: 16px; height: 16px; }
    .top { display: grid; grid-template-columns: minmax(240px, 1.4fr) repeat(3, minmax(120px, 1fr)); gap: 24px; align-items: start; }
    .sum { display: grid; gap: 8px; }
    .v { font: 700 52px/1 var(--w-num); background: linear-gradient(120deg, var(--w-sesh), var(--w-accent)); -webkit-background-clip: text;
      background-clip: text; color: transparent; }
    .v small { font-size: 18px; font-weight: 600; }
    .vs { display: grid; gap: 4px; margin-top: 4px; }
    .vs .row { display: grid; grid-template-columns: 44px minmax(0, 1fr) 60px; gap: 8px; align-items: center; font-size: 12px; color: var(--w-dim); }
    .vs i { display: block; height: 8px; border-radius: 4px; background: color-mix(in srgb, var(--w-fg) 22%, transparent);
      transform-origin: left; animation: grow .9s cubic-bezier(.2,.8,.2,1) both; }
    .vs i.now { background: linear-gradient(90deg, var(--w-sesh), var(--w-accent)); }
    .vs b { color: var(--w-fg); text-align: right; }
    .vs small { font-size: 11px; }
    .warn { margin: 0; font-size: 12px; color: var(--w-warn); }
    .chips { display: flex; gap: 6px; flex-wrap: wrap; }
    .chips span { display: inline-flex; gap: 4px; align-items: center; padding: 3px 10px; border-radius: 999px; font-size: 12px;
      border: 1px solid var(--w-glass-edge); }
    .chips mat-icon { font-size: 15px; width: 15px; height: 15px; color: var(--w-stash); }
    figure { margin: 0; display: grid; gap: 4px; justify-items: center; }
    figure > :first-child { width: min(170px, 100%); }
    .legend { list-style: none; margin: 2px 0 0; padding: 0; display: grid; gap: 2px; font-size: 11px; color: var(--w-dim); width: 100%; }
    .legend li { display: grid; grid-template-columns: 8px minmax(0, 1fr) auto; gap: 6px; align-items: center; }
    .legend span { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
    .legend i { width: 8px; height: 8px; border-radius: 2px; }
    .legend b { color: var(--w-fg); }
    figcaption { font-size: 11px; color: var(--w-dim); text-transform: uppercase; letter-spacing: .1em; }

    .split { display: flex; gap: 3px; height: 12px; border-radius: 6px; overflow: hidden; }
    .split i { display: block; flex-basis: 0; min-width: 4px; transform-origin: left; animation: grow .8s cubic-bezier(.2,.8,.2,1) both; }
    .sessions { list-style: none; margin: 0; padding: 0; display: grid; gap: 6px; }
    .sessions li { display: grid; grid-template-columns: 50px minmax(0, 1fr) 64px 150px 48px 70px; gap: 12px; align-items: center;
      padding: 8px 12px; border-radius: 12px; border-left: 3px solid var(--tone);
      background: color-mix(in srgb, var(--tone) 6%, transparent); }
    .time { color: var(--w-dim); }
    .what, .mg { display: grid; min-width: 0; line-height: 1.3; }
    .what b, .what small, .mg small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    small { font-size: 11px; color: var(--w-dim); }
    .g, .pct { text-align: right; }
    .mg b { color: var(--tone); }
    .stars { color: var(--w-avb); letter-spacing: 1px; font-size: 13px; text-align: right; }
    .none { display: flex; gap: 16px; align-items: center; flex-wrap: wrap; }
    .none > mat-icon { font-size: 40px; width: 40px; height: 40px; color: var(--w-avb); }
    .none > div { display: grid; gap: 2px; flex: 1; min-width: 200px; }
    @keyframes grow { from { transform: scaleX(0); } }
    @include bp.phone {
      .today { padding: 18px 16px; }
      .top { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 12px; }
      .sum { grid-column: 1 / -1; }
      .top figure:last-child { grid-column: 1 / -1; }
      .top figure:last-child > :first-child { width: min(200px, 70%); }
      .v { font-size: 42px; }
      .sessions li { grid-template-columns: 44px minmax(0, 1fr) auto; gap: 4px 10px; }
      .sessions .g, .sessions .pct { display: none; }
      .sessions .mg { grid-column: 2; }
      .sessions .stars { grid-column: 3; grid-row: 1; }
    }
  `,
})
export class TodayCard {
  private readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);
  protected readonly busy = signal(false);

  /**
   * The last session, to have again in one tap: its strain, grams, device,
   * mode, heater and temperature — while the stash still holds its grams.
   */
  protected readonly again = computed(() => {
    const last = this.wits.journal.value().entries.find((e) => e.type === 'sesh' && !e.corrected);
    const jar = last && this.wits.jars.value().find((j) => j.slug === last.product);
    return last && jar && jar.stash >= last.grams ? last : null;
  });

  protected deviceAt(e: Entry): string {
    const name = this.wits.devices.value().find((d) => d.slug === e.device)?.name;
    return name ? ` · ${name}${e.temperature ? ` ${String(e.temperature)} °C` : ''}` : '';
  }

  protected async repeat(e: Entry): Promise<void> {
    this.busy.set(true);
    try {
      // The journal leaves out what a session did not say, so each is passed as it stands.
      const w = await this.wits.sesh({ product: e.product ?? '', grams: e.grams, device: e.device,
        temperature: e.temperature, mode: e.mode, heater: e.heater, setting: e.setting });
      this.snack.open(w.message, 'Undo', { duration: 8000 }).onAction().subscribe(() => {
        void this.wits.undo(w.entry.hash).then((u) => this.snack.open(u.message, undefined, { duration: 3000 }));
      });
    } catch (err) {
      this.snack.open((err as Error).message, 'OK', { duration: 6000 });
    } finally {
      this.busy.set(false);
    }
  }
  protected readonly day = httpResource<DayToday>(() => `/api/today?v=${String(this.wits.version())}`);
  protected readonly now = computed(() => {
    this.day.value();
    return new Date();
  });

  /** Each strain's colour, by its place among the day's strains, the most THC first. */
  private readonly tones = computed(() => new Map((this.day.value()?.strains ?? []).map((s, i) => [s.name, tone(i)])));

  protected readonly byStrain = computed<Slice[]>(() =>
    (this.day.value()?.strains ?? []).map((s) => ({ label: s.name, value: s.thc_mg, color: this.colour(s.name) })));

  protected readonly byDevice = computed<Slice[]>(() =>
    (this.day.value()?.devices ?? []).map((s, i) => ({ label: s.name, value: s.grams, color: tone(i + 2) })));

  protected readonly moments = computed<Moment[]>(() =>
    (this.day.value()?.sessions ?? []).map((s) => ({ at: s.at, size: s.thc_mg || s.grams * 100, color: this.colour(s.name),
      label: `${new Date(s.at).toTimeString().slice(0, 5)} ${s.name}, ${s.thc_mg.toFixed(1)} mg THC` })));

  protected colour(name: string): string {
    return this.tones().get(name) ?? 'var(--w-dim)';
  }

  /** A figure's share of the larger of today and a usual day, or of a whole when given. */
  protected share(v: number, d: DayToday, whole?: number): number {
    const of = whole ?? Math.max(d.thc_mg, d.usual.thc_mg, 1);
    return of > 0 ? (v / of) * 100 : 0;
  }

  protected record(): void {
    openRecord(this.dialog, { tab: 1 });
  }
}
