import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';

import { Binding } from '../data/keys';
import { Replay, SPEEDS } from '../data/replay';
import { dayAt, place, tape, timeline } from '../data/timeline';
import { KINDS } from '../pages/journal';

/** How many entries the tape holds; the narrowest screen shows the newest of them. */
const TAPE = 8;

/**
 * The replay's controls and where it stands. One panel of one height, live or
 * playing, so nothing under it moves: play, step and speed; the day being
 * played; the whole record as a timeline — the grams ground week by week, a
 * tick at every pickup — lit up to the playhead, scrubbed by pointing; and the
 * tape, the newest entries in a row, each new one coming in at the right and
 * pushing the others along. The keys are the terminal's — p or space to play,
 * ←/→ to step, +/- for speed — with esc for back to now.
 */
@Component({
  selector: 'w-transport',
  imports: [DatePipe, DecimalPipe, MatButtonModule, MatIconModule, MatTooltipModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="glass deck" [class.on]="!replay().live()" [style.--tick.ms]="tick()">
      <div class="controls">
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

        <div class="when" aria-live="off">
          @if (replay().live()) {
            <span class="day"><i class="live-dot"></i>Live</span>
            <span class="dim">{{ replay().position().of | number }} entries, everything applied</span>
          } @else {
            <span class="day num">{{ (replay().current()?.occurred_at ?? first()) | date: 'EEE d MMM y' }}</span>
            <span class="dim"><span class="num">{{ replay().position().at | number }}</span> of {{ replay().position().of | number }} entries</span>
          }
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
        <button mat-stroked-button class="golive" (click)="replay().goLive()" [disabled]="replay().live()">
          <mat-icon>radio_button_checked</mat-icon> Live
        </button>
      </div>

      <!-- The record in time. A pointer scrubs it; the keys step it, as everywhere on the page. -->
      <div class="track" role="slider" tabindex="0" aria-label="Where in the record"
        [attr.aria-valuemin]="0" [attr.aria-valuemax]="replay().size()" [attr.aria-valuenow]="replay().played().length"
        [attr.aria-valuetext]="replay().live() ? 'live' : (replay().current()?.occurred_at | date: 'd MMMM y')"
        (pointerdown)="grab($event)" (pointermove)="drag($event)" (pointerup)="drop($event)" (pointerleave)="hover.set(null)"
        (keydown.home)="replay().scrub(0)" (keydown.end)="replay().goLive()">
        <svg class="bars" viewBox="0 0 1000 40" preserveAspectRatio="none" aria-hidden="true">
          <defs><clipPath [attr.id]="clip"><rect class="lit" x="0" y="0" height="40" [attr.width]="head() * 1000" /></clipPath></defs>
          <g class="dim-bars">
            @for (b of line().bars; track $index) { <rect [attr.x]="$index * bw()" [attr.width]="bw() * 0.72" [attr.y]="40 - h(b)" [attr.height]="h(b)" rx="1" /> }
          </g>
          <g class="lit-bars" [attr.clip-path]="'url(#' + clip + ')'">
            @for (b of line().bars; track $index) { <rect [attr.x]="$index * bw()" [attr.width]="bw() * 0.72" [attr.y]="40 - h(b)" [attr.height]="h(b)" rx="1" /> }
          </g>
        </svg>
        @for (p of line().picks; track $index) {
          <i class="pick" [class.past]="p <= head()" [style.left.%]="p * 100"></i>
        }
        @for (y of line().marks; track y.label) {
          <span class="year" [style.left.%]="y.at * 100">{{ y.label }}</span>
        }
        <span class="head" [style.left.%]="head() * 100"></span>
        @if (hover(); as x) {
          <span class="ghost" [style.left.%]="x * 100"><b>{{ ghostDay() | date: 'd MMM y' }}</b></span>
        }
      </div>

      <!-- The tape: newest at the right; each new entry comes in there and pushes the rest along. -->
      <div class="tape" aria-hidden="true">
        @for (e of strip(); track e.hash; let i = $index) {
          <div class="chip" [class.newest]="i === 0" [style.--i]="i" [style.--tone]="kinds[e.type].tone" animate.enter="in" animate.leave="out">
            <mat-icon>{{ kinds[e.type].icon }}</mat-icon>
            <span class="txt">
              <b>{{ kinds[e.type].verb }} <span class="num">{{ e.grams | number: '1.2-2' }} g</span></b>
              <small>{{ e.occurred_at | date: 'd MMM' }} · {{ e.product_name ?? e.device ?? '' }}</small>
            </span>
          </div>
        }
      </div>
    </div>
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: block; position: sticky; top: 80px; z-index: 5; }
    .deck { display: grid; gap: 10px; padding: 12px 18px 14px; border-radius: 26px; transition: box-shadow .4s, border-color .4s; }
    .deck.on { border-color: color-mix(in srgb, var(--w-primary) 40%, transparent);
      box-shadow: 0 0 40px -12px color-mix(in srgb, var(--w-primary) 70%, transparent); }
    .controls { display: flex; align-items: center; gap: 8px; }
    .play { width: 48px; height: 48px; flex: none; border-radius: 50%; border: 0; cursor: pointer; display: grid; place-items: center;
      color: var(--w-on-accent); background: linear-gradient(120deg, var(--w-primary), var(--w-accent));
      box-shadow: 0 6px 24px -6px color-mix(in srgb, var(--w-primary) 80%, transparent); transition: transform .15s; }
    .play:hover { transform: scale(1.06); }
    .play mat-icon { font-size: 28px; width: 28px; height: 28px; }
    .when { flex: 1; min-width: 0; display: grid; padding-left: 8px; }
    .day { display: flex; align-items: center; gap: 8px; font: 600 20px var(--w-num); white-space: nowrap; }
    .when .dim { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .live-dot { width: 9px; height: 9px; border-radius: 50%; background: var(--w-warn); box-shadow: 0 0 10px var(--w-warn);
      animation: blink 1.6s ease-in-out infinite; }
    .speed { display: flex; align-items: center; }
    .speed .num { min-width: 44px; text-align: center; font-weight: 600; }
    .speed small { color: var(--w-dim); }

    .track { position: relative; height: 54px; cursor: pointer; touch-action: none; border-radius: 10px; outline: none; }
    .track:focus-visible { box-shadow: 0 0 0 2px var(--w-primary); }
    .bars { position: absolute; inset: 0 0 14px; width: 100%; height: 40px; }
    .dim-bars rect { fill: color-mix(in srgb, var(--w-fg) 14%, transparent); }
    .lit-bars rect { fill: var(--w-stash); }
    .lit { transition: width var(--tick) linear; }
    .pick { position: absolute; top: 0; width: 2px; height: 40px; margin-left: -1px; border-radius: 1px;
      background: color-mix(in srgb, var(--w-storage) 30%, transparent); }
    .pick.past { background: var(--w-storage); box-shadow: 0 0 6px color-mix(in srgb, var(--w-storage) 60%, transparent); }
    .year { position: absolute; bottom: 0; transform: translateX(-50%); font: 10px var(--w-mono); color: var(--w-dim); }
    .year::before { content: ''; position: absolute; left: 50%; bottom: 13px; height: 4px; border-left: 1px solid var(--w-glass-edge); }
    .head { position: absolute; top: -4px; height: 48px; width: 3px; margin-left: -1.5px; border-radius: 2px; background: var(--w-fg);
      box-shadow: 0 0 12px var(--w-primary), 0 0 2px var(--w-fg); transition: left var(--tick) linear; }
    .ghost { position: absolute; top: -4px; height: 48px; border-left: 1px dashed var(--w-dim); pointer-events: none; }
    .ghost b { position: absolute; bottom: 100%; transform: translateX(-50%); margin-bottom: 4px; padding: 2px 8px; border-radius: 6px;
      font: 600 11px var(--w-font); white-space: nowrap; background: var(--w-dialog); border: 1px solid var(--w-glass-edge); }

    .tape { --w: 228px; position: relative; height: 46px; overflow: hidden;
      mask-image: linear-gradient(90deg, transparent, #000 22%); }
    .chip { position: absolute; right: 0; top: 0; width: var(--w); height: 46px; box-sizing: border-box; display: flex; align-items: center; gap: 10px;
      padding: 0 12px; border-radius: 14px; border: 1px solid color-mix(in srgb, var(--tone) 30%, transparent);
      background: color-mix(in srgb, var(--tone) 9%, transparent);
      transform: translateX(calc(var(--i) * (var(--w) + 8px) * -1)); opacity: calc(1 - var(--i) * 0.09);
      transition: transform var(--tick) cubic-bezier(.3,.7,.3,1), opacity var(--tick); }
    .chip.newest { border-color: var(--tone); box-shadow: 0 0 22px -8px var(--tone); }
    .chip mat-icon { color: var(--tone); flex: none; }
    .txt { display: grid; min-width: 0; line-height: 1.25; }
    .txt b { font-size: 13px; font-weight: 600; white-space: nowrap; }
    .txt small { font-size: 11px; color: var(--w-dim); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .in { animation: chip-in var(--tick) cubic-bezier(.2,.8,.2,1); }
    .out { animation: chip-out var(--tick) ease-in forwards; }
    @keyframes chip-in { from { opacity: 0; translate: 40px 0; scale: .92; } }
    @keyframes chip-out { to { opacity: 0; translate: -30px 0; } }
    @keyframes blink { 50% { opacity: .35; } }
    @media (width <= 760px) { .speed, .golive { display: none; } }
    @include bp.phone { .deck { padding: 10px 12px 12px; border-radius: 20px; } .tape { --w: 200px; } .day { font-size: 17px; } }
  `,
})
export class Transport {
  readonly replay = input.required<Replay>();
  protected readonly kinds = KINDS;
  /** An id for the clip path, one per transport on the page. */
  protected readonly clip = `lit-${Math.random().toString(36).slice(2, 8)}`;

  protected readonly line = computed(() => timeline(this.replay().events()));
  protected readonly bw = computed(() => 1000 / Math.max(this.line().bars.length, 1));
  protected readonly first = computed(() => this.replay().events()[0]?.occurred_at ?? '');
  /** Where the playhead stands, 0 to 1. */
  protected readonly head = computed(() => {
    const e = this.replay().current();
    return this.replay().live() || !e ? (this.replay().live() ? 1 : 0) : place(this.line(), e.occurred_at);
  });
  protected readonly strip = computed(() => tape(this.replay().played(), TAPE));
  protected readonly perSecond = computed(() => (1000 / SPEEDS[this.replay().speed()]).toFixed(1));
  /** One step's time, which every movement takes, so the tape keeps pace at any speed. */
  protected readonly tick = computed(() => Math.min(SPEEDS[this.replay().speed()] * 0.9, 420));

  /** Where the pointer is over the track, 0 to 1, while it hovers. */
  protected readonly hover = signal<number | null>(null);
  protected readonly ghostDay = computed(() => {
    const x = this.hover();
    return x === null ? '' : dayAt(this.line(), x);
  });
  private dragging = false;

  protected h(b: number): number {
    return b > 0 ? Math.max(b * 38, 2) : 0;
  }

  protected grab(ev: PointerEvent): void {
    this.dragging = true;
    (ev.currentTarget as HTMLElement).setPointerCapture(ev.pointerId);
    this.seek(ev);
  }

  protected drag(ev: PointerEvent): void {
    this.hover.set(this.at(ev));
    if (this.dragging) {
      this.seek(ev);
    }
  }

  protected drop(ev: PointerEvent): void {
    this.dragging = false;
    (ev.currentTarget as HTMLElement).releasePointerCapture(ev.pointerId);
  }

  private at(ev: PointerEvent): number {
    const r = (ev.currentTarget as HTMLElement).getBoundingClientRect();
    return Math.min(Math.max((ev.clientX - r.left) / r.width, 0), 1);
  }

  private seek(ev: PointerEvent): void {
    const x = this.at(ev);
    if (x >= 0.999) {
      this.replay().goLive();
    } else {
      this.replay().scrubTo(dayAt(this.line(), x));
    }
  }
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
