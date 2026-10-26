import { DecimalPipe } from '@angular/common';
import { httpResource } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatIconModule } from '@angular/material/icon';
import { MatSliderModule } from '@angular/material/slider';
import { DomSanitizer } from '@angular/platform-browser';

import { Packing } from '../data/models';
import { Wits } from '../data/wits';
import { CountUp } from '../viz/count-up';
import { GlassJar } from '../viz/glass-jar';
import { TRIP_TONES, TripLine } from '../viz/trip-line';

/** Trips people take, a tap each. */
const PRESETS = [
  { days: 3, label: 'Weekend' },
  { days: 7, label: 'A week' },
  { days: 10, label: '10 days' },
  { days: 14, label: 'Two weeks' },
  { days: 21, label: 'Three weeks' },
];

/**
 * Packing for a trip, as wits pack plans it: the grams for the days away at
 * the refill planner's rate with a margin, drawn evenly from the jars on the
 * shelf so they go down together — or oldest first — each jar's ground grams
 * before its sealed ones; the trip day by day; each jar with what it gives;
 * and a statement signed for exactly what is packed. Nothing is recorded.
 */
@Component({
  selector: 'w-trip',
  imports: [CountUp, DecimalPipe, GlassJar, MatButtonModule, MatButtonToggleModule, MatIconModule, MatSliderModule, TripLine],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="glass head">
      <div class="ask">
        <span class="eyebrow">packing for a trip</span>
        <h1><span class="num">{{ days() }}</span> {{ days() === 1 ? 'day' : 'days' }} away</h1>
        <div class="presets">
          @for (p of presets; track p.days) {
            <button mat-stroked-button [class.on]="p.days === days()" (click)="days.set(p.days)">{{ p.label }}</button>
          }
        </div>
      </div>
      <div class="knobs">
        <label><span>Days away <b class="num">{{ days() }}</b></span>
          <mat-slider min="1" max="42" step="1" discrete><input matSliderThumb [value]="days()" (valueChange)="days.set($event)" aria-label="Days away" /></mat-slider>
        </label>
        <label><span>To spare <b class="num">{{ margin() }} %</b></span>
          <mat-slider min="0" max="50" step="5" discrete><input matSliderThumb [value]="margin()" (valueChange)="margin.set($event)" aria-label="Margin" /></mat-slider>
        </label>
        <mat-button-toggle-group [value]="mode()" (change)="mode.set($event.value)" aria-label="Draw from" hideSingleSelectionIndicator>
          <mat-button-toggle value="even"><mat-icon>balance</mat-icon> Evenly</mat-button-toggle>
          <mat-button-toggle value="oldest"><mat-icon>history</mat-icon> Oldest first</mat-button-toggle>
        </mat-button-toggle-group>
      </div>
    </section>

    @if (plan.value(); as p) {
      <section class="glass take">
        <mat-icon class="bag">luggage</mat-icon>
        <div>
          <span class="eyebrow">take <span class="scope">the shelf</span></span>
          <span class="v num"><w-count [value]="p.packed" [decimals]="2" [duration]="500" />&thinsp;g</span>
          @if (p.short) {
            <p class="warn">The shelf holds only {{ p.shelf | number: '1.2-2' }} g: {{ p.packed / p.rate | number: '1.1-1' }} days' worth of the
              {{ p.need | number: '1.2-2' }} g the trip wants. Take it all, and ask for the next prescription before you go.</p>
          } @else {
            <p class="dim">{{ p.days }} days at {{ p.rate | number: '1.2-2' }} g a day — this fill's rate with grinding — and
              {{ p.margin }} % to spare, from {{ p.lines.length }} {{ p.lines.length === 1 ? 'jar' : 'jars' }}.</p>
          }
        </div>
      </section>

      <section class="glass pad">
        <header><span class="eyebrow">the trip, day by day</span><span class="dim">a jar a colour; hatched, the margin</span></header>
        <w-trip-line [legs]="legs()" [days]="p.days" />
        <div class="legend">
          @for (l of p.lines; track l.name; let i = $index) {
            <span><i [style.background]="tone(i)"></i>{{ l.name }} <b class="num">{{ l.days | number: '1.1-1' }} d</b></span>
          }
        </div>
      </section>

      <section class="glass pad">
        <header>
          <span class="eyebrow">from the jars</span>
          <span class="key"><i class="packed"></i>packed <i style="background: var(--w-stash)"></i>stays ground <i style="background: var(--w-storage)"></i>stays sealed</span>
        </header>
        <div class="jars">
          @for (l of p.lines; track l.name; let i = $index) {
            <figure [style.--i]="i">
              <w-jar [capacity]="most()" [layers]="layers(l)" />
              <figcaption>
                <strong>{{ l.name }}</strong>
                <span class="num take-g">{{ l.stash + l.sealed | number: '1.2-2' }} g</span>
                <span class="dim">
                  @if (l.stash) { {{ l.stash | number: '1.2-2' }} ground }
                  @if (l.stash && l.sealed) { + }
                  @if (l.sealed) { <b class="grind">{{ l.sealed | number: '1.2-2' }} to grind</b> }
                </span>
                <span class="dim">leaves {{ l.held_stash + l.held_sealed - l.stash - l.sealed | number: '1.2-2' }} g</span>
              </figcaption>
            </figure>
          }
        </div>
        @if (toGrind() > 0) {
          <p class="note"><mat-icon>grain</mat-icon>Grind {{ toGrind() | number: '1.2-2' }} g before you go; nothing here is recorded until you do.</p>
        }
      </section>

      @if (p.qr) {
        <section class="glass pad sign">
          <div class="qr" [innerHTML]="qr()"></div>
          <div class="said">
            <span class="eyebrow">what I carry, signed</span>
            <strong>Signed for exactly {{ p.packed | number: '1.2-2' }} g</strong>
            <p class="dim">The statement names each jar packed and the fill that dispensed it, against the journal's head. Scanned with
              any phone, it opens a page that checks the signature there; nothing in it is sent to a server.</p>
            <code>{{ p.fingerprint }}</code>
            <a mat-stroked-button [href]="qrFile()" download="wits-trip.svg"><mat-icon>download</mat-icon>The QR code</a>
          </div>
        </section>
      }
    } @else if (plan.error(); as e) {
      <section class="glass pad"><p class="warn">{{ message(e) }}</p></section>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; gap: 20px; }
    .head { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 24px; padding: 24px 28px; align-items: center;
      background: radial-gradient(90% 140% at 100% 0%, color-mix(in srgb, var(--w-accent) 14%, transparent), transparent 60%), var(--w-glass); }
    .ask { display: grid; gap: 10px; }
    h1 { margin: 0; font: 600 34px var(--w-head); }
    h1 .num { font-size: 52px; background: linear-gradient(120deg, var(--w-primary), var(--w-accent)); -webkit-background-clip: text;
      background-clip: text; color: transparent; }
    .presets { display: flex; gap: 6px; flex-wrap: wrap; }
    .presets .on { border-color: var(--w-primary); color: var(--w-primary); }
    .knobs { display: grid; gap: 6px; }
    .knobs label { display: grid; font-size: 13px; color: var(--w-dim); }
    .knobs label span { display: flex; justify-content: space-between; }
    .knobs label b { color: var(--w-fg); }
    mat-slider { width: 100%; margin: 0; }
    mat-button-toggle-group { justify-self: start; }
    mat-button-toggle mat-icon { margin-right: 4px; vertical-align: -6px; font-size: 20px; }

    .take { display: flex; gap: 22px; align-items: center; padding: 22px 28px; }
    .bag { font-size: 64px; width: 64px; height: 64px; color: var(--w-accent); flex: none;
      filter: drop-shadow(0 0 18px color-mix(in srgb, var(--w-accent) 50%, transparent)); }
    .take .v { display: block; font: 700 56px/1.05 var(--w-num); }
    .take p { margin: 4px 0 0; max-width: 62ch; }
    .warn { color: var(--w-warn); }
    .pad { display: grid; gap: 14px; padding: 20px 24px; }
    .pad header { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; flex-wrap: wrap; }
    .legend, .key { display: flex; gap: 6px 16px; flex-wrap: wrap; font-size: 13px; align-items: center; }
    .legend i, .key i { display: inline-block; width: 10px; height: 10px; border-radius: 3px; margin-right: 6px; }
    .key { color: var(--w-dim); font-size: 12px; }
    .packed { background: var(--w-primary); box-shadow: 0 0 6px var(--w-primary); }

    .jars { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 18px; }
    figure { margin: 0; display: grid; gap: 8px; justify-items: center; text-align: center; animation: up .6s cubic-bezier(.2,.8,.2,1) both;
      animation-delay: calc(var(--i) * 70ms); }
    w-jar { width: 96px; height: 134px; }
    figcaption { display: grid; gap: 2px; font-size: 12px; }
    figcaption strong { font-size: 13px; font-weight: 600; }
    .take-g { font: 600 20px var(--w-num); color: var(--w-primary); }
    .grind { color: var(--w-storage); font-weight: 600; }
    .note { display: flex; gap: 8px; align-items: center; margin: 0; font-size: 13px; color: var(--w-dim); }
    .note mat-icon { color: var(--w-stash); }

    .sign { grid-template-columns: 220px minmax(0, 1fr); align-items: center; gap: 24px; }
    .qr { background: #fff; border-radius: 14px; overflow: hidden; box-shadow: 0 10px 30px -10px rgb(0 0 0 / 50%); }
    .qr :is(svg) { display: block; width: 100%; height: auto; }
    .said { display: grid; gap: 8px; justify-items: start; }
    .said strong { font: 600 20px var(--w-head); }
    .said p { margin: 0; max-width: 60ch; }
    code { font: 12px var(--w-mono); color: var(--w-dim); overflow-wrap: anywhere; }
    @keyframes up { from { opacity: 0; transform: translateY(14px); } }
    @include bp.phone {
      .head { grid-template-columns: minmax(0, 1fr); padding: 18px 16px; }
      .take { padding: 18px 16px; gap: 14px; }
      .bag { font-size: 44px; width: 44px; height: 44px; }
      .take .v { font-size: 40px; }
      .pad { padding: 16px; }
      .jars { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .sign { grid-template-columns: minmax(0, 1fr); }
      .qr { width: min(240px, 100%); justify-self: center; }
    }
  `,
})
export class Trip {
  private readonly wits = inject(Wits);
  private readonly sanitizer = inject(DomSanitizer);
  protected readonly presets = PRESETS;
  protected readonly days = signal(7);
  protected readonly margin = signal(20);
  protected readonly mode = signal<'even' | 'oldest'>('even');
  protected readonly plan = httpResource<Packing>(() =>
    `/api/pack?days=${String(this.days())}&margin=${String(this.margin())}&mode=${this.mode()}&v=${String(this.wits.version())}`);

  protected readonly legs = computed(() => (this.plan.value()?.lines ?? []).map((l) => ({ name: l.name, days: l.days })));
  /** The fullest jar, which every jar is drawn against, so their sizes compare. */
  protected readonly most = computed(() => Math.max(...(this.plan.value()?.lines ?? []).map((l) => l.held_stash + l.held_sealed), 1));
  protected readonly toGrind = computed(() => (this.plan.value()?.lines ?? []).reduce((t, l) => t + l.sealed, 0));
  protected readonly qr = computed(() => this.sanitizer.bypassSecurityTrustHtml(this.plan.value()?.qr ?? ''));
  protected readonly qrFile = computed(() =>
    this.sanitizer.bypassSecurityTrustUrl(`data:image/svg+xml;charset=utf-8,${encodeURIComponent(this.plan.value()?.qr ?? '')}`));

  /** A jar, bottom up: what stays sealed, the sealed packed, what stays ground, the ground packed. */
  protected layers(l: Packing['lines'][number]) {
    return [
      { grams: l.held_sealed - l.sealed, color: 'var(--w-storage)', label: 'stays sealed' },
      { grams: l.sealed, color: 'var(--w-primary)', label: 'packed, to grind' },
      { grams: l.held_stash - l.stash, color: 'var(--w-stash)', label: 'stays ground' },
      { grams: l.stash, color: 'var(--w-primary)', label: 'packed, ground' },
    ];
  }

  protected tone(i: number): string {
    return TRIP_TONES[i % TRIP_TONES.length];
  }

  /** The server's words, when it says why there is no plan. */
  protected message(e: unknown): string {
    const err = (e as { error?: { error?: string } }).error;
    return err?.error ?? 'The plan could not be made.';
  }
}
