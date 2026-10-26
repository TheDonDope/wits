import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { RouterLink } from '@angular/router';
import { scaleLinear, scaleSqrt } from 'd3-scale';
import { symbol } from 'd3-shape';

import { GROUPS, groupOf } from '../data/aroma';
import { Strain } from '../data/models';
import { AromaWheel } from './aroma-wheel';
import { scoreColor } from './effect-matrix';
import { hostWidth } from './width';

/** One strain as the map draws it. */
interface Mark {
  s: Strain;
  cx: number;
  cy: number;
  path: string;
  fill: string;
  group: string;
}

const PAD = 26;

/**
 * Every product with its terpenes written down, laid out by them: alike sits
 * together, whatever its name or strength. Each is marked by the family it
 * tastes most of — colour and shape both, so neither has to be told apart
 * alone — or coloured by how its sessions went, and sized by what was
 * dispensed. Picked, a strain shows its taste as a wheel, its terpenes, and
 * the five most like it, how alike and in what; pointing at one of them sets
 * its taste over the picked one's.
 */
@Component({
  selector: 'w-strain-map',
  imports: [AromaWheel, DecimalPipe, MatButtonToggleModule, RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="bar">
      <mat-button-toggle-group [value]="by()" (change)="by.set($event.value)" aria-label="Colour the strains by" hideSingleSelectionIndicator>
        <mat-button-toggle value="taste">By taste</mat-button-toggle>
        <mat-button-toggle value="rating">By rating</mat-button-toggle>
      </mat-button-toggle-group>
      <div class="legend">
        @if (by() === 'taste') {
          @for (g of groups; track g.id) {
            <span><svg viewBox="-7 -7 14 14" class="key"><path [attr.d]="keyPath(g.id)" [style.fill]="g.color" /></svg>{{ g.label }}</span>
          }
        } @else {
          <span><i class="sw" [style.background]="color(1.5)"></i>rated low</span>
          <span><i class="sw" [style.background]="color(4.5)"></i>rated high</span>
          <span><i class="sw" style="background: var(--w-dim)"></i>not rated</span>
        }
      </div>
    </div>
    <div class="body" [class.side]="side()">
      <svg [attr.viewBox]="'0 0 ' + mapWidth() + ' ' + height()" [attr.height]="height()" role="img"
        [attr.aria-label]="'A map of ' + marks().length + ' strains, alike placed together by their terpenes'">
        @if (picked(); as p) {
          @for (n of near(); track n.mark.s.slug) {
            <line class="link" [attr.x1]="p.cx" [attr.y1]="p.cy" [attr.x2]="n.mark.cx" [attr.y2]="n.mark.cy"
              [style.stroke-opacity]="0.25 + n.score * 0.6" />
          }
        }
        @for (m of marks(); track m.s.slug) {
          <path class="mark" [attr.d]="m.path" [attr.transform]="'translate(' + m.cx + ',' + m.cy + ')'" [style.fill]="m.fill"
            [class.on]="picked()?.s?.slug === m.s.slug" [class.near]="isNear(m.s.slug)"
            [class.dim]="picked() && !isNear(m.s.slug) && picked()?.s?.slug !== m.s.slug"
            (click)="pick(m.s.slug)" (mouseenter)="hover.set(m.s.slug)" (mouseleave)="hover.set(null)">
            <title>{{ m.s.name }}{{ m.s.taste ? ' — ' + m.s.taste.toLowerCase() : '' }} · {{ m.group }}</title>
          </path>
        }
        @for (l of labels(); track l.slug) {
          <text class="label" [attr.x]="l.x" [attr.y]="l.y" [attr.text-anchor]="l.anchor">{{ l.name }}</text>
        }
      </svg>
      @if (picked(); as p) {
        <aside class="panel">
          <a class="name" [routerLink]="['/shelf/jars', p.s.slug]">{{ p.s.name }}</a>
          @if (p.s.taste) { <p class="taste">{{ p.s.taste }}</p> }
          <w-aroma-wheel [aroma]="p.s.aroma" [compare]="compareAroma()" [of]="p.s.name" />
          @if (compared(); as c) { <p class="dim small">dashed: {{ c.name }}</p> }
          <ol class="terps">
            @for (t of p.s.terpenes; track t; let i = $index) { <li [class.lead]="i === 0">{{ t }}</li> }
          </ol>
          @if (p.s.ratings) {
            <p class="dim small">rated {{ p.s.overall | number: '1.1-1' }} overall, over {{ p.s.ratings }} sessions</p>
          }
          <span class="eyebrow">most like it</span>
          <ul class="likes">
            @for (n of near(); track n.mark.s.slug) {
              <li (mouseenter)="hover.set(n.mark.s.slug)" (mouseleave)="hover.set(null)">
                <button type="button" class="like" (click)="pick(n.mark.s.slug)">
                  <span class="score num">{{ n.score * 100 | number: '1.0-0' }}%</span>
                  <span class="who">{{ n.mark.s.name }}<span class="dim"> · shares {{ n.shared.join(', ') || 'no terpene' }}</span></span>
                </button>
              </li>
            }
          </ul>
        </aside>
      }
    </div>
    @if (unplaced()) {
      <p class="dim small">{{ unplaced() }} {{ unplaced() === 1 ? 'product has' : 'products have' }} no terpenes written down,
        and no place here: look them up under I forgot to, or on the jar's page.</p>
    }
  `,
  styles: `
    :host { display: grid; gap: 12px; width: 100%; }
    .bar { display: flex; flex-wrap: wrap; gap: 10px 18px; align-items: center; }
    .legend { display: flex; flex-wrap: wrap; gap: 6px 16px; font-size: 12px; color: var(--w-dim); }
    .legend span { display: inline-flex; align-items: center; gap: 6px; }
    .key { width: 12px; height: 12px; }
    .sw { display: inline-block; width: 10px; height: 10px; border-radius: 50%; }
    .body { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; align-items: start; }
    .body.side { grid-template-columns: minmax(0, 1fr) 300px; }
    svg { display: block; width: 100%; }
    .mark { stroke: var(--w-glass); stroke-width: 1.5; cursor: pointer; opacity: .9; transition: opacity .2s; }
    .mark.on { stroke: var(--w-hi); stroke-width: 2.5; opacity: 1; }
    .mark.near { stroke: var(--w-hi); stroke-width: 1.5; opacity: 1; }
    .mark.dim { opacity: .22; }
    .link { stroke: var(--w-fg); stroke-width: 1.5; }
    .label { fill: var(--mat-sys-on-surface); font: 500 11px var(--w-font); pointer-events: none;
      paint-order: stroke; stroke: var(--w-ink); stroke-width: 3px; stroke-linejoin: round; }
    .panel { display: grid; gap: 8px; align-content: start; }
    .panel w-aroma-wheel { width: 100%; max-width: 280px; justify-self: center; }
    .name { font: 600 16px/1.3 var(--w-font); color: var(--mat-sys-on-surface); text-decoration: none; }
    .name:hover { color: var(--w-primary); }
    .taste { margin: 0; color: var(--w-dim); font-size: 13px; }
    .terps { display: flex; flex-wrap: wrap; gap: 6px; list-style: none; margin: 0; padding: 0; font-size: 12px; }
    .terps li { padding: 1px 9px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); color: var(--w-dim); }
    .terps li.lead { color: var(--mat-sys-on-surface); font-weight: 600; }
    .likes { list-style: none; margin: 0; padding: 0; display: grid; gap: 2px; }
    .like { all: unset; display: grid; grid-template-columns: 44px minmax(0, 1fr); gap: 8px; padding: 5px 6px;
      border-radius: 8px; cursor: pointer; font-size: 13px; width: 100%; box-sizing: border-box; }
    .like:hover, .like:focus-visible { background: color-mix(in srgb, var(--w-primary) 10%, transparent); }
    .score { font-weight: 600; color: var(--w-primary); }
    .who { min-width: 0; }
    .dim { color: var(--w-dim); }
    .small { font-size: 12px; margin: 0; }
  `,
})
export class StrainMap {
  readonly strains = input.required<Strain[]>();
  protected readonly width = hostWidth();
  protected readonly by = signal<'taste' | 'rating'>('taste');
  protected readonly chosen = signal<string | null>(null);
  protected readonly hover = signal<string | null>(null);
  protected readonly groups = GROUPS;
  protected readonly color = scoreColor;

  /** Narrow, the map is shorter, and the panel goes under it. */
  protected readonly height = computed(() => (this.width() < 640 ? 340 : 540));

  private readonly placed = computed(() => this.strains().filter((s) => s.placed));
  /** Whether the panel stands beside the map: when the map is wide enough to share; else it goes under. */
  protected readonly side = computed(() => !!this.chosenStrain() && this.width() >= 640);
  /** The map's own width: the host's, less the panel beside it (300 px and the gap). */
  protected readonly mapWidth = computed(() => this.width() - (this.side() ? 320 : 0));
  protected readonly unplaced = computed(() => this.strains().length - this.placed().length);

  protected readonly marks = computed<Mark[]>(() => {
    const all = this.placed();
    const w = this.mapWidth();
    const x = scaleLinear().domain([0, 1]).range([PAD, w - PAD]);
    const y = scaleLinear().domain([0, 1]).range([this.height() - PAD, PAD]);
    const xs = spread(all.map((s) => s.x));
    const ys = spread(all.map((s) => s.y));
    const size = scaleSqrt().domain([0, Math.max(...all.map((s) => s.dispensed), 1)]).range([6, 16]);
    return all.map((s, i) => {
      const g = groupOf(s.lead);
      const r = size(s.dispensed);
      return {
        s, cx: x(xs[i]), cy: y(ys[i]), group: g?.label ?? 'no taste known',
        path: symbol(g?.symbol, Math.PI * r * r)() ?? '',
        fill: this.by() === 'taste' ? (g?.color ?? 'var(--w-dim)') : s.ratings && s.overall ? scoreColor(s.overall) : 'var(--w-dim)',
      };
    });
  });

  /** The strain picked: the one asked for, or the one most rated, so the map says something at once. */
  private readonly chosenStrain = computed(() => {
    const all = this.placed();
    return all.find((s) => s.slug === this.chosen())
      ?? [...all].sort((a, b) => b.ratings - a.ratings || b.dispensed - a.dispensed).at(0);
  });

  protected readonly picked = computed(() => {
    const s = this.chosenStrain();
    return s ? this.marks().find((m) => m.s.slug === s.slug) : undefined;
  });

  protected readonly near = computed(() => {
    const p = this.picked();
    if (!p) {
      return [];
    }
    const by = new Map(this.marks().map((m) => [m.s.slug, m]));
    return p.s.like.flatMap((l) => {
      const mark = by.get(l.slug);
      return mark ? [{ mark, score: l.score, shared: l.shared }] : [];
    });
  });

  /** The neighbour pointed at, whose taste is set over the picked one's. */
  protected readonly compared = computed(() => {
    const h = this.hover();
    return h && h !== this.picked()?.s.slug ? this.near().find((n) => n.mark.s.slug === h)?.mark.s : undefined;
  });
  protected readonly compareAroma = computed(() => this.compared()?.aroma ?? null);

  /**
   * The picked strain and those most like it are named on the map, nothing
   * else; each name goes above its mark, or below, right or left of it,
   * whichever first clears the names already placed.
   */
  protected readonly labels = computed(() => {
    const p = this.picked();
    if (!p) {
      return [];
    }
    const placed: Box[] = [];
    const out: { slug: string; name: string; x: number; y: number; anchor: string }[] = [];
    for (const m of [p, ...this.near().map((n) => n.mark)]) {
      const name = short(m.s.name);
      const spot = spotFor(name, m.cx, m.cy, placed, this.mapWidth());
      if (spot) {
        placed.push(spot.box);
        out.push({ slug: m.s.slug, name, x: spot.x, y: spot.y, anchor: spot.anchor });
      }
    }
    return out;
  });

  protected isNear(slug: string): boolean {
    return this.near().some((n) => n.mark.s.slug === slug);
  }

  protected pick(slug: string): void {
    this.chosen.set(slug);
  }

  protected keyPath(id: string): string {
    return symbol(GROUPS.find((g) => g.id === id)?.symbol, 80)() ?? '';
  }
}

/**
 * Where some values fall between 0 and 1: half by their place in order and
 * half by their value, so a cluster opens up and an outlier still stands off
 * — by value alone, two outliers pressed fifty strains into a corner.
 */
export function spread(values: number[]): number[] {
  const lo = Math.min(...values);
  const span = Math.max(...values) - lo || 1;
  const order = values.map((v, i) => ({ v, i })).sort((a, b) => a.v - b.v);
  const rank = new Array<number>(values.length);
  order.forEach((o, r) => {
    rank[o.i] = values.length > 1 ? r / (values.length - 1) : 0.5;
  });
  return values.map((v, i) => 0.5 * rank[i] + 0.5 * ((v - lo) / span));
}

/** A label's room on the map. */
interface Box {
  x0: number;
  y0: number;
  x1: number;
  y1: number;
}

/** Roughly how wide a label is: the font is 11 px, a letter some 6 of it. */
const letter = 6.2;

/** Where a label can go beside its mark without covering one placed before, if anywhere. */
function spotFor(name: string, cx: number, cy: number, placed: Box[], width: number) {
  const w = name.length * letter;
  const tries = [
    { x: cx, y: cy - 13, anchor: 'middle' },
    { x: cx, y: cy + 22, anchor: 'middle' },
    { x: cx + 14, y: cy + 4, anchor: 'start' },
    { x: cx - 14, y: cy + 4, anchor: 'end' },
  ];
  for (const t of tries) {
    const x0 = t.anchor === 'middle' ? t.x - w / 2 : t.anchor === 'start' ? t.x : t.x - w;
    const box = { x0, y0: t.y - 11, x1: x0 + w, y1: t.y + 3 };
    const inside = box.x0 >= 0 && box.x1 <= width && box.y0 >= 0;
    if (inside && !placed.some((b) => b.x0 < box.x1 && box.x0 < b.x1 && b.y0 < box.y1 && box.y0 < b.y1)) {
      return { ...t, box };
    }
  }
  return undefined;
}

/** A product's name as a label: its strain, without the producer's words and the ratio. */
function short(name: string): string {
  const parts = name.split(/\s+\d+\/\d+\s+/);
  return (parts[1] ?? parts[0]).replace(/\s*\(g\)$/, '');
}
