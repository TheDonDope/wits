import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { arc } from 'd3-shape';

import { CountUp } from './count-up';

interface Segment {
  key: string;
  label: string;
  grams: number;
  color: string;
}

/**
 * The fill at a glance: one ring for what the prescription dispensed, split
 * into what is still sealed, ground into the stash, and collected as AVB —
 * the rest has gone through a vaporizer. A thin outer halo is the shelf's: the
 * older jars still standing beside this fill, drawn apart so the two scopes
 * never read as one sum.
 */
@Component({
  selector: 'w-fill-ring',
  imports: [CountUp],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="-150 -150 300 300" role="img" [attr.aria-label]="label()">
      <defs>
        <filter id="ring-glow" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="6" result="b" />
          <feMerge><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge>
        </filter>
        <radialGradient id="ring-core">
          <stop offset="0" stop-color="rgba(139,123,255,0.22)" />
          <stop offset="1" stop-color="rgba(139,123,255,0)" />
        </radialGradient>
      </defs>
      <circle r="96" fill="url(#ring-core)" />
      <path class="track" [attr.d]="track" />
      @for (s of arcs(); track s.key; let i = $index) {
        <path
          class="seg"
          [attr.d]="s.d"
          [attr.fill]="s.color"
          filter="url(#ring-glow)"
          [style.animation-delay.ms]="120 * i"
        >
          <title>{{ s.label }}: {{ s.grams.toFixed(2) }} g</title>
        </path>
      }
      @if (halo(); as h) {
        <path class="halo-track" [attr.d]="haloTrack" />
        <path class="halo" [attr.d]="h"><title>{{ carried().toFixed(2) }} g in older jars</title></path>
      }
    </svg>
    <div class="centre">
      <div class="eyebrow">left of this fill</div>
      <div class="big"><w-count [value]="remaining()" /><span class="unit">g</span></div>
      <div class="of dim">of {{ dispensed().toFixed(0) }} g dispensed</div>
    </div>
  `,
  styles: `
    :host { position: relative; display: block; aspect-ratio: 1; width: 100%; max-width: 340px; }
    svg { width: 100%; height: 100%; overflow: visible; }
    .track { fill: rgba(255, 255, 255, 0.05); }
    /* The viewBox is centred on the ring, so its origin is the ring's centre. */
    .seg { opacity: 0; transform-box: view-box; transform-origin: 0 0;
      animation: grow 1.1s cubic-bezier(.2,.8,.2,1) forwards; }
    .halo-track { fill: rgba(255, 255, 255, 0.04); }
    .halo { fill: var(--w-violet); opacity: 0.75; }
    @keyframes grow { from { opacity: 0; transform: rotate(-30deg) scale(.92); } to { opacity: 1; transform: none; } }
    .centre {
      position: absolute; inset: 0; display: grid; place-content: center; text-align: center; gap: 2px;
    }
    .big { font: 600 clamp(36px, 7vw, 54px)/1 var(--w-num); letter-spacing: -0.04em;
      background: linear-gradient(120deg, #fff, #c9c2ff 60%, var(--w-mint));
      -webkit-background-clip: text; background-clip: text; color: transparent; }
    .unit { font-size: 0.45em; margin-left: 4px; color: var(--w-dim); -webkit-text-fill-color: var(--w-dim); }
    .of { font-size: 13px; }
  `,
})
export class FillRing {
  readonly dispensed = input.required<number>();
  readonly sealed = input.required<number>();
  readonly stash = input.required<number>();
  readonly avb = input.required<number>();
  readonly carried = input(0);

  protected readonly remaining = computed(() => this.sealed());

  protected readonly track = arc()({ innerRadius: 104, outerRadius: 126, startAngle: 0, endAngle: 2 * Math.PI }) ?? '';
  protected readonly haloTrack =
    arc()({ innerRadius: 134, outerRadius: 138, startAngle: 0, endAngle: 2 * Math.PI }) ?? '';

  protected readonly arcs = computed(() => {
    const total = this.dispensed();
    if (total <= 0) {
      return [];
    }
    const segs: Segment[] = [
      { key: 'sealed', label: 'Still sealed', grams: this.sealed(), color: 'var(--w-storage)' },
      { key: 'stash', label: 'Ground, in the stash', grams: this.stash(), color: 'var(--w-stash)' },
      { key: 'avb', label: 'Collected as AVB', grams: this.avb(), color: 'var(--w-avb)' },
    ];
    const gap = 0.025;
    let at = 0;
    const out: (Segment & { d: string })[] = [];
    for (const s of segs) {
      const span = (s.grams / total) * 2 * Math.PI;
      if (span > gap) {
        const d = arc().cornerRadius(11)({
          innerRadius: 104,
          outerRadius: 126,
          startAngle: at + gap / 2,
          endAngle: at + span - gap / 2,
        });
        out.push({ ...s, d: d ?? '' });
      }
      at += span;
    }
    return out;
  });

  /** The older jars, as a share of the whole shelf around the fill. */
  protected readonly halo = computed(() => {
    const c = this.carried();
    if (c <= 0) {
      return null;
    }
    const share = c / (c + this.sealed());
    return arc().cornerRadius(2)({ innerRadius: 134, outerRadius: 138, startAngle: 0, endAngle: share * 2 * Math.PI });
  });

  protected readonly label = computed(
    () => `${this.sealed().toFixed(2)} grams of ${this.dispensed().toFixed(0)} still sealed`,
  );
}
