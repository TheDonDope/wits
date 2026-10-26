import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';
import { arc } from 'd3-shape';

/** A mark on the dial: a mode's usual temperature. */
export interface DialMark {
  temp: number;
  label: string;
}

/** A terpene of the strain picked, and where it boils. */
export interface Boils {
  name: string;
  at: number;
}

/** The dial sweeps 240°, from eight o'clock to four. */
const SWEEP = (240 / 180) * Math.PI;
const R = 78;
const WIDTH = 14;

/**
 * A vaporizer's dial to turn: dragged round, or stepped with the arrow keys
 * (page keys by five), between what the device can be set to. Its modes are
 * marked where they sit, and under it the strain's own terpenes light up as
 * the heat passes the point each boils at — the number on the dial read as
 * what it releases.
 */
@Component({
  selector: 'w-temp-dial',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <span class="eyebrow">temperature</span>
    <svg viewBox="-100 -100 200 178" role="slider" tabindex="0" aria-label="Temperature" [attr.aria-valuemin]="min()"
      [attr.aria-valuemax]="max()" [attr.aria-valuenow]="value()" [attr.aria-valuetext]="value() + ' °C'"
      (pointerdown)="grab($event)" (pointermove)="drag($event)" (pointerup)="release($event)" (keydown)="key($event)">
      <defs>
        <linearGradient id="dial-heat" x1="0" x2="1">
          <stop offset="0" stop-color="var(--w-storage)" />
          <stop offset=".55" stop-color="var(--w-avb)" />
          <stop offset="1" stop-color="var(--w-warn)" />
        </linearGradient>
      </defs>
      <path class="track" [attr.d]="track" />
      <path class="heat" [attr.d]="lit()" />
      @for (m of placed(); track m.label) {
        <g class="mark" [class.at]="m.temp === value()" (click)="value.set(m.temp)">
          <line [attr.x1]="m.inner[0]" [attr.y1]="m.inner[1]" [attr.x2]="m.outer[0]" [attr.y2]="m.outer[1]" />
          <text [attr.x]="m.text[0]" [attr.y]="m.text[1]" [attr.text-anchor]="m.anchor">{{ m.label }}</text>
        </g>
      }
      <circle class="knob" [attr.cx]="knob()[0]" [attr.cy]="knob()[1]" r="11" />
      <text class="deg" y="8" text-anchor="middle">{{ value() }}<tspan class="c" dx="2">°C</tspan></text>
      <text class="ends" [attr.x]="ends()[0][0]" [attr.y]="ends()[0][1] + 18" text-anchor="middle">{{ min() }}</text>
      <text class="ends" [attr.x]="ends()[1][0]" [attr.y]="ends()[1][1] + 18" text-anchor="middle">{{ max() }}</text>
    </svg>
    @if (terpenes().length) {
      <div class="boils" aria-live="polite">
        <span class="dim">{{ freed() }} of its {{ terpenes().length }} terpenes boil off by {{ value() }} °C</span>
        <div class="chips">
          @for (t of terpenes(); track t.name) {
            <span class="chip" [class.on]="t.at <= value()">{{ t.name }} <span class="num">{{ t.at }}°</span></span>
          }
        </div>
      </div>
    }
  `,
  styles: `
    :host { display: grid; gap: 6px; justify-items: center; }
    .eyebrow { font-size: 11px; justify-self: start; }
    svg { width: min(260px, 100%); touch-action: none; cursor: grab; user-select: none; overflow: visible; }
    svg:focus-visible { outline: none; }
    svg:focus-visible .knob { stroke: var(--w-fg); stroke-width: 3; }
    svg:active { cursor: grabbing; }
    .track { fill: color-mix(in srgb, var(--w-fg) 8%, transparent); }
    .heat { fill: url(#dial-heat); filter: drop-shadow(0 0 6px color-mix(in srgb, var(--w-avb) 55%, transparent)); }
    .knob { fill: var(--w-fg); stroke: color-mix(in srgb, var(--w-bg) 40%, transparent); stroke-width: 2;
      filter: drop-shadow(0 2px 6px color-mix(in srgb, var(--w-bg) 80%, transparent)); }
    .deg { fill: var(--w-fg); font: 600 40px var(--w-num); letter-spacing: -.02em; }
    .deg .c { font-size: 15px; fill: var(--w-dim); font-weight: 500; }
    .ends { fill: var(--w-dim); font: 10px var(--w-num); }
    .mark { cursor: pointer; }
    .mark line { stroke: var(--w-dim); stroke-width: 2; }
    .mark text { fill: var(--w-dim); font: 9px var(--w-font); }
    .mark.at line { stroke: var(--w-primary); }
    .mark.at text { fill: var(--w-primary); font-weight: 600; }
    .boils { display: grid; gap: 6px; justify-items: center; text-align: center; font-size: 12px; }
    .chips { display: flex; flex-wrap: wrap; gap: 5px; justify-content: center; }
    .chip { padding: 2px 9px; border-radius: 99px; font-size: 11px; border: 1px solid var(--w-glass-edge); color: var(--w-dim);
      transition: background .25s, color .25s, border-color .25s; }
    .chip .num { opacity: .75; }
    .chip.on { color: var(--w-fg); border-color: var(--w-avb); background: color-mix(in srgb, var(--w-avb) 22%, transparent); }
    .dim { color: var(--w-dim); }
    @media (prefers-reduced-motion: reduce) { .chip { transition: none; } }
  `,
})
export class TempDial {
  readonly value = model(185);
  readonly min = input(120);
  readonly max = input(230);
  readonly marks = input<DialMark[]>([]);
  readonly terpenes = input<Boils[]>([]);

  private readonly shape = arc<{ startAngle: number; endAngle: number }>()
    .innerRadius(R - WIDTH / 2).outerRadius(R + WIDTH / 2).cornerRadius(WIDTH / 2);
  protected readonly track = this.shape({ startAngle: -SWEEP / 2, endAngle: SWEEP / 2 }) ?? '';
  protected readonly lit = computed(() => this.shape({ startAngle: -SWEEP / 2, endAngle: this.angle(this.value()) }) ?? '');
  protected readonly knob = computed(() => this.point(this.angle(this.value()), R));
  protected readonly ends = computed(() => [this.point(-SWEEP / 2, R), this.point(SWEEP / 2, R)]);
  protected readonly placed = computed(() => this.marks()
    .filter((m) => m.temp >= this.min() && m.temp <= this.max())
    .map((m) => {
      const a = this.angle(m.temp);
      return { ...m, inner: this.point(a, R + 10), outer: this.point(a, R + 16), text: this.point(a, R + 24),
        anchor: Math.abs(a) < 0.3 ? 'middle' : a < 0 ? 'end' : 'start' };
    }));
  protected readonly freed = computed(() => this.terpenes().filter((t) => t.at <= this.value()).length);
  private dragging = false;

  private angle(t: number): number {
    const span = Math.max(1, this.max() - this.min());
    const share = (Math.min(Math.max(t, this.min()), this.max()) - this.min()) / span;
    return -SWEEP / 2 + share * SWEEP;
  }

  /** The point at an angle from twelve o'clock, clockwise, as d3's arcs measure it. */
  private point(a: number, r: number): [number, number] {
    return [r * Math.sin(a), -r * Math.cos(a)];
  }

  protected grab(e: PointerEvent): void {
    this.dragging = true;
    (e.currentTarget as Element).setPointerCapture(e.pointerId);
    this.drag(e);
  }

  protected drag(e: PointerEvent): void {
    if (!this.dragging) {
      return;
    }
    const svg = e.currentTarget as SVGSVGElement;
    const box = svg.getBoundingClientRect();
    // The viewBox is 200 wide from -100; its centre sits 100/178 of the way down.
    const scale = 200 / box.width;
    const x = (e.clientX - box.left) * scale - 100;
    const y = (e.clientY - box.top) * scale - 100;
    const a = Math.min(Math.max(Math.atan2(x, -y), -SWEEP / 2), SWEEP / 2);
    this.value.set(Math.round(this.min() + ((a + SWEEP / 2) / SWEEP) * (this.max() - this.min())));
  }

  protected release(e: PointerEvent): void {
    this.dragging = false;
    (e.currentTarget as Element).releasePointerCapture(e.pointerId);
  }

  protected key(e: KeyboardEvent): void {
    const by = { ArrowUp: 1, ArrowRight: 1, ArrowDown: -1, ArrowLeft: -1, PageUp: 5, PageDown: -5 }[e.key];
    const to = e.key === 'Home' ? this.min() : e.key === 'End' ? this.max() : null;
    if (by === undefined && to === null) {
      return;
    }
    e.preventDefault();
    this.value.set(to ?? Math.min(this.max(), Math.max(this.min(), this.value() + (by ?? 0))));
  }
}
