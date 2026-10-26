import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { FAMILIES } from '../data/aroma';

/** A point on the wheel, and where its spoke's label goes. */
interface Spoke {
  id: string;
  label: string;
  x: number;
  y: number;
  lx: number;
  ly: number;
  anchor: string;
  value: number;
}

const R = 70;
const LABEL = 86;

/**
 * A profile's taste as a wheel: the eight families round it, from the bright
 * to the deep, each spoke as long as that family is strong — the strongest
 * reaches the rim, and the rest on a square-root scale, so the second and
 * third notes a taste names show, not only the first. A second profile, when given, is drawn over it in outline,
 * to set two strains side by side.
 */
@Component({
  selector: 'w-aroma-wheel',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="-120 -104 240 208" role="img" [attr.aria-label]="label()">
      @for (ring of rings; track ring) {
        <circle class="ring" [attr.r]="ring * R" />
      }
      @for (s of spokes(); track s.id) {
        <line class="spoke" x1="0" y1="0" [attr.x2]="s.x" [attr.y2]="s.y" />
        <text class="name" [attr.x]="s.lx" [attr.y]="s.ly" [attr.text-anchor]="s.anchor" dominant-baseline="middle"
          [class.strong]="s.value >= 0.5">{{ s.label }}</text>
      }
      @if (other(); as o) {
        <polygon class="other" [attr.points]="o" />
      }
      <polygon class="mine" [attr.points]="mine()" />
      @for (p of points(); track p.id) {
        @if (p.value > 0) {
          <circle class="dot" [attr.cx]="p.px" [attr.cy]="p.py" r="2.5"><title>{{ p.label }}: {{ (p.value * 100).toFixed(0) }} %</title></circle>
        }
      }
    </svg>
  `,
  styles: `
    :host { display: block; }
    svg { display: block; width: 100%; height: auto; overflow: visible; }
    .ring { fill: none; stroke: color-mix(in srgb, var(--w-fg) 8%, transparent); }
    .spoke { stroke: color-mix(in srgb, var(--w-fg) 10%, transparent); }
    .name { fill: var(--w-dim); font: 500 9.5px var(--w-font); }
    .name.strong { fill: var(--mat-sys-on-surface); }
    .mine { fill: color-mix(in srgb, var(--w-primary) 28%, transparent); stroke: var(--w-primary); stroke-width: 2;
      stroke-linejoin: round; }
    .other { fill: none; stroke: var(--w-accent); stroke-width: 1.5; stroke-dasharray: 4 3; stroke-linejoin: round; }
    .dot { fill: var(--w-primary); stroke: var(--w-glass); stroke-width: 1; }
  `,
})
export class AromaWheel {
  /** The profile's taste, family by family, the strongest 1. */
  readonly aroma = input.required<Partial<Record<string, number>>>();
  /** Another to set beside it, in outline. */
  readonly compare = input<Partial<Record<string, number>> | null>(null);
  /** What the wheel is of, for a screen reader. */
  readonly of = input('this profile');

  protected readonly R = R;
  protected readonly rings = [0.33, 0.66, 1];

  protected readonly spokes = computed<Spoke[]>(() =>
    FAMILIES.map((f, i) => {
      const a = angle(i);
      const cos = Math.cos(a);
      return {
        id: f.id, label: f.label, x: R * cos, y: R * Math.sin(a), lx: LABEL * cos, ly: LABEL * Math.sin(a) * 0.92,
        anchor: Math.abs(cos) < 0.2 ? 'middle' : cos > 0 ? 'start' : 'end',
        value: this.aroma()[f.id] ?? 0,
      };
    }),
  );

  protected readonly points = computed(() => polygonOf(this.aroma()));
  protected readonly mine = computed(() => this.points().map((p) => `${p.px.toFixed(1)},${p.py.toFixed(1)}`).join(' '));
  protected readonly other = computed(() => {
    const c = this.compare();
    return c ? polygonOf(c).map((p) => `${p.px.toFixed(1)},${p.py.toFixed(1)}`).join(' ') : null;
  });

  protected readonly label = computed(() => {
    const strong = this.spokes().filter((s) => s.value >= 0.5).map((s) => s.label);
    return `How ${this.of()} tastes: ${strong.length ? strong.join(', ') : 'nothing written down'}`;
  });
}

/** Where the i-th family's spoke points: the first straight up, round clockwise. */
function angle(i: number): number {
  return -Math.PI / 2 + (i * 2 * Math.PI) / FAMILIES.length;
}

/** A taste's points on the wheel, a family each. */
function polygonOf(aroma: Partial<Record<string, number>>) {
  return FAMILIES.map((f, i) => {
    const value = aroma[f.id] ?? 0;
    // A family not tasted still sits a little off the centre, so the shape reads as one.
    const r = R * Math.max(Math.sqrt(value), 0.04);
    return { id: f.id, label: f.label, value, px: r * Math.cos(angle(i)), py: r * Math.sin(angle(i)) };
  });
}
