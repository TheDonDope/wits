import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { MatChipsModule } from '@angular/material/chips';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { Keys } from '../data/keys';
import { Dimension } from '../data/models';
import { Wits } from '../data/wits';
import { DayWheel } from '../viz/day-wheel';
import { EffectMatrix, scoreColor } from '../viz/effect-matrix';
import { GroupBars } from '../viz/group-bars';
import { StrainMap } from '../viz/strain-map';
import { ASKS } from './rate-queue';

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
    MatChipsModule,
    MatIconModule,
    RouterLink,
    StrainMap,
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

      @if (wits.strains.value().length) {
        <section class="glass pad">
          <div>
            <span class="eyebrow">a map of the strains</span>
            <p class="dim mapnote">Across by THC; up and down by name and terpenes, where they are written down
              (<code>wits describe --terpenes</code>), alike at one height. Coloured by how the sessions went; pick one
              for what is most like it.</p>
          </div>
          <w-strain-map [strains]="wits.strains.value()" />
        </section>
      }

      @if (log.rated.length) {
        <section class="glass pad">
          <span class="eyebrow">rated</span>
          @for (r of log.rated.slice(0, 30); track r.hash) {
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
        </section>
      }
    }
  `,
  styles: `
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
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
