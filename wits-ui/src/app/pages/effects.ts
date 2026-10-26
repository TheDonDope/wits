import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatChipsModule } from '@angular/material/chips';
import { MatIconModule } from '@angular/material/icon';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import { Keys } from '../data/keys';
import { Dimension } from '../data/models';
import { Wits } from '../data/wits';
import { DayWheel } from '../viz/day-wheel';
import { EffectMatrix, scoreColor } from '../viz/effect-matrix';
import { GroupBars } from '../viz/group-bars';
import { StrainMap } from '../viz/strain-map';
import { TerpeneTable } from '../viz/terpene-table';
import { ASKS } from './rate-queue';

const VIEWS = [
  { key: 'works', label: 'What works', icon: 'psychology' },
  { key: 'strains', label: 'Strains', icon: 'bubble_chart' },
  { key: 'terpenes', label: 'Terpenes', icon: 'spa' },
  { key: 'ratings', label: 'Ratings', icon: 'rate_review' },
] as const;

type View = (typeof VIEWS)[number]['key'];

function asView(v: string | null): View {
  return VIEWS.find((x) => x.key === v)?.key ?? 'works';
}

/**
 * The effect log: what each session did, rated afterwards in a few taps (the
 * sessions waiting are under "I forgot to", RateQueue), and what that adds up to — which strains help with what, at which temperature,
 * at which time of day. Every mean says how many ratings it stands on.
 */
