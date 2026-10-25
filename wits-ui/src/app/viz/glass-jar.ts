import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export interface Layer {
  grams: number;
  color: string;
  label: string;
}

let next = 0;

// The inside of the jar, in its own 100 × 140 box: where liquid can be.
const TOP = 34;
const BOTTOM = 128;
const LEFT = 14;
const RIGHT = 86;

/**
 * A jar of glass, filled to what it holds: the layers stack from the bottom —
 * sealed, then ground — against what it was filled with, and the surface
 * moves, two waves out of step, so a full jar and a nearly empty one are told
 * apart before a number is read.
 */
@Component({
  selector: 'w-jar',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="0 0 100 140" role="img" [attr.aria-label]="aria()">
      <defs>
        <clipPath [attr.id]="clip">
          <path [attr.d]="inside" />
        </clipPath>
        <linearGradient [attr.id]="shine" x1="0" x2="1">
          <stop offset="0" stop-color="color-mix(in srgb, var(--w-fg) 16%, transparent)" />
          <stop offset="0.18" stop-color="color-mix(in srgb, var(--w-fg) 2%, transparent)" />
          <stop offset="0.8" stop-color="transparent" />
          <stop offset="1" stop-color="color-mix(in srgb, var(--w-fg) 8%, transparent)" />
        </linearGradient>
      </defs>
      <!-- lid -->
      <rect x="22" y="6" width="56" height="16" rx="5" class="lid" />
      <rect x="18" y="20" width="64" height="8" rx="3" class="rim" />
      <g [attr.clip-path]="'url(#' + clip + ')'">
        <rect x="0" y="0" width="100" height="140" class="empty" />
        @for (l of bands(); track $index) {
          <rect x="0" [attr.y]="l.y" width="100" [attr.height]="l.h + 1" [attr.fill]="l.color" class="band">
            <title>{{ l.label }}: {{ l.grams.toFixed(2) }} g</title>
          </rect>
        }
        @if (surface(); as s) {
          <g [attr.transform]="'translate(0,' + s.y + ')'">
            <path class="wave back" [attr.d]="wave" [attr.fill]="s.color" />
            <path class="wave front" [attr.d]="wave" [attr.fill]="s.color" />
          </g>
        }
      </g>
      <path [attr.d]="inside" class="glass" [attr.fill]="'url(#' + shine + ')'" />
      <path d="M22 44 Q20 80 23 116" class="glint" />
    </svg>
  `,
  styles: `
    :host { display: block; }
    svg { width: 100%; height: 100%; overflow: visible; }
    .lid { fill: color-mix(in srgb, var(--w-fg) 16%, var(--w-ink)); stroke: color-mix(in srgb, var(--w-fg) 8%, transparent); }
    .rim { fill: color-mix(in srgb, var(--w-fg) 9%, var(--w-ink)); }
    .empty { fill: color-mix(in srgb, var(--w-fg) 25%, transparent); }
    .band { transition: y 900ms cubic-bezier(.2,.8,.2,1), height 900ms cubic-bezier(.2,.8,.2,1); }
    .glass { stroke: color-mix(in srgb, var(--w-fg) 16%, transparent); stroke-width: 1.2; }
    .glint { fill: none; stroke: color-mix(in srgb, var(--w-fg) 22%, transparent); stroke-width: 2.4; stroke-linecap: round; }
    .wave { transform-box: fill-box; }
    .wave.back { opacity: 0.45; animation: drift 7s linear infinite reverse; }
    .wave.front { opacity: 0.95; animation: drift 4.5s linear infinite; }
    @keyframes drift { from { transform: translateX(0); } to { transform: translateX(-100px); } }
  `,
})
export class GlassJar {
  readonly layers = input.required<Layer[]>();
  readonly capacity = input.required<number>();

  protected readonly clip = `jar-clip-${++next}`;
  protected readonly shine = `jar-shine-${next}`;

  protected readonly inside = `M${LEFT + 6} ${TOP - 4} H${RIGHT - 6} Q${RIGHT} ${TOP - 4} ${RIGHT} ${TOP + 8}
    V${BOTTOM - 10} Q${RIGHT} ${BOTTOM} ${RIGHT - 10} ${BOTTOM} H${LEFT + 10}
    Q${LEFT} ${BOTTOM} ${LEFT} ${BOTTOM - 10} V${TOP + 8} Q${LEFT} ${TOP - 4} ${LEFT + 6} ${TOP - 4} Z`;

  /** Two periods of a gentle sine, wide enough to slide by one. */
  protected readonly wave = (() => {
    let d = 'M0 0';
    for (let x = 0; x <= 200; x += 5) {
      d += ` L${x} ${(Math.sin((x / 100) * 2 * Math.PI) * 2.2).toFixed(2)}`;
    }
    return `${d} L200 12 L0 12 Z`;
  })();

  protected readonly bands = computed(() => {
    const cap = Math.max(this.capacity(), 0.01);
    const height = BOTTOM - TOP;
    let y = BOTTOM;
    return this.layers().map((l) => {
      const h = Math.min((l.grams / cap) * height, y - TOP);
      y -= Math.max(h, 0);
      return { ...l, y, h: Math.max(h, 0) };
    });
  });

  protected readonly surface = computed(() => {
    const bands = this.bands().filter((b) => b.h > 0.5);
    const top = bands.at(-1);
    return top ? { y: top.y - 2, color: top.color } : null;
  });

  protected readonly aria = computed(() =>
    this.layers()
      .map((l) => `${l.label} ${l.grams.toFixed(2)} grams`)
      .join(', '),
  );
}
