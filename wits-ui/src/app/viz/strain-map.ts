import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { scaleLinear, scaleSqrt } from 'd3-scale';

import { Strain } from '../data/models';
import { scoreColor } from './effect-matrix';
import { hostWidth } from './width';

const H = 380;
const PAD = 28;

/**
 * Every product dispensed, placed by its profile: across by its THC, up and
 * down by its name and terpenes, alike at one height — sized by how much of it
 * was dispensed, coloured by what the effect log says it did. Picked, a product shows its nearest neighbours: "something like the
 * Wedding Cake".
 */
@Component({
  selector: 'w-strain-map',
  imports: [DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg [attr.viewBox]="'0 0 ' + width() + ' ' + h" [attr.height]="h" role="img" aria-label="A map of the strains">
      @if (picked(); as p) {
        @for (n of neighbours(); track n.slug) {
          <line class="link" [attr.x1]="p.cx" [attr.y1]="p.cy" [attr.x2]="n.cx" [attr.y2]="n.cy" />
        }
      }
      @for (d of dots(); track d.slug) {
        <circle [attr.cx]="d.cx" [attr.cy]="d.cy" [attr.r]="d.r" [style.fill]="d.fill" class="dot"
          [class.on]="picked()?.slug === d.slug" [class.near]="isNear(d.slug)" [class.dim]="picked() && !isNear(d.slug) && picked()?.slug !== d.slug"
          (click)="pick(d.slug)"><title>{{ d.name }}</title></circle>
      }
      @for (t of ticks(); track t.v) {
        <text class="tick" [attr.x]="t.x" [attr.y]="h - 8" text-anchor="middle">{{ t.v }}%</text>
      }
      <text class="tick" [attr.x]="width() - 8" [attr.y]="h - 8" text-anchor="end">THC</text>
      @for (d of labelled(); track d.slug) {
        <text class="label" [attr.x]="d.cx + d.r + 4" [attr.y]="d.ly">{{ d.name }}</text>
      }
    </svg>
    <div class="legend">
      <span><i class="sw" style="background: var(--w-warn)"></i>rated low</span>
      <span><i class="sw" style="background: var(--w-accent)"></i>rated high</span>
      <span><i class="sw" style="background: var(--w-dim)"></i>not rated yet</span>
      <span class="dim">size: grams dispensed · pick one for what is like it</span>
    </div>
    @if (picked(); as p) {
      <p class="like">Like <b>{{ p.name }}</b>
        @if (p.terpenes.length) { <span class="dim">({{ p.terpenes.join(', ') }})</span> }:
        @for (n of neighbours(); track n.slug; let last = $last) {
          {{ n.name }}@if (n.ratings) { <span class="dim"> · {{ n.overall | number: '1.1-1' }} over {{ n.ratings }}</span> }@if (!last) {, }
        }
      </p>
    }
  `,
  styles: `
    :host { display: block; width: 100%; }
    svg { display: block; width: 100%; }
    .dot { stroke: color-mix(in srgb, var(--w-ink) 80%, transparent); stroke-width: 1.5; cursor: pointer; opacity: .85; transition: opacity .2s; }
    .dot.on { stroke: var(--w-hi); stroke-width: 2.5; opacity: 1; }
    .dot.near { stroke: var(--w-hi); opacity: 1; }
    .dot.dim { opacity: .25; }
    .link { stroke: color-mix(in srgb, var(--w-fg) 35%, transparent); stroke-width: 1.5; }
    .tick { fill: var(--w-dim); font: 500 10px var(--w-num); }
    .label { fill: var(--mat-sys-on-surface); font: 500 11px var(--w-font); pointer-events: none; }
    .legend { display: flex; flex-wrap: wrap; gap: 8px 18px; font-size: 12px; color: var(--w-dim); margin-top: 6px; }
    .sw { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 6px; vertical-align: middle; }
    .like { margin: 10px 0 0; font-size: 14px; }
    .dim { color: var(--w-dim); }
  `,
})
export class StrainMap {
  readonly strains = input.required<Strain[]>();
  protected readonly width = hostWidth();
  protected readonly h = H;
  protected readonly chosen = signal<string | null>(null);

  private readonly x = computed(() => {
    const xs = this.strains().map((s) => s.x);
    return scaleLinear().domain([Math.min(...xs), Math.max(...xs)]).nice().range([PAD + 8, this.width() - PAD * 6]);
  });

  protected readonly ticks = computed(() => this.x().ticks(6).map((v) => ({ v, x: this.x()(v) })));

  protected readonly dots = computed(() => {
    const all = this.strains();
    const x = this.x();
    // Up and down by rank, not by value: alike stays together, and the many
    // that are as unlike as each other spread out instead of forming one band.
    const order = all.map((s, i) => [s.y, i] as const).sort((a, b) => a[0] - b[0]);
    const rank = new Map(order.map(([, i], r) => [i, r]));
    const y = scaleLinear().domain([0, Math.max(all.length - 1, 1)]).range([H - PAD * 2, PAD]);
    const r = scaleSqrt().domain([0, Math.max(...all.map((s) => s.dispensed), 1)]).range([3, 16]);
    return all.map((s, i) => ({
      ...s,
      cx: x(s.x),
      cy: y(rank.get(i) ?? 0),
      r: r(s.dispensed),
      fill: s.ratings ? scoreColor(s.overall ?? 3) : 'color-mix(in srgb, var(--w-dim) 55%, var(--w-ink))',
    }));
  });

  protected readonly picked = computed(() => this.dots().find((d) => d.slug === this.chosen()) ?? null);
  protected readonly neighbours = computed(() => {
    const p = this.picked();
    return p ? this.dots().filter((d) => p.like.includes(d.slug)) : [];
  });

  /**
   * The picked product and its neighbours carry their names, each nudged
   * down past the one above it so none is written over another; the rest
   * carry a title.
   */
  protected readonly labelled = computed(() => {
    const p = this.picked();
    if (!p) {
      return [];
    }
    const out = [p, ...this.neighbours()].map((d) => ({ ...d, ly: d.cy + 4 })).sort((a, b) => a.ly - b.ly);
    for (let i = 1; i < out.length; i++) {
      out[i].ly = Math.max(out[i].ly, out[i - 1].ly + 14);
    }
    return out;
  });

  protected isNear(slug: string): boolean {
    return this.picked()?.like.includes(slug) ?? false;
  }

  protected pick(slug: string): void {
    this.chosen.update((c) => (c === slug ? null : slug));
  }
}