@Component({
  selector: 'w-effects',
  imports: [
    DatePipe,
    DecimalPipe,
    DayWheel,
    EffectMatrix,
    GroupBars,
    MatButtonModule,
    MatButtonToggleModule,
    MatChipsModule,
    MatIconModule,
    RouterLink,
    StrainMap,
    TerpeneTable,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (wits.effects.value(); as log) {
      <section class="glass head">
        <div>
          <span class="eyebrow">the effect log</span>
          <h1 class="num">{{ log.rated.length }} <span class="dim">rated · {{ log.unrated.length }} waiting</span></h1>
        </div>
        <p class="dim lede">What a session did, rated afterwards. The more you rate, the more the patterns below are yours rather than chance.</p>
      </section>

      @if (log.unrated.length) {
        <a class="glass waiting" routerLink="/forgot">
          <mat-icon>rate_review</mat-icon>
          <span>{{ log.unrated.length }} {{ log.unrated.length === 1 ? 'session waits' : 'sessions wait' }} for a rating, under <b>I forgot to</b>.</span>
          <mat-icon>chevron_right</mat-icon>
        </a>
      }

      <mat-button-toggle-group class="views" [value]="view()" (change)="show($event.value)" aria-label="What to show"
        hideSingleSelectionIndicator>
        @for (v of views; track v.key) {
          <mat-button-toggle [value]="v.key"><mat-icon>{{ v.icon }}</mat-icon><span>{{ v.label }}</span></mat-button-toggle>
        }
      </mat-button-toggle-group>

      @if (view() === 'works') {
      @if (log.insights.rated) {
        <section class="glass pad">
          <header>
            <span class="eyebrow">what works for you · by product</span>
            <span class="dim small">{{ log.insights.rated }} rated sessions · hatched cells stand on one rating</span>
          </header>
          <w-effect-matrix [groups]="log.insights.by_product" [dimensions]="log.dimensions" />
        </section>

        <section class="pick">
          <mat-chip-listbox [value]="dimension()" (change)="dimension.set($event.value ?? 'overall')" aria-label="Dimension">
            @for (d of log.dimensions; track d) {
              <mat-chip-option [value]="d">{{ asks[d] }}</mat-chip-option>
            }
          </mat-chip-listbox>
        </section>

        <section class="pair">
          <div class="glass pad">
            <span class="eyebrow">{{ asks[dimension()] }} · by temperature</span>
            <w-group-bars [groups]="log.insights.by_temperature" [dimension]="dimension()" />
          </div>
          <div class="glass pad">
            <span class="eyebrow">{{ asks[dimension()] }} · by time of day</span>
            <w-day-wheel [groups]="log.insights.by_time_of_day" [dimension]="dimension()" />
          </div>
        </section>
      } @else {
        <section class="glass empty">
          <mat-icon>psychology_alt</mat-icon>
          <p>Rate a few sessions and wits starts to show what helps — which strains, at which temperature, at which time of day.</p>
        </section>
      }
      }

      @if (view() === 'strains') {
      @if (wits.strains.value().length) {
        <section class="glass pad">
          <div>
            <span class="eyebrow">a map of the strains</span>
            <p class="dim mapnote">Laid out by terpene profile: strains that share their terpenes, the leading ones and the
              rarer ones most, sit together, whatever their name or strength. Marked by what each tastes of most, or by
              how its sessions went; pick one for what is most like it, and in what.</p>
          </div>
          <w-strain-map [strains]="wits.strains.value()" />
        </section>
      } @else {
        <section class="glass empty"><mat-icon>bubble_chart</mat-icon><p>No strain on the shelf has its terpenes written down yet.</p></section>
      }
      }

      @if (view() === 'terpenes') {
      @if (wits.shelfTerpenes.value().length) {
        <section class="glass pad">
          <div>
            <span class="eyebrow">the terpenes on the shelf</span>
            <p class="dim mapnote">Every terpene your products name, what it tastes of, how many name it and lead with it,
              and how their sessions went — the products it leads counting most.</p>
          </div>
          <w-terpene-table [terpenes]="wits.shelfTerpenes.value()" />
        </section>
      } @else {
        <section class="glass empty"><mat-icon>spa</mat-icon><p>No product names its terpenes yet.</p></section>
      }
      }

      @if (view() === 'ratings') {
      @if (log.rated.length) {
        <section class="glass pad">
          <span class="eyebrow">rated · newest first</span>
          @for (r of log.rated.slice(0, shown()); track r.hash) {
            <div class="rated">
              <span class="when dim">{{ r.session.occurred_at | date: 'd MMM' }}</span>
              <span class="what">{{ r.session.grams | number: '1.2-2' }} g {{ r.session.product_name }}
                @if (r.session.temperature) { <span class="dim">· {{ r.session.temperature }} °C</span> }</span>
              <span class="pills">
                @for (d of log.dimensions; track d) {
                  @if (r.scores[d]; as v) {
                    <span class="pill" [style.--c]="color(v)">{{ asks[d] }} <b>{{ v }}</b></span>
                  }
                }
              </span>
              @if (r.note) { <span class="note dim">“{{ r.note }}”</span> }
            </div>
          }
          @if (log.rated.length > shown()) {
            <button mat-button class="older" (click)="shown.set(shown() + 30)">
              <mat-icon>expand_more</mat-icon>{{ min(30, log.rated.length - shown()) }} older of {{ log.rated.length - shown() }}
            </button>
          }
        </section>
      } @else {
        <section class="glass empty"><mat-icon>rate_review</mat-icon><p>No session is rated yet.</p></section>
      }
      }
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .views { justify-self: start; max-width: 100%; }
    .views mat-icon { margin-right: 6px; vertical-align: -6px; font-size: 20px; }
    .older { justify-self: center; }
    @include bp.phone {
      .views { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); width: 100%; }
      .views mat-button-toggle { min-width: 0; }
      .views mat-icon { display: none; }
    }
    .mapnote { margin: 6px 0 0; font-size: 13px; }
    .head { padding: 18px 24px; display: flex; flex-wrap: wrap; gap: 18px; align-items: center; justify-content: space-between; }
    h1 { font: 600 28px var(--w-num); margin: 4px 0 0; }
    h1 .dim { font: 400 15px var(--w-font); }
    .lede { max-width: 460px; margin: 0; font-size: 13px; }
    .waiting { display: flex; align-items: center; gap: 12px; padding: 14px 20px; color: inherit; text-decoration: none; }
    .waiting span { flex: 1; }
    .waiting mat-icon:first-child { color: var(--w-primary); }
    .pad { padding: 20px 24px; display: grid; gap: 14px; }
    .pad header { display: flex; justify-content: space-between; align-items: baseline; flex-wrap: wrap; gap: 8px; }
    .small { font-size: 12px; }
    .pair { display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 20px; }
    .empty { padding: 36px; display: grid; justify-items: center; text-align: center; gap: 6px; }
    .empty mat-icon { font-size: 42px; width: 42px; height: 42px; color: var(--w-primary); }
    .rated { display: grid; grid-template-columns: 56px minmax(160px, 1fr) auto; gap: 4px 14px; align-items: center;
      padding: 8px 0; border-bottom: 1px solid color-mix(in srgb, var(--w-fg) 5%, transparent); }
    .pills { display: flex; gap: 6px; flex-wrap: wrap; justify-content: flex-end; }
    .pill { font-size: 11px; padding: 2px 8px; border-radius: 99px; border: 1px solid color-mix(in srgb, var(--c) 50%, transparent);
      background: color-mix(in srgb, var(--c) 14%, transparent); }
    .pill b { color: var(--c); }
    .note { grid-column: 2 / -1; font-size: 12px; font-style: italic; }
  `,
})
export class Effects {
  protected readonly wits = inject(Wits);
  protected readonly asks = ASKS;
  protected readonly color = scoreColor;
  protected readonly dimension = signal<Dimension>('overall');
  protected readonly dimensions = computed(() => this.wits.effects.value()?.dimensions ?? []);
  /** The page in four: what works, the strains' map, the terpenes, every rating. ?view= names one. */
  protected readonly views = VIEWS;
  private readonly router = inject(Router);
  protected readonly view = signal<View>(asView(inject(ActivatedRoute).snapshot.queryParamMap.get('view')));
  /** How many ratings are listed: thirty, and thirty more a tap. */
  protected readonly shown = signal(30);
  protected readonly min = Math.min;

  protected show(v: View): void {
    this.view.set(v);
    void this.router.navigate([], { queryParams: { view: v === 'works' ? null : v }, replaceUrl: true });
  }

  constructor() {
    inject(Keys).register(
      'Effects',
      () => [
        { keys: ['m'], help: `showing ${ASKS[this.dimension()]}`, run: () => {
          this.nextDimension();
        } },
      ],
      inject(DestroyRef),
    );
  }

  private nextDimension(): void {
    const ds = this.dimensions();
    if (ds.length) {
      this.dimension.set(ds[(ds.indexOf(this.dimension()) + 1) % ds.length]);
    }
  }
}
