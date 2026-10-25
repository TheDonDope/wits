import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSliderModule } from '@angular/material/slider';
import { MatTooltipModule } from '@angular/material/tooltip';

import { Binding } from '../data/keys';
import { Replay, SPEEDS } from '../data/replay';
import { KINDS } from '../pages/journal';

/**
 * The replay's controls: play from empty, step, speed, a scrubber across the
 * whole record, and the entry that just took the stage. The keys are the
 * terminal's — p or space to play, ←/→ to step, +/- for speed — with esc for
 * back to now.
 */
@Component({
  selector: 'w-transport',
  imports: [DatePipe, DecimalPipe, MatButtonModule, MatIconModule, MatSliderModule, MatTooltipModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="glass bar" [class.on]="!replay().live()">
      <button class="play" (click)="replay().toggle()" [attr.aria-label]="replay().playing() ? 'Pause' : 'Play'"
        [matTooltip]="replay().live() ? 'Replay from empty (p)' : replay().playing() ? 'Pause (p)' : 'Play (p)'">
        <mat-icon>{{ replay().playing() ? 'pause' : 'play_arrow' }}</mat-icon>
      </button>
      <button mat-icon-button (click)="replay().step(-1)" matTooltip="Back one (←)" aria-label="Back one">
        <mat-icon>skip_previous</mat-icon>
      </button>
      <button mat-icon-button (click)="replay().step(1)" matTooltip="On one (→)" aria-label="On one">
        <mat-icon>skip_next</mat-icon>
      </button>

      <div class="scrub">
        <mat-slider [min]="0" [max]="total()" step="1" [discrete]="false" aria-label="Where in the record">
          <input matSliderThumb [value]="head()" (valueChange)="replay().scrub($event)" />
        </mat-slider>
        <div class="where">
          @if (replay().live()) {
            <span class="live-dot"></span><span>live — everything applied</span>
          } @else {
            <span class="num">{{ replay().position().at | number }}</span>
            <span class="dim">of {{ replay().position().of | number }} entries</span>
            @if (replay().current(); as e) {
              <span class="dim">· {{ e.occurred_at | date: 'd MMM y' }}</span>
            }
          }
        </div>
      </div>

      <div class="speed">
        <button mat-icon-button (click)="replay().retune(-1)" aria-label="Slower" matTooltip="Slower (−)">
          <mat-icon>remove</mat-icon>
        </button>
        <span class="num">{{ perSecond() }}<small>/s</small></span>
        <button mat-icon-button (click)="replay().retune(1)" aria-label="Faster" matTooltip="Faster (+)">
          <mat-icon>add</mat-icon>
        </button>
      </div>
      <button mat-stroked-button (click)="replay().goLive()" [disabled]="replay().live()">
        <mat-icon>radio_button_checked</mat-icon> Live
      </button>
    </div>

    @if (replay().current(); as e) {
      @for (k of [e.hash]; track k) {
        <div class="now glass" [style.--tone]="kinds[e.type].tone">
          <div class="icon"><mat-icon>{{ kinds[e.type].icon }}</mat-icon></div>
          <div class="what">
            <span class="eyebrow">{{ e.occurred_at | date: 'EEEE, d MMMM y' }}</span>
            <strong>
              {{ kinds[e.type].verb }} <span class="num">{{ e.grams | number: '1.2-2' }} g</span>
              <span class="dim">{{ e.product_name }}</span>
            </strong>
          </div>
        </div>
      }
    }
  `,
  styles: `
    :host { display: grid; gap: 12px; position: sticky; top: 80px; z-index: 5; }
    .bar { display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-radius: 999px;
      transition: box-shadow .4s, border-color .4s; }
    .bar.on { border-color: color-mix(in srgb, var(--w-primary) 40%, transparent); box-shadow: 0 0 40px -12px color-mix(in srgb, var(--w-primary) 70%, transparent); }
    .play { width: 48px; height: 48px; border-radius: 50%; border: 0; cursor: pointer; display: grid; place-items: center;
      color: var(--w-on-accent); background: linear-gradient(120deg, var(--w-primary), var(--w-accent));
      box-shadow: 0 6px 24px -6px color-mix(in srgb, var(--w-primary) 80%, transparent); transition: transform .15s; }
    .play:hover { transform: scale(1.06); }
    .play mat-icon { font-size: 28px; width: 28px; height: 28px; }
    .scrub { flex: 1; display: grid; min-width: 140px; }
    .scrub mat-slider { width: 100%; }
    .where { display: flex; gap: 6px; align-items: center; font-size: 12px; margin-top: -8px; padding-left: 8px; }
    .live-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--w-warn); box-shadow: 0 0 10px var(--w-warn);
      animation: blink 1.6s ease-in-out infinite; }
    .speed { display: flex; align-items: center; }
    .speed .num { min-width: 44px; text-align: center; font-weight: 600; }
    .speed small { color: var(--w-dim); }
    .now { display: flex; gap: 14px; align-items: center; padding: 12px 18px; border-radius: 18px;
      animation: pop .35s cubic-bezier(.2,.8,.2,1); border-left: 3px solid var(--tone); }
    .icon { width: 40px; height: 40px; border-radius: 12px; display: grid; place-items: center;
      background: color-mix(in srgb, var(--tone) 20%, transparent); color: var(--tone); }
    .what { display: grid; gap: 2px; }
    .what strong { font-size: 16px; }
    .what strong .dim { margin-left: 6px; font-weight: 500; }
    @keyframes pop { from { opacity: .3; transform: translateY(-6px) scale(.98); } to { opacity: 1; transform: none; } }
    @keyframes blink { 50% { opacity: .35; } }
    @media (width <= 760px) { .speed, .bar > button[mat-stroked-button] { display: none; } }
  `,
})
export class Transport {
  readonly replay = input.required<Replay>();
  protected readonly kinds = KINDS;

  protected readonly total = computed(() => this.replay().size());
  protected readonly head = computed(() =>
    this.replay().live() ? this.total() : this.replay().played().length,
  );
  protected readonly perSecond = computed(() => (1000 / SPEEDS[this.replay().speed()]).toFixed(1));
}

/**
 * The replay's keys — the terminal's: p or space to play, ←/→ to step, +/- for
 * speed — and esc back to now. A page folds them into its own layer.
 */
export function replayKeys(r: Replay): Binding[] {
  return [
    { keys: ['p', ' '], label: 'p', help: r.playing() ? 'pause' : r.live() ? 'replay' : 'play', run: () => {
      r.toggle();
    } },
    { keys: ['ArrowLeft'], label: '←/→', help: 'step', run: () => {
      r.step(-1);
    } },
    { keys: ['ArrowRight'], label: '→', help: 'step on', quiet: true, run: () => {
      r.step(1);
    } },
    { keys: ['+', '='], label: '+/-', help: 'speed', run: () => {
      r.retune(1);
    } },
    { keys: ['-', '_'], label: '-', help: 'slower', quiet: true, run: () => {
      r.retune(-1);
    } },
    { keys: ['Escape'], label: 'esc', help: 'live', quiet: r.live(), run: () => {
      r.goLive();
    } },
  ];
}
