import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

/** The colours a trip's jars take in turn: the theme's, so every theme speaks them. */
export const TRIP_TONES = ['var(--w-stash)', 'var(--w-storage)', 'var(--w-sesh)', 'var(--w-avb)', 'var(--w-accent)', 'var(--w-primary)'];

export interface TripLeg {
  name: string;
  days: number;
}

/**
 * A trip, day by day: what is packed laid along the days it lasts at the
 * rate, a jar a colour, from leaving home to coming back — and past that,
 * hatched, the margin.
 */
@Component({
  selector: 'w-trip-line',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="trip" role="img" [attr.aria-label]="aria()">
      <div class="ends"><span><b>leave</b></span><span [style.left.%]="back()"><b>back</b></span></div>
      <div class="fuel">
        @for (l of bars(); track l.name) {
          <span class="leg" [style.left.%]="l.at" [style.width.%]="l.w" [style.--tone]="l.tone" [style.--i]="$index" [title]="l.title"></span>
        }
        @if (margin() > 0) { <span class="margin" [style.left.%]="back()" [style.width.%]="margin()"></span> }
      </div>
      <div class="days">
        @for (d of ticks(); track d.n) { <span [style.left.%]="d.x" [class.home]="d.home">{{ d.n }}</span> }
      </div>
    </div>
  `,
  styles: `
    :host { display: block; }
    .trip { position: relative; display: grid; gap: 6px; }
    .ends { position: relative; height: 18px; font-size: 11px; color: var(--w-dim); text-transform: uppercase; letter-spacing: .08em; }
    .ends span { position: absolute; transform: translateX(-50%); }
    .ends span:first-child { left: 0; transform: none; }
    .fuel { position: relative; height: 34px; border-radius: 10px; overflow: hidden; background: color-mix(in srgb, var(--w-fg) 6%, transparent); }
    .leg { position: absolute; top: 0; bottom: 0; background: linear-gradient(180deg, var(--tone), color-mix(in srgb, var(--tone) 60%, transparent));
      border-right: 2px solid var(--w-ink); transform-origin: left; animation: fill .8s cubic-bezier(.2,.8,.2,1) both;
      animation-delay: calc(var(--i) * 90ms); }
    .margin { position: absolute; top: 0; bottom: 0; border-left: 2px dashed var(--w-fg);
      background: repeating-linear-gradient(135deg, color-mix(in srgb, var(--w-ink) 55%, transparent) 0 5px, transparent 5px 10px); }
    .days { position: relative; height: 16px; }
    .days span { position: absolute; transform: translateX(-50%); font: 10px var(--w-num); color: var(--w-dim); }
    .days .home { color: var(--w-fg); font-weight: 600; }
    @keyframes fill { from { transform: scaleX(0); } }
  `,
})
export class TripLine {
  readonly legs = input.required<TripLeg[]>();
  /** The days away. */
  readonly days = input.required<number>();

  /** The span drawn: the trip, or what is packed when it lasts longer. */
  private readonly span = computed(() => Math.max(this.days(), this.legs().reduce((t, l) => t + l.days, 0), 0.01));
  protected readonly back = computed(() => (this.days() / this.span()) * 100);
  protected readonly margin = computed(() => Math.max(100 - this.back(), 0));

  protected readonly bars = computed(() => {
    let at = 0;
    return this.legs().map((l, i) => {
      const w = (l.days / this.span()) * 100;
      const out = { name: l.name, at, w, tone: TRIP_TONES[i % TRIP_TONES.length],
        title: `${l.name}: ${l.days.toFixed(1)} days` };
      at += w;
      return out;
    });
  });

  /** A tick a day, fewer on a long trip; the day back always. */
  protected readonly ticks = computed(() => {
    const every = Math.ceil(this.days() / 14);
    const out: { n: number; x: number; home: boolean }[] = [];
    for (let n = every; n < this.days(); n += every) {
      out.push({ n, x: (n / this.span()) * 100, home: false });
    }
    out.push({ n: this.days(), x: this.back(), home: true });
    return out;
  });

  protected readonly aria = computed(() =>
    `${this.legs().map((l) => `${l.name} for ${l.days.toFixed(1)} days`).join(', ')}, for a trip of ${String(this.days())} days`);
}
