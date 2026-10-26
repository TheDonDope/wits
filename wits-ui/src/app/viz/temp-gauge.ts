import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

const LO = 150;
const HI = 230;
const R = 80;

/**
 * A vaporizer's dial, 150 to 230 °C, with one ten-degree band lit: the band
 * the year's sessions were rated best at.
 */
@Component({
  selector: 'w-temp-gauge',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="-100 -96 200 116" role="img" [attr.aria-label]="'Rated best at ' + band()">
      <defs>
        <linearGradient id="tg-heat" x1="0" x2="1">
          <stop offset="0" stop-color="var(--w-storage)" /><stop offset=".55" stop-color="var(--w-avb)" />
          <stop offset="1" stop-color="var(--w-warn)" />
        </linearGradient>
      </defs>
      <path [attr.d]="arc(LO, HI)" class="track" />
      <path [attr.d]="arc(LO, HI)" class="heat" />
      <path [attr.d]="arc(span()[0], span()[1])" class="lit" />
      @for (t of ticks; track t) {
        <text [attr.x]="at(t, R + 14)[0]" [attr.y]="at(t, R + 14)[1]" text-anchor="middle">{{ t }}</text>
      }
      <line x1="0" y1="0" [attr.x2]="needle()[0]" [attr.y2]="needle()[1]" class="needle" />
      <circle r="6" class="hub" />
    </svg>
  `,
  styles: `
    :host { display: block; }
    svg { display: block; width: 100%; overflow: visible; }
    .track { fill: none; stroke: color-mix(in srgb, var(--w-fg) 8%, transparent); stroke-width: 16; stroke-linecap: round; }
    .heat { fill: none; stroke: url(#tg-heat); stroke-width: 4; opacity: .5; stroke-linecap: round; }
    .lit { fill: none; stroke: var(--w-avb); stroke-width: 16; stroke-linecap: round;
      filter: drop-shadow(0 0 8px color-mix(in srgb, var(--w-avb) 80%, transparent)); }
    text { fill: var(--w-dim); font: 9px var(--w-num); }
    .needle { stroke: var(--w-fg); stroke-width: 3; stroke-linecap: round; transform-origin: 0 0;
      animation: swing 1.2s cubic-bezier(.3,1.4,.5,1) both; }
    .hub { fill: var(--w-fg); }
    @keyframes swing { from { transform: rotate(-80deg); } }
  `,
})
export class TempGauge {
  /** The band as the effect log labels it, "180–189 °C". */
  readonly band = input.required<string>();
  protected readonly LO = LO;
  protected readonly HI = HI;
  protected readonly R = R;
  protected readonly ticks = [150, 170, 190, 210, 230];

  protected readonly span = computed(() => {
    const lo = Number(/\d+/.exec(this.band())?.[0] ?? LO);
    return [Math.max(lo, LO), Math.min(lo + 10, HI)];
  });

  protected readonly needle = computed(() => this.at((this.span()[0] + this.span()[1]) / 2, R - 18));

  /** A temperature's point on the dial, a half circle from left to right. */
  protected at(t: number, r: number): [number, number] {
    const a = Math.PI * (1 - (t - LO) / (HI - LO));
    return [r * Math.cos(a), -r * Math.sin(a)];
  }

  protected arc(from: number, to: number): string {
    const [x1, y1] = this.at(from, R);
    const [x2, y2] = this.at(to, R);
    return `M${x1.toFixed(1)} ${y1.toFixed(1)} A${R} ${R} 0 0 1 ${x2.toFixed(1)} ${y2.toFixed(1)}`;
  }
}
