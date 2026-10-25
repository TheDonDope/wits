import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, computed, effect, inject, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { ActivatedRoute } from '@angular/router';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { firstValueFrom } from 'rxjs';

import { balances, dayOf, stands } from '../data/derive';
import { capacities, filmFrames } from '../data/film';
import { Binding, Keys } from '../data/keys';
import { Jar } from '../data/models';
import { Replay } from '../data/replay';
import { Wits } from '../data/wits';
import { FILM, record } from '../viz/film';
import { GlassJar } from '../viz/glass-jar';
import { Transport, replayKeys } from '../viz/transport';
import { CarryDialog } from './carry';
import { Confirm, ConfirmDialog } from './confirm';

/**
 * Every product, one card each, in the jar's scope: its whole life. Cards are
 * ticked to have their stashes cleared together — recorded as consumed, the
 * sessions never logged — and the whole shelf can be played back, the jars
 * filling as fills arrive and draining as they are ground.
 */
@Component({
  selector: 'w-jars',
  imports: [
    DatePipe,
    DecimalPipe,
    GlassJar,
    MatButtonModule,
    MatButtonToggleModule,
    MatCheckboxModule,
    MatIconModule,
    MatTooltipModule,
    Transport,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="glass head">
      <div>
        <span class="eyebrow">the jars <span class="scope">each across its life</span></span>
        <h1 class="num">{{ list().length }} <span class="dim">{{ which() === 'shelf' ? 'on the shelf' : 'finished' }}</span></h1>
      </div>
      <div class="actions">
        <button mat-stroked-button (click)="film()" [disabled]="filming() !== null || !history().length"
          matTooltip="Three years of the shelf in thirty seconds, as a video">
          <mat-icon>movie</mat-icon>
          {{ filming() === null ? 'Film' : 'Filming ' + filming() + '%' }}
        </button>
        <button mat-stroked-button (click)="carry()" matTooltip="A signed statement of what is held, as a QR code">
          <mat-icon>qr_code_2</mat-icon>
          What I carry
        </button>
        <button mat-stroked-button (click)="toggleAll()" [disabled]="!replay.live() || !list().length">
          <mat-icon>{{ allSelected() ? 'deselect' : 'select_all' }}</mat-icon>
          {{ allSelected() ? 'Select none' : 'Select all' }}
        </button>
        <button mat-flat-button class="clear" (click)="clear()" [disabled]="!replay.live() || !selectedWithStash().length"
          [matTooltip]="selectedWithStash().length ? '' : 'Tick the jars whose stash to clear'">
          <mat-icon>delete_sweep</mat-icon>
          Clear stash
          @if (selected().size) { <span class="count num">{{ selected().size }}</span> }
        </button>
        <mat-button-toggle-group [value]="which()" (change)="which.set($event.value)" aria-label="Which jars">
          <mat-button-toggle value="shelf">On the shelf</mat-button-toggle>
          <mat-button-toggle value="finished">Finished</mat-button-toggle>
        </mat-button-toggle-group>
      </div>
    </section>

    <w-transport [replay]="replay" />

    <section class="grid">
      @for (j of list(); track j.slug; let i = $index) {
        <article class="card glass" [class.picked]="selected().has(j.slug)" [class.focus]="i === cursor()"
          [style.animation-delay.ms]="i * 30" (click)="toggle(j.slug)" (keydown.enter)="toggle(j.slug)" tabindex="0">
          <mat-checkbox class="tick" [checked]="selected().has(j.slug)" [disabled]="!replay.live()"
            (click)="$event.stopPropagation()" (change)="toggle(j.slug)" [aria-label]="'Select ' + j.name" />
          <w-jar [capacity]="j.bought"
            [layers]="[
              { grams: j.storage, color: 'var(--w-storage)', label: 'sealed' },
              { grams: j.stash, color: 'var(--w-stash)', label: 'in the stash' },
            ]" />
          <div class="body">
            <h3>{{ j.name }}</h3>
            <div class="chips">
              @if (j.thc) { <span class="chip thc">THC {{ j.thc }}%</span> }
              @if (j.cbd) { <span class="chip cbd">CBD {{ j.cbd }}%</span> }
              <span class="chip">{{ j.fills }} {{ j.fills === 1 ? 'fill' : 'fills' }}</span>
            </div>
            <dl>
              <div><dt>sealed</dt><dd class="num" style="color: var(--w-storage)">{{ j.storage | number: '1.2-2' }}</dd></div>
              <div><dt>stash</dt><dd class="num" style="color: var(--w-stash)">{{ j.stash | number: '1.2-2' }}</dd></div>
              <div><dt>ground</dt><dd class="num">{{ j.ground | number: '1.0-0' }}</dd></div>
              <div><dt>dispensed</dt><dd class="num">{{ j.bought | number: '1.0-0' }}</dd></div>
            </dl>
            <span class="dim seen"><mat-icon>schedule</mat-icon>last {{ j.last_seen | date: 'd MMM y' }}</span>
          </div>
        </article>
      } @empty {
        <p class="dim">Nothing here{{ replay.live() ? '' : ' yet, at this point in the record' }}.</p>
      }
    </section>
  `,
  styles: `
    :host { display: grid; gap: 20px; }
    .head { padding: 18px 24px; display: flex; flex-wrap: wrap; gap: 18px; align-items: center; justify-content: space-between; }
    h1 { font: 600 28px var(--w-num); margin: 4px 0 0; }
    h1 .dim { font: 400 15px 'Inter Variable', sans-serif; }
    .actions { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
    .clear { background: linear-gradient(120deg, #ff7a8a, #ffb454) !important; color: #0b0e14 !important; }
    .clear:disabled { background: rgb(255 255 255 / 6%) !important; color: var(--w-dim) !important; }
    .count { margin-left: 6px; padding: 0 7px; border-radius: 99px; background: rgb(11 14 20 / 25%); font-size: 12px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 18px; }
    .card { position: relative; display: grid; grid-template-columns: 96px 1fr; gap: 16px; padding: 18px; opacity: 0;
      animation: rise .6s cubic-bezier(.2,.8,.2,1) forwards; transition: transform .25s, border-color .25s, box-shadow .25s;
      cursor: pointer; outline: none; }
    .card:hover { transform: translateY(-3px); border-color: rgb(139 123 255 / 35%); }
    .card.focus { border-color: rgb(139 123 255 / 70%); }
    .card.picked { border-color: rgb(255 122 138 / 70%); box-shadow: 0 0 0 1px rgb(255 122 138 / 40%), 0 0 36px -10px rgb(255 122 138 / 60%); }
    .tick { position: absolute; top: 6px; right: 6px; }
    w-jar { width: 96px; height: 134px; }
    .body { display: grid; gap: 8px; align-content: start; min-width: 0; padding-right: 28px; }
    h3 { margin: 0; font: 600 15px/1.3 'Inter Variable', sans-serif; }
    .chips { display: flex; gap: 6px; flex-wrap: wrap; }
    .chip { font: 600 10px/1.8 'Inter Variable', sans-serif; padding: 0 8px; border-radius: 99px; background: rgb(255 255 255 / 6%); color: var(--w-dim); }
    .chip.thc { background: rgb(124 245 196 / 12%); color: var(--w-mint); }
    .chip.cbd { background: rgb(110 168 255 / 14%); color: var(--w-storage); }
    dl { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 14px; margin: 4px 0 0; }
    dl div { display: grid; gap: 1px; min-width: 0; }
    dt { font: 600 9px 'Inter Variable', sans-serif; letter-spacing: .1em; text-transform: uppercase; color: var(--w-dim); }
    dd { margin: 0; font-size: 15px; font-weight: 600; }
    .seen { display: flex; gap: 4px; align-items: center; font-size: 12px; }
    .seen mat-icon { font-size: 14px; width: 14px; height: 14px; }
    @keyframes rise { from { opacity: 0; transform: translateY(14px); } to { opacity: 1; transform: none; } }
  `,
})
export class Jars {
  private readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);
  protected readonly which = signal<'shelf' | 'finished'>('shelf');
  protected readonly selected = signal(new Set<string>());
  protected readonly cursor = signal(-1);

  /** Every entry that has happened, oldest first: what the replay plays. */
  protected readonly history = computed(() =>
    [...this.wits.everything.value().entries].reverse(),
  );
  protected readonly replay = new Replay(this.history);

  /** The jars live from the server, or as they stood at the replay's head. */
  private readonly jars = computed<Jar[]>(() => {
    const live = this.wits.jars.value();
    return this.replay.live() ? live : this.jarsAt(live);
  });

  protected readonly list = computed<Jar[]>(() => {
    const held = (j: Jar) => j.storage + j.stash > 0.004;
    return this.jars().filter((j) => (this.which() === 'shelf' ? held(j) : !held(j)));
  });

  protected readonly allSelected = computed(
    () => this.list().length > 0 && this.list().every((j) => this.selected().has(j.slug)),
  );

  protected readonly selectedWithStash = computed(() =>
    this.list().filter((j) => this.selected().has(j.slug) && j.stash > 0),
  );

  /** How far a film being recorded has got, in percent, or null. */
  protected readonly filming = signal<number | null>(null);

  /**
   * Records the replay as a thirty-second film of the shelf, and hands it over
   * to save — or, on a phone, to share.
   */
  protected async film(): Promise<void> {
    const frames = filmFrames(this.history(), FILM.fps * 30);
    const names = new Map(this.wits.jars.value().map((j) => [j.slug, j.name]));
    this.filming.set(0);
    try {
      const video = await record(frames, names, capacities(frames), (done) => {
        this.filming.set(Math.round(done * 100));
      });
      const file = new File([video], `wits-shelf.${video.type.includes('mp4') ? 'mp4' : 'webm'}`, { type: video.type });
      if ('canShare' in navigator && navigator.maxTouchPoints > 0 && navigator.canShare({ files: [file] })) {
        await navigator.share({ files: [file] }).catch(() => undefined);
      } else {
        const a = document.createElement('a');
        a.href = URL.createObjectURL(file);
        a.download = file.name;
        a.click();
        URL.revokeObjectURL(a.href);
      }
    } catch (e) {
      this.snack.open(e instanceof Error ? e.message : 'the film could not be made', undefined, { duration: 4000 });
    } finally {
      this.filming.set(null);
    }
  }

  /** Opens the signed statement of what is held. */
  protected carry(): void {
    this.dialog.open(CarryDialog, { maxWidth: 'min(640px, 94vw)' });
  }

  constructor() {
    const destroy = inject(DestroyRef);
    // ?at=2025-06-01 opens the record as it stood that day — a moment to
    // bookmark. It waits for the entries to load, then lets go.
    const at = inject(ActivatedRoute).snapshot.queryParamMap.get('at');
    if (at) {
      const once = effect(() => {
        if (this.replay.size() > 0) {
          untracked(() => {
            this.replay.scrubTo(at);
          });
          once.destroy();
        }
      });
    }
    destroy.onDestroy(() => {
      this.replay.destroy();
    });
    // A tick means "this jar, now": a replay or a change of tab drops them.
    effect(() => {
      this.which();
      if (!this.replay.live()) {
        this.selected.set(new Set());
      }
    });
    inject(Keys).register('Jars', () => this.keys(), destroy);
  }

  /** The jars as the played entries leave them, named from the live list. */
  private jarsAt(live: Jar[]): Jar[] {
    const known = new Map(live.map((j) => [j.slug, j]));
    const played = this.replay.played();
    const held = balances(played);
    const life = new Map<string, { bought: number; ground: number; fills: number; seen: string }>();
    // What was bought and ground counts what stands; the balances above fold everything.
    for (const e of played) {
      if (!e.product || !stands(e)) {
        continue;
      }
      const l = life.get(e.product) ?? { bought: 0, ground: 0, fills: 0, seen: '' };
      if (e.type === 'purchase') {
        l.bought += e.grams;
        l.fills++;
      } else if (e.type === 'grind') {
        l.ground += e.grams;
      }
      if (dayOf(e) >= l.seen.slice(0, 10)) {
        l.seen = e.occurred_at;
      }
      life.set(e.product, l);
    }
    return [...life.entries()]
      .map(([slug, l]) => {
        const h = held.get(slug) ?? { storage: 0, stash: 0, avb: 0 };
        return {
          ...(known.get(slug) ?? { slug, name: slug }),
          storage: h.storage,
          stash: h.stash,
          avb: h.avb,
          bought: l.bought,
          ground: l.ground,
          fills: l.fills,
          last_seen: l.seen,
        };
      })
      .sort((a, b) => Date.parse(b.last_seen) - Date.parse(a.last_seen));
  }

  private keys(): Binding[] {
    const move = (d: number) => () => {
      this.cursor.update((c) => Math.min(Math.max(c + d, 0), this.list().length - 1));
    };
    return [
      { keys: ['j', 'ArrowDown'], label: 'j/k', help: 'move', run: move(1) },
      { keys: ['k', 'ArrowUp'], label: 'k', help: 'up', quiet: true, run: move(-1) },
      // Space marks here, as in the terminal; play stays on p.
      { keys: [' '], label: 'space', help: 'select', run: () => {
        const j = this.list().at(this.cursor());
        if (j) {
          this.toggle(j.slug);
        }
      } },
      { keys: ['a'], help: this.allSelected() ? 'select none' : 'select all', run: () => {
        this.toggleAll();
      } },
      { keys: ['c'], help: 'clear stash', run: () => {
        void this.clear();
      } },
      { keys: ['f'], help: this.which() === 'shelf' ? 'finished' : 'on the shelf', run: () => {
        this.which.update((w) => (w === 'shelf' ? 'finished' : 'shelf'));
      } },
      ...replayKeys(this.replay).map((b) => (b.keys.includes(' ') ? { ...b, keys: ['p'] } : b)),
    ];
  }

  protected toggle(slug: string): void {
    if (!this.replay.live()) {
      return;
    }
    this.selected.update((s) => {
      const next = new Set(s);
      if (!next.delete(slug)) {
        next.add(slug);
      }
      return next;
    });
  }

  protected toggleAll(): void {
    this.selected.set(this.allSelected() ? new Set() : new Set(this.list().map((j) => j.slug)));
  }

  protected async clear(): Promise<void> {
    const jars = this.selectedWithStash();
    if (!jars.length || !this.replay.live()) {
      return;
    }
    const grams = jars.reduce((s, j) => s + j.stash, 0);
    const ok = await firstValueFrom(
      this.dialog
        .open<ConfirmDialog, Confirm, boolean>(ConfirmDialog, {
          data: {
            title: `Clear ${jars.length === 1 ? 'this stash' : `${jars.length} stashes`}?`,
            body:
              `${grams.toFixed(2)} g will be recorded as consumed — the grams went through a device at some point, ` +
              'the sessions never logged. Nothing is deleted: each is an adjustment, in the journal beside the rest.',
            yes: 'Clear',
          },
        })
        .afterClosed(),
    );
    if (!ok) {
      return;
    }
    try {
      const out = await this.wits.flush(jars.map((j) => j.slug));
      this.selected.set(new Set());
      this.snack.open(out.message, undefined, { duration: 4000 });
    } catch (err) {
      this.snack.open((err as Error).message, 'OK', { duration: 6000 });
    }
  }
}
