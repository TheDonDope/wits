import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, ElementRef, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatChipsModule } from '@angular/material/chips';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';

import { Keys } from '../data/keys';
import { Dimension, Entry } from '../data/models';
import { Wits } from '../data/wits';
import { DayWheel } from '../viz/day-wheel';
import { EffectMatrix, scoreColor } from '../viz/effect-matrix';
import { GroupBars } from '../viz/group-bars';
import { ScoreDots } from '../viz/score-dots';

/** What each dimension asks, in a word a tap can answer. */
const ASKS: Record<Dimension, string> = {
  overall: 'overall',
  pain: 'pain relief',
  sleep: 'sleep',
  calm: 'calm',
  mood: 'mood',
  focus: 'focus',
  appetite: 'appetite',
};

/**
 * The effect log: what each session did, rated afterwards in a few taps, and
 * what that adds up to — which strains help with what, at which temperature,
 * at which time of day. Every mean says how many ratings it stands on.
 */
@Component({
  selector: 'w-effects',
  imports: [
    DatePipe,
    DecimalPipe,
    DayWheel,
    EffectMatrix,
    FormsModule,
    GroupBars,
    MatButtonModule,
    MatChipsModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    ScoreDots,
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
        <section class="queue">
          <span class="eyebrow">how did it go?</span>
          <div class="cards">
            @for (s of everyWaiting() ? log.unrated : log.unrated.slice(0, 3); track s.hash; let i = $index) {
              <article class="card glass" [attr.id]="'rate-' + i">
                <header>
                  <div>
                    <strong>{{ s.grams | number: '1.2-2' }} g {{ s.product_name }}</strong>
                    <span class="dim">{{ s.occurred_at | date: 'EEE d MMM · HH:mm' }}
                      @if (s.device) { · {{ s.device }} } @if (s.temperature) { · {{ s.temperature }} °C }</span>
                  </div>
                </header>
                <div class="dims">
                  @for (d of log.dimensions; track d) {
                    <span class="ask" [class.lead]="d === 'overall'">{{ asks[d] }}</span>
                    <w-score-dots [label]="asks[d]" [value]="draft(s.hash)[d] ?? null" (valueChange)="set(s.hash, d, $event)"
                      [style.--tone]="d === 'overall' ? 'var(--w-mint)' : 'var(--w-violet)'" />
                  }
                </div>
                <mat-form-field appearance="outline" subscriptSizing="dynamic">
                  <mat-label>A line about it</mat-label>
                  <input matInput [ngModel]="notes()[s.hash] ?? ''" (ngModelChange)="note(s.hash, $event)" />
                </mat-form-field>
                <button mat-flat-button (click)="save(s)" [disabled]="!scored(s.hash) || busy()">
                  <mat-icon>done</mat-icon> Save
                </button>
              </article>
            }
          </div>
          @if (log.unrated.length > 3) {
            <button mat-button class="more" (click)="everyWaiting.set(!everyWaiting())">
              <mat-icon>{{ everyWaiting() ? 'expand_less' : 'expand_more' }}</mat-icon>
              {{ everyWaiting() ? 'Just the newest' : 'Rate ' + (log.unrated.length - 3) + ' older' }}
            </button>
          }
        </section>
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
    :host { display: grid; gap: 20px; }
    .head { padding: 18px 24px; display: flex; flex-wrap: wrap; gap: 18px; align-items: center; justify-content: space-between; }
    h1 { font: 600 28px var(--w-num); margin: 4px 0 0; }
    h1 .dim { font: 400 15px 'Inter Variable', sans-serif; }
    .lede { max-width: 460px; margin: 0; font-size: 13px; }
    .queue { display: grid; gap: 12px; }
    .more { justify-self: center; }
    .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(330px, 1fr)); gap: 18px; }
    .card { padding: 18px 20px; display: grid; gap: 14px; }
    .card header div { display: grid; gap: 2px; }
    .dims { display: grid; grid-template-columns: auto 1fr; gap: 9px 16px; align-items: center; }
    .ask { font-size: 13px; color: var(--w-dim); }
    .ask.lead { color: var(--mat-sys-on-surface); font-weight: 600; }
    .pad { padding: 20px 24px; display: grid; gap: 14px; }
    .pad header { display: flex; justify-content: space-between; align-items: baseline; flex-wrap: wrap; gap: 8px; }
    .small { font-size: 12px; }
    .pair { display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 20px; }
    .empty { padding: 36px; display: grid; justify-items: center; text-align: center; gap: 6px; }
    .empty mat-icon { font-size: 42px; width: 42px; height: 42px; color: var(--w-violet); }
    .rated { display: grid; grid-template-columns: 56px minmax(160px, 1fr) auto; gap: 4px 14px; align-items: center;
      padding: 8px 0; border-bottom: 1px solid rgb(255 255 255 / 5%); }
    .pills { display: flex; gap: 6px; flex-wrap: wrap; justify-content: flex-end; }
    .pill { font-size: 11px; padding: 2px 8px; border-radius: 99px; border: 1px solid color-mix(in srgb, var(--c) 50%, transparent);
      background: color-mix(in srgb, var(--c) 14%, transparent); }
    .pill b { color: var(--c); }
    .note { grid-column: 2 / -1; font-size: 12px; font-style: italic; }
  `,
})
export class Effects {
  protected readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  protected readonly asks = ASKS;
  protected readonly color = scoreColor;
  protected readonly dimension = signal<Dimension>('overall');
  protected readonly busy = signal(false);
  /** Whether every waiting session is offered, or only the newest three. */
  protected readonly everyWaiting = signal(false);

  /** Scores being given, per session, before they are saved. */
  private readonly drafts = signal<Record<string, Partial<Record<Dimension, number>>>>({});
  protected readonly notes = signal<Record<string, string>>({});

  protected readonly dimensions = computed(() => this.wits.effects.value()?.dimensions ?? []);

  constructor() {
    inject(Keys).register(
      'Effects',
      () => [
        { keys: ['m'], help: `showing ${ASKS[this.dimension()]}`, run: () => {
          this.nextDimension();
        } },
        { keys: ['n'], help: 'next to rate', run: () => {
          this.host.nativeElement.querySelector<HTMLElement>('#rate-0 button')?.focus();
        } },
      ],
      inject(DestroyRef),
    );
  }

  protected draft(hash: string): Partial<Record<Dimension, number>> {
    return this.drafts()[hash] ?? {};
  }

  protected scored(hash: string): boolean {
    return Object.keys(this.draft(hash)).length > 0;
  }

  protected set(hash: string, d: Dimension, v: number | null): void {
    this.drafts.update((all) => {
      const kept = Object.entries(all[hash] ?? {}).filter(([k]) => k !== d);
      const next = Object.fromEntries(v === null ? kept : [...kept, [d, v]]) as Partial<Record<Dimension, number>>;
      return { ...all, [hash]: next };
    });
  }

  protected note(hash: string, text: string): void {
    this.notes.update((all) => ({ ...all, [hash]: text }));
  }

  protected async save(s: Entry): Promise<void> {
    this.busy.set(true);
    try {
      const out = await this.wits.rate(s.hash, this.draft(s.hash), this.notes()[s.hash] ?? '');
      this.snack.open(out.message, undefined, { duration: 3000 });
    } catch (err) {
      this.snack.open((err as Error).message, 'OK', { duration: 6000 });
    } finally {
      this.busy.set(false);
    }
  }

  private nextDimension(): void {
    const ds = this.dimensions();
    if (ds.length) {
      this.dimension.set(ds[(ds.indexOf(this.dimension()) + 1) % ds.length]);
    }
  }
}
