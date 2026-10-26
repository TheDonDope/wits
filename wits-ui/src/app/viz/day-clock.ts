import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export interface Moment {
  at: string;
  /** How big its dot is drawn, relative to the others. */
  size: number;
  color: string;
  label: string;
}

const R = 64;

/**
 * The day as a clock of 24 hours, midnight at the top: each session a dot at
 * its hour, sized by what was in it, and the hand at now.
 */
@Component({
  selector: 'w-day-clock',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="-86 -86 172 172" role="img" [attr.aria-label]="aria()">
      <circle [attr.r]="R" class="face" />
      @for (h of hours; track h) {
        <line [attr.x1]="pt(h, R - 4)[0]" [attr.y1]="pt(h, R - 4)[1]" [attr.x2]="pt(h, R + (h % 6 ? 0 : 4))[0]"
          [attr.y2]="pt(h, R + (h % 6 ? 0 : 4))[1]" class="tick" />
      }
      @for (h of [0, 6, 12, 18]; track h) {
        <text [attr.x]="pt(h, R + 14)[0]" [attr.y]="pt(h, R + 14)[1] + 3" text-anchor="middle">{{ h }}</text>
      }
      @if (hand(); as n) {
        <path [attr.d]="sweep()" class="past" />
        <line x1="0" y1="0" [attr.x2]="n[0]" [attr.y2]="n[1]" class="hand" />
      }
      @for (d of dots(); track $index) {
        <circle [attr.cx]="d.x" [attr.cy]="d.y" [attr.r]="d.r" [attr.fill]="d.color" class="dot" [style.--i]="$index"><title>{{ d.label }}</title></circle>
      }
      <circle r="3" class="hub" />
    </svg>
  `,
  styles: `
    :host { display: block; }
    svg { display: block; width: 100%; overflow: visible; }
    .face { fill: color-mix(in srgb, var(--w-fg) 3%, transparent); stroke: var(--w-glass-edge); }
    .tick { stroke: color-mix(in srgb, var(--w-fg) 25%, transparent); stroke-width: 1.2; }
    text { fill: var(--w-dim); font: 9px var(--w-num); }
    .past { fill: color-mix(in srgb, var(--w-primary) 10%, transparent); }
    .hand { stroke: var(--w-primary); stroke-width: 2; stroke-linecap: round; }
    .hub { fill: var(--w-primary); }
    .dot { stroke: var(--w-ink); stroke-width: 1.5; opacity: .92; transform-box: fill-box; transform-origin: center;
      animation: land .6s cubic-bezier(.3,1.4,.5,1) both; animation-delay: calc(var(--i) * 80ms + 200ms); }
    @keyframes land { from { transform: scale(0); } }
  `,
})
export class DayClock {
  readonly moments = input.required<Moment[]>();
  /** Now, when the clock shows today; null for a day gone by. */
  readonly now = input<Date | null>(null);
  protected readonly R = R;
  protected readonly hours = Array.from({ length: 24 }, (_, h) => h);

  /** A point at hour h (fractional), r out from the middle. */
  protected pt(h: number, r: number): [number, number] {
    const a = (h / 24) * 2 * Math.PI - Math.PI / 2;
    return [r * Math.cos(a), r * Math.sin(a)];
  }

  private hourOf(d: Date): number {
    return d.getHours() + d.getMinutes() / 60;
  }

  protected readonly hand = computed(() => {
    const n = this.now();
    return n ? this.pt(this.hourOf(n), R - 10) : null;
  });

  /** The part of the day gone, a wedge from midnight to now. */
  protected readonly sweep = computed(() => {
    const n = this.now();
    if (!n) {
      return '';
    }
    const h = this.hourOf(n);
    const [x, y] = this.pt(h, R);
    return `M0 0 L0 ${String(-R)} A${String(R)} ${String(R)} 0 ${h > 12 ? 1 : 0} 1 ${x.toFixed(1)} ${y.toFixed(1)} Z`;
  });

  protected readonly dots = computed(() => {
    const most = Math.max(...this.moments().map((m) => m.size), 0.01);
    return this.moments().map((m, i) => {
      // Sessions close in time are nudged out a little, so each dot shows.
      const [x, y] = this.pt(this.hourOf(new Date(m.at)), R - 22 + (i % 3) * 8);
      return { x, y, r: 4 + Math.sqrt(m.size / most) * 8, color: m.color, label: m.label };
    });
  });

  protected readonly aria = computed(() => this.moments().map((m) => m.label).join('; '));
}
