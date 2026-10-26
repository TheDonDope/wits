import { DatePipe, DecimalPipe, PercentPipe } from '@angular/common';
import { httpResource } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatIconModule } from '@angular/material/icon';
import { ActivatedRoute, Router } from '@angular/router';

import { YearReview } from '../data/models';
import { Wits } from '../data/wits';
import { BreakLine } from '../viz/break-line';
import { CountUp } from '../viz/count-up';
import { Heatmap } from '../viz/heatmap';
import { PriceStrip } from '../viz/price-strip';
import { TempGauge } from '../viz/temp-gauge';
import { YearMonths } from '../viz/year-months';

/**
 * The year on one card, as wits review gathers it: the grams and the days,
 * the months, every day of it, the strains, the terpene most of it led with,
 * the longest break, the temperature and the strain rated best, the device
 * most used, and what it cost from the cheapest gram to the dearest. Every
 * figure is the year's own, to today in a year still running.
 */
@Component({
  selector: 'w-year',
  imports: [BreakLine, CountUp, DatePipe, DecimalPipe, Heatmap, MatButtonToggleModule, MatIconModule, PercentPipe, PriceStrip,
    TempGauge, YearMonths],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (review.value(); as r) {
      <section class="hero glass">
        <div class="glow" aria-hidden="true"></div>
        <div class="title">
          <span class="eyebrow">the year in wits <span class="scope">the year</span></span>
          <h1 class="num">{{ r.year }}</h1>
          <p class="dim">{{ r.from | date: 'd MMMM' }} to {{ r.to | date: 'd MMMM' }}{{ r.running ? ', and counting' : '' }}</p>
        </div>
        <div class="big">
          <span class="v num"><w-count [value]="r.ground" [decimals]="1" />&thinsp;g</span>
          <span class="dim">ground on <b>{{ r.grind_days }}</b> of {{ r.days }} days · {{ r.per_grind_day | number: '1.2-2' }} g an active day</span>
          <div class="pills">
            <span><mat-icon>local_pharmacy</mat-icon><b class="num">{{ r.pickups }}</b> {{ r.pickups === 1 ? 'prescription' : 'prescriptions' }}</span>
            <span><mat-icon>air</mat-icon><b class="num">{{ r.sessions | number }}</b> sessions</span>
            <span><mat-icon>spa</mat-icon><b class="num">{{ r.products }}</b> strains</span>
          </div>
        </div>
        @if (r.years.length > 1) {
          <mat-button-toggle-group [value]="r.year" (change)="pick($event.value)" aria-label="Year" hideSingleSelectionIndicator>
            @for (y of r.years; track y) { <mat-button-toggle [value]="y">{{ y }}</mat-button-toggle> }
          </mat-button-toggle-group>
        }
      </section>

      @if (!r.ground && !r.sessions) {
        <section class="glass card wide"><p class="dim">Nothing was ground in {{ r.year }}, and there were no sessions.</p></section>
      } @else {
        <div class="bento">
          <section class="glass card w8" style="--i: 0">
            <span class="eyebrow">month by month</span>
            <w-year-months [months]="r.months" [upTo]="r.running ? month(r.to) : 12" />
          </section>

          @if (r.busiest; as b) {
            <section class="glass card w4 busiest" style="--i: 1">
              <span class="eyebrow">the busiest day</span>
              <mat-icon class="big-icon">local_fire_department</mat-icon>
              <span class="v num">{{ b.ground | number: '1.2-2' }} g</span>
              <span>{{ b.date | date: 'EEEE, d MMMM' }}</span>
            </section>
          }

          <section class="glass card w12" style="--i: 2">
            <header><span class="eyebrow">every day of it</span><span class="dim">{{ r.grind_days }} days with grinding</span></header>
            <w-heatmap [days]="r.daily" [year]="r.year" tone="var(--w-stash)" />
          </section>

          <section class="glass card w7" style="--i: 3">
            <span class="eyebrow">the strains of the year</span>
            <ol class="podium">
              @for (s of r.strains.slice(0, 5); track s.name; let i = $index) {
                <li [class.first]="i === 0">
                  <span class="rank num">{{ i + 1 }}</span>
                  <span class="name">{{ s.name }}</span>
                  <span class="bar"><i [style.width.%]="(s.ground / r.strains[0].ground) * 100"></i></span>
                  <span class="g num">{{ s.ground | number: '1.1-1' }} g <small>{{ s.share | percent }}</small></span>
                </li>
              }
            </ol>
          </section>

          @if (r.terpene; as t) {
            <section class="glass card w5 terp" style="--i: 4">
              <span class="eyebrow">the terpene of the year</span>
              <div class="ring" [style.--p]="t.share">
                <svg viewBox="0 0 120 120" aria-hidden="true"><circle cx="60" cy="60" r="50" class="t" /><circle cx="60" cy="60" r="50" class="f" pathLength="100" /></svg>
                <span class="num">{{ t.share | percent }}</span>
              </div>
              <strong>{{ t.name }}</strong>
              <span class="dim">of the grams ground led with it</span>
              <div class="chips">@for (f of t.flavors ?? []; track f) { <span>{{ f }}</span> }</div>
            </section>
          }

          @if (r.break; as b) {
            <section class="glass card w6" style="--i: 5">
              <span class="eyebrow">the longest break</span>
              <span class="v num">{{ b.days }} <small>days</small></span>
              <span>{{ b.from | date: 'd MMMM' }} to {{ b.to | date: 'd MMMM' }}<span class="dim"> · {{ b.breaks }} {{ b.breaks === 1 ? 'break' : 'breaks' }} of a week or more in all</span></span>
              <w-break-line [year]="r.year" [days]="activeDays()" [from]="b.from" [to]="b.to" />
            </section>
          }

          @if (r.best_temperature; as b) {
            <section class="glass card w6 temp" style="--i: 6">
              <span class="eyebrow">the temperature rated best</span>
              <w-temp-gauge [band]="b.label" />
              <span class="v num">{{ b.label }}</span>
              <span class="dim">{{ b.mean | number: '1.1-1' }} overall, over {{ b.count }} ratings</span>
            </section>
          }

          @if (r.best_strain; as s) {
            <section class="glass card w6" style="--i: 7">
              <span class="eyebrow">rated best</span>
              <strong class="title-l">{{ s.label }}</strong>
              <span class="stars" [style.--r]="s.mean" [attr.aria-label]="s.mean + ' of 5'">★★★★★</span>
              <span class="dim">{{ s.mean | number: '1.1-1' }} overall, over {{ s.count }} rated sessions</span>
            </section>
          }

          @if (r.device; as d) {
            <section class="glass card w6" style="--i: 8">
              <span class="eyebrow">the device of the year</span>
              <mat-icon class="big-icon">air</mat-icon>
              <strong class="title-l">{{ d.name }}</strong>
              <span class="dim">{{ d.sessions | number }} sessions, {{ d.seshed | number: '1.2-2' }} g through it</span>
            </section>
          }

          @if (r.bought.length) {
            <section class="glass card w12" style="--i: 9">
              <header>
                <span class="eyebrow">what it cost</span>
                <span class="dim">{{ r.bought.length }} purchases with a price</span>
              </header>
              <div class="paid">
                <span class="v num">€{{ r.paid / 100 | number: '1.2-2' }}</span>
                <span class="dim">for {{ r.priced | number: '1.2-2' }} g · <b>€{{ r.per_gram / 100 | number: '1.2-2' }}</b> a gram</span>
              </div>
              <w-price-strip [bought]="r.bought" [perGram]="r.per_gram" />
              <div class="ends">
                @if (r.cheapest; as c) { <span><b class="good">cheapest</b> {{ c.name }}, {{ c.at | date: 'd MMM' }}</span> }
                @if (r.dearest; as d) { <span><b class="bad">dearest</b> {{ d.name }}, {{ d.at | date: 'd MMM' }}</span> }
              </div>
            </section>
          }
        </div>
      }
    } @else if (review.error()) {
      <section class="glass card"><p class="dim">The year could not be read.</p></section>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; gap: 20px; }
    .eyebrow { display: flex; gap: 8px; align-items: center; }
    .hero { position: relative; overflow: hidden; display: grid; grid-template-columns: auto 1fr; gap: 12px 40px; align-items: end; padding: 32px 36px; }
    .glow { position: absolute; inset: -40% -10% auto auto; width: 70%; aspect-ratio: 1; border-radius: 50%; pointer-events: none;
      background: radial-gradient(circle, color-mix(in srgb, var(--w-primary) 35%, transparent), transparent 65%);
      animation: breathe 9s ease-in-out infinite; }
    h1 { margin: 0; font: 700 clamp(84px, 13vw, 168px)/.85 var(--w-head); letter-spacing: -.05em;
      background: linear-gradient(120deg, var(--w-fg) 10%, var(--w-primary) 55%, var(--w-accent)); -webkit-background-clip: text;
      background-clip: text; color: transparent; }
    .big { display: grid; gap: 6px; position: relative; }
    .v { font: 600 44px/1 var(--w-num); }
    .v small { font-size: 18px; color: var(--w-dim); }
    .pills { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
    .pills span { display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; border-radius: 999px; font-size: 13px;
      border: 1px solid var(--w-glass-edge); background: color-mix(in srgb, var(--w-fg) 4%, transparent); }
    .pills mat-icon { font-size: 18px; width: 18px; height: 18px; color: var(--w-accent); }
    mat-button-toggle-group { grid-column: 1 / -1; justify-self: start; position: relative; }

    .bento { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); gap: 20px; }
    .card { display: grid; gap: 10px; align-content: start; padding: 22px 24px; animation: rise .7s cubic-bezier(.2,.8,.2,1) both;
      animation-delay: calc(var(--i, 0) * 80ms); }
    .card header { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; flex-wrap: wrap; }
    .w4 { grid-column: span 4; } .w5 { grid-column: span 5; } .w6 { grid-column: span 6; }
    .w7 { grid-column: span 7; } .w8 { grid-column: span 8; } .w12 { grid-column: span 12; }
    .big-icon { font-size: 40px; width: 40px; height: 40px; color: var(--w-avb); }
    .busiest { justify-items: start; align-content: center; }
    .title-l { font: 600 22px/1.2 var(--w-head); }

    .podium { list-style: none; margin: 0; padding: 0; display: grid; gap: 10px; }
    .podium li { display: grid; grid-template-columns: 28px minmax(0, 1fr) minmax(60px, 30%) auto; gap: 12px; align-items: center; }
    .rank { font-weight: 700; color: var(--w-dim); text-align: center; }
    .first .rank { width: 28px; height: 28px; border-radius: 50%; display: grid; place-items: center; color: var(--w-on-accent);
      background: linear-gradient(135deg, var(--w-avb), var(--w-warn)); }
    .first .name { font-weight: 600; }
    .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .bar { height: 8px; border-radius: 4px; background: color-mix(in srgb, var(--w-fg) 8%, transparent); overflow: hidden; }
    .bar i { display: block; height: 100%; border-radius: 4px; background: var(--w-stash); animation: grow 1s cubic-bezier(.2,.8,.2,1) both; transform-origin: left; }
    .g { white-space: nowrap; font-size: 13px; } .g small { color: var(--w-dim); }

    .terp { justify-items: center; text-align: center; }
    .ring { position: relative; width: 132px; display: grid; place-items: center; }
    .ring svg { width: 100%; transform: rotate(-90deg); }
    .ring circle { fill: none; stroke-width: 12; }
    .ring .t { stroke: color-mix(in srgb, var(--w-fg) 8%, transparent); }
    .ring .f { stroke: var(--w-accent); stroke-linecap: round; stroke-dasharray: calc(var(--p) * 100) 100;
      filter: drop-shadow(0 0 6px color-mix(in srgb, var(--w-accent) 60%, transparent)); }
    .ring span { position: absolute; font: 600 24px var(--w-num); }
    .terp strong { font: 600 26px var(--w-head); }
    .chips { display: flex; gap: 6px; flex-wrap: wrap; justify-content: center; }
    .chips span { padding: 3px 10px; border-radius: 999px; font-size: 12px; color: var(--w-accent);
      border: 1px solid color-mix(in srgb, var(--w-accent) 40%, transparent); }

    .temp { justify-items: center; text-align: center; }
    .temp w-temp-gauge { width: min(260px, 100%); }
    .stars { font-size: 30px; letter-spacing: 4px; width: max-content;
      background: linear-gradient(90deg, var(--w-avb) calc(var(--r) / 5 * 100%), color-mix(in srgb, var(--w-fg) 15%, transparent) 0);
      -webkit-background-clip: text; background-clip: text; color: transparent; }
    .paid { display: flex; gap: 14px; align-items: baseline; flex-wrap: wrap; }
    .ends { display: flex; justify-content: space-between; gap: 12px; flex-wrap: wrap; font-size: 13px; }
    .good { color: var(--w-good); } .bad { color: var(--w-warn); }
    @keyframes rise { from { opacity: 0; transform: translateY(18px); } }
    @keyframes grow { from { transform: scaleX(0); } }
    @keyframes breathe { 50% { transform: scale(1.12); opacity: .8; } }
    @include bp.phone {
      .hero { grid-template-columns: 1fr; padding: 22px 18px; }
      .v { font-size: 34px; }
      .bento { gap: 14px; }
      .w4, .w5, .w6, .w7, .w8, .w12 { grid-column: span 12; }
      .card { padding: 18px 16px; }
      .podium li { grid-template-columns: 24px minmax(0, 1fr) auto; }
      .podium .bar { grid-column: 2 / -1; grid-row: 2; }
    }
  `,
})
export class Year {
  private readonly wits = inject(Wits);
  private readonly router = inject(Router);
  /** The year asked for, ?year=; this one when none is. */
  protected readonly year = signal(Number(inject(ActivatedRoute).snapshot.queryParamMap.get('year')) || new Date().getFullYear());
  protected readonly review = httpResource<YearReview>(() => `/api/review?year=${String(this.year())}&v=${String(this.wits.version())}`);
  protected readonly activeDays = computed(() => this.review.value()?.daily.map((d) => d.date) ?? []);

  protected pick(year: number): void {
    this.year.set(year);
    void this.router.navigate([], { queryParams: { year }, replaceUrl: true });
  }

  /** The months so far in a year still running: through the month of its last day. */
  protected month(day: string): number {
    return Number(day.slice(5, 7));
  }
}
