import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDialog } from '@angular/material/dialog';
import { MatSnackBar } from '@angular/material/snack-bar';
import { firstValueFrom } from 'rxjs';

import { Jar } from '../data/models';
import { Wits } from '../data/wits';
import { Confirm, ConfirmDialog } from './confirm';

/**
 * The stashes left behind: jars with nothing sealed left whose stash still
 * holds grams, the sessions never logged. All ticked to begin with, cleared
 * together — recorded as consumed, an adjustment each, as the Jars page does.
 */
@Component({
  selector: 'w-stash-clear',
  imports: [DatePipe, DecimalPipe, MatButtonModule, MatCheckboxModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="list">
      <mat-checkbox class="all" [checked]="allTicked()" [indeterminate]="someTicked()" (change)="tickAll($event.checked)">
        every one
      </mat-checkbox>
      @for (j of jars(); track j.slug) {
        <div class="row">
          <mat-checkbox [checked]="!skipped().has(j.slug)" (change)="tick(j.slug, $event.checked)">
            <span class="name">{{ j.name }}</span>
          </mat-checkbox>
          <span class="dim when">last {{ j.last_seen | date: 'd MMM y' }}</span>
          <span class="num grams">{{ j.stash | number: '1.2-2' }} g</span>
        </div>
      }
    </div>
    <div class="go">
      <span class="dim">{{ ticked().length }} of {{ jars().length }} · <span class="num">{{ grams() | number: '1.2-2' }} g</span></span>
      <button mat-flat-button class="clear" [disabled]="!ticked().length || busy()" (click)="clear()">
        Clear {{ ticked().length === 1 ? 'this stash' : ticked().length + ' stashes' }}
      </button>
    </div>
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; gap: 12px; }
    .list { display: grid; max-height: 420px; overflow: auto; }
    .all { padding: 0 0 6px; }
    .row { display: grid; grid-template-columns: minmax(0, 1fr) auto 90px; gap: 10px; align-items: center;
      padding: 4px 0; border-top: 1px solid var(--w-glass-edge); }
    .row mat-checkbox { min-width: 0; }
    .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .when { font-size: 12px; }
    .grams { text-align: right; }
    .dim { color: var(--w-dim); }
    .go { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; }
    .clear { background: linear-gradient(120deg, var(--w-warn), var(--w-avb)) !important; color: var(--w-on-accent) !important; }
    .clear:disabled { background: color-mix(in srgb, var(--w-fg) 6%, transparent) !important; color: var(--w-dim) !important; }
    @include bp.phone {
      .row { grid-template-columns: minmax(0, 1fr) auto; }
      .when { grid-column: 1; grid-row: 2; padding-left: 40px; }
      .grams { grid-row: 1 / 3; grid-column: 2; }
    }
  `,
})
export class StashClear {
  private readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);

  readonly jars = input.required<Jar[]>();
  /** The jars unticked: everything else is cleared. */
  protected readonly skipped = signal(new Set<string>());
  protected readonly busy = signal(false);
  protected readonly ticked = computed(() => this.jars().filter((j) => !this.skipped().has(j.slug)));
  protected readonly grams = computed(() => this.ticked().reduce((s, j) => s + j.stash, 0));
  protected readonly allTicked = computed(() => this.ticked().length === this.jars().length);
  protected readonly someTicked = computed(() => this.ticked().length > 0 && !this.allTicked());

  protected tick(slug: string, on: boolean): void {
    this.skipped.update((s) => {
      const next = new Set(s);
      if (on) {
        next.delete(slug);
      } else {
        next.add(slug);
      }
      return next;
    });
  }

  protected tickAll(on: boolean): void {
    this.skipped.set(on ? new Set() : new Set(this.jars().map((j) => j.slug)));
  }

  protected async clear(): Promise<void> {
    const jars = this.ticked();
    if (!jars.length) {
      return;
    }
    const ok = await firstValueFrom(
      this.dialog
        .open<ConfirmDialog, Confirm, boolean>(ConfirmDialog, {
          data: {
            title: `Clear ${jars.length === 1 ? 'this stash' : `${jars.length} stashes`}?`,
            body:
              `${this.grams().toFixed(2)} g will be recorded as consumed — the grams went through a device at some point, ` +
              'the sessions never logged. Nothing is deleted: each is an adjustment, in the journal beside the rest.',
            yes: 'Clear',
          },
        })
        .afterClosed(),
    );
    if (!ok) {
      return;
    }
    this.busy.set(true);
    try {
      const out = await this.wits.flush(jars.map((j) => j.slug));
      this.skipped.set(new Set());
      this.snack.open(out.message, undefined, { duration: 4000 });
    } catch (err) {
      this.snack.open((err as Error).message, 'OK', { duration: 6000 });
    } finally {
      this.busy.set(false);
    }
  }
}
