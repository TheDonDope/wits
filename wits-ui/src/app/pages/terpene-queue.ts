import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatSnackBar } from '@angular/material/snack-bar';
import { RouterLink } from '@angular/router';

import { Jar } from '../data/models';
import { Wits } from '../data/wits';

/** Where one jar's lookup stands. */
interface Look {
  state: 'looking' | 'found' | 'none' | 'error';
  terpenes: string[];
  pages: number;
  error?: string;
}

/** How many jars are listed before the rest are folded away. */
const FOLDED = 10;

/** How long to wait between lookups, so the searches are not asked too fast. */
const PAUSE_MS = 1500;

/**
 * The jars with no terpene profile written down, each looked up on the web
 * on its own or all in turn, one after another. What is found is shown
 * beside the jar and kept with a press — one, or every one found — or opened
 * in full to be checked first. Nothing is written down unasked.
 */
@Component({
  selector: 'w-terpene-queue',
  imports: [DatePipe, MatButtonModule, MatIconModule, MatProgressSpinnerModule, RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="bar">
      @if (running()) {
        <button mat-flat-button disabled><mat-spinner diameter="16" /> Researching {{ done() }} of {{ jars().length }}</button>
        <button mat-button (click)="stop()">Stop</button>
      } @else {
        <button mat-flat-button (click)="all()"><mat-icon>travel_explore</mat-icon> Research all {{ jars().length }}</button>
      }
      <button mat-stroked-button (click)="keepAll()" [disabled]="!found().length || saving()">
        <mat-icon>done_all</mat-icon> Keep all found @if (found().length) { ({{ found().length }}) }
      </button>
    </div>
    @for (j of shown(); track j.slug) {
      <div class="row">
        <mat-icon>spa</mat-icon>
        <a class="name" [routerLink]="['/shelf/jars', j.slug]">{{ j.name }}</a>
        <span class="what">
          @switch (looks()[j.slug]?.state) {
            @case ('looking') { <mat-spinner diameter="14" /> }
            @case ('found') {
              <span class="found">{{ looks()[j.slug]?.terpenes?.join(', ') }}</span>
              <span class="dim pages">{{ looks()[j.slug]?.pages }} {{ looks()[j.slug]?.pages === 1 ? 'page' : 'pages' }}</span>
            }
            @case ('none') { <span class="dim">nothing found — open it to type the label's</span> }
            @case ('error') { <span class="error">{{ looks()[j.slug]?.error }}</span> }
            @default { <span class="dim">{{ j.storage > 0 ? 'sealed on the shelf' : 'last used ' + (j.last_seen | date: 'MMM y') }}</span> }
          }
        </span>
        @if (looks()[j.slug]?.state === 'found') {
          <button mat-stroked-button (click)="keep(j)" [disabled]="saving()">Keep</button>
        } @else {
          <button mat-button (click)="one(j)" [disabled]="running() || looks()[j.slug]?.state === 'looking'">Research</button>
        }
      </div>
    }
    @if (hidden() > 0) {
      <button mat-button class="more" (click)="all_.set(true)">Show all {{ jars().length }}, {{ hidden() }} more</button>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; gap: 4px; }
    .bar { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 6px; }
    .bar mat-spinner { display: inline-block; margin-right: 6px; vertical-align: middle; }
    .row { display: grid; grid-template-columns: auto minmax(0, 14rem) minmax(0, 1fr) auto; gap: 12px; align-items: center;
      padding: 6px 0; border-top: 1px solid var(--w-glass-edge); font-size: 13px; }
    .row mat-icon { color: var(--w-stash); }
    .name { color: inherit; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-decoration: none; }
    .name:hover { color: var(--w-primary); }
    .what { min-width: 0; display: flex; align-items: center; flex-wrap: wrap; }
    .found { color: var(--w-stash); }
    .pages::before { content: '·'; margin: 0 6px; }
    .more { justify-self: start; }
    .dim { color: var(--w-dim); }
    .error { color: var(--w-warn); }
    @include bp.phone {
      .row { grid-template-columns: auto minmax(0, 1fr) auto; }
      .what { grid-column: 2; grid-row: 2; }
      .row button { grid-row: 1 / 3; grid-column: 3; }
    }
  `,
})
export class TerpeneQueue {
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  readonly jars = input.required<Jar[]>();

  protected readonly looks = signal<Partial<Record<string, Look>>>({});
  protected readonly running = signal(false);
  protected readonly saving = signal(false);
  private readonly stopped = signal(false);

  /** Whether every jar is listed, not only the first ten. */
  protected readonly all_ = signal(false);
  /** The jars listed: the first ten, and any a lookup has reached, or all of them. */
  protected readonly shown = computed(() => this.all_()
    ? this.jars()
    : this.jars().filter((j, i) => i < FOLDED || this.looks()[j.slug] !== undefined));
  protected readonly hidden = computed(() => this.jars().length - this.shown().length);

  /** The jars a lookup found a profile for, still without one. */
  protected readonly found = computed(() => this.jars().filter((j) => this.looks()[j.slug]?.state === 'found'));
  /** How far a run through every jar has got. */
  protected readonly done = computed(() =>
    this.jars().filter((j) => { const s = this.looks()[j.slug]?.state; return s !== undefined && s !== 'looking'; }).length);

  constructor() {
    inject(DestroyRef).onDestroy(() => {
      this.stopped.set(true);
    });
  }

  private set(slug: string, look: Look): void {
    this.looks.update((all) => ({ ...all, [slug]: look }));
  }

  /** Looks one jar up. */
  protected async one(j: Jar): Promise<void> {
    this.set(j.slug, { state: 'looking', terpenes: [], pages: 0 });
    try {
      const f = await this.wits.research(j.slug);
      this.set(j.slug, f.suggested.length
        ? { state: 'found', terpenes: f.suggested, pages: f.sources.length }
        : { state: 'none', terpenes: [], pages: 0 });
    } catch (e) {
      this.set(j.slug, { state: 'error', terpenes: [], pages: 0, error: (e as Error).message });
    }
  }

  /** Looks every jar not yet looked up, one after another, a pause between. */
  protected async all(): Promise<void> {
    this.running.set(true);
    this.stopped.set(false);
    const todo = this.jars().filter((j) => this.looks()[j.slug]?.state !== 'found');
    for (const [i, j] of todo.entries()) {
      if (this.stopped()) {
        break;
      }
      if (i > 0) {
        await new Promise((go) => setTimeout(go, PAUSE_MS));
      }
      await this.one(j);
    }
    this.running.set(false);
  }

  protected stop(): void {
    this.stopped.set(true);
  }

  /** Writes down what was found for one jar. */
  protected async keep(j: Jar): Promise<void> {
    const look = this.looks()[j.slug];
    if (!look) {
      return;
    }
    this.saving.set(true);
    try {
      const out = await this.wits.terpenes(j.slug, look.terpenes);
      this.snack.open(out.message, undefined, { duration: 3000 });
    } catch (e) {
      this.set(j.slug, { ...look, state: 'error', error: (e as Error).message });
    } finally {
      this.saving.set(false);
    }
  }

  /** Writes down every profile found, one jar after another. */
  protected async keepAll(): Promise<void> {
    const jars = this.found();
    this.saving.set(true);
    let kept = 0;
    for (const j of jars) {
      const look = this.looks()[j.slug];
      if (!look) {
        continue;
      }
      try {
        await this.wits.terpenes(j.slug, look.terpenes);
        kept++;
      } catch (e) {
        this.set(j.slug, { ...look, state: 'error', error: (e as Error).message });
      }
    }
    this.saving.set(false);
    this.snack.open(`Terpenes written down for ${String(kept)} ${kept === 1 ? 'jar' : 'jars'}`, undefined, { duration: 4000 });
  }
}
