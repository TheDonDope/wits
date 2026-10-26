import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { arc, pie } from 'd3-shape';

export interface Slice {
  label: string;
  value: number;
  color: string;
}

const R = 70;

/**
 * A ring of slices, each its share of the whole, the whole said in the
 * middle. Slices grow in from nothing, one after another.
 */
@Component({
  selector: 'w-donut',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="-80 -80 160 160" role="img" [attr.aria-label]="aria()">
      <circle r="58" class="track" />
      @for (p of paths(); track p.label) {
        <path [attr.d]="p.d" [attr.fill]="p.color" [style.--i]="$index" class="slice"><title>{{ p.title }}</title></path>
      }
      <text y="-2" text-anchor="middle" class="v">{{ center() }}</text>
      <text y="16" text-anchor="middle" class="u">{{ unit() }}</text>
    </svg>
  `,
  styles: `
    :host { display: block; }
    svg { display: block; width: 100%; overflow: visible; }
    .track { fill: none; stroke: color-mix(in srgb, var(--w-fg) 7%, transparent); stroke-width: 24; }
    .slice { stroke: var(--w-ink); stroke-width: 2; transform-origin: 0 0; animation: pop .7s cubic-bezier(.2,.8,.2,1) both;
      animation-delay: calc(var(--i) * 90ms); }
    .slice:hover { filter: brightness(1.15) drop-shadow(0 0 6px color-mix(in srgb, var(--w-fg) 30%, transparent)); }
    .v { fill: var(--w-fg); font: 600 22px var(--w-num); }
    .u { fill: var(--w-dim); font: 10px var(--w-font); letter-spacing: .08em; text-transform: uppercase; }
    @keyframes pop { from { transform: scale(.6) rotate(-30deg); opacity: 0; } }
  `,
})
export class Donut {
  readonly slices = input.required<Slice[]>();
  /** What the middle says: the whole, as given. */
  readonly center = input('');
  readonly unit = input('');

  protected readonly paths = computed(() => {
    const total = this.slices().reduce((t, s) => t + s.value, 0);
    const shape = arc<{ startAngle: number; endAngle: number }>().innerRadius(R - 24).outerRadius(R).cornerRadius(3).padAngle(0.02);
    const arcs = pie<Slice>().value((s) => s.value).sort(null)(this.slices().filter((s) => s.value > 0));
    return arcs.map((a) => ({ label: a.data.label, color: a.data.color, d: shape(a) ?? '',
      title: `${a.data.label}: ${total > 0 ? Math.round((a.data.value / total) * 100) : 0} %` }));
  });

  protected readonly aria = computed(() => this.slices().map((s) => `${s.label} ${s.value.toFixed(1)}`).join(', '));
}
