import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';

import { Listing, Place, Verdict } from '../data/models';
import { Wits } from '../data/wits';
import { Confirm, ConfirmDialog } from './confirm';

const RECENT = 'wits.recent';

/** The repositories opened here before, newest first; none when storage is refused. */
function recalled(): string[] {
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(RECENT) ?? '[]');
    return Array.isArray(raw) ? raw.filter((p): p is string => typeof p === 'string') : [];
  } catch {
    return [];
  }
}

/**
 * Which repository the server serves, and what goes into it: open another
 * from the folders on this computer, or make one in an empty folder; and
 * import a tracking workbook, read first and reported as wits import reports
 * it, written only when asked and only into an empty journal.
 */
@Component({
  selector: 'w-repository',
  imports: [DecimalPipe, MatButtonModule, MatIconModule, MatTooltipModule, RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="glass pad">
      <span class="eyebrow">the repository</span>
      @if (place(); as p) {
        <h1>{{ p.name }}</h1>
        <code class="path">{{ p.path }}</code>
        <span class="dim">{{ p.entries | number }} {{ p.entries === 1 ? 'entry' : 'entries' }} in the journal</span>
      }
    </section>

    <section class="glass pad">
      <header>
        <span class="eyebrow">open another</span>
        @if (recent().length) {
          <div class="recent">
            @for (r of recent(); track r) {
              <button mat-stroked-button (click)="open(r)" [disabled]="busy() || r === place()?.path" [matTooltip]="r">
                <mat-icon>history</mat-icon>{{ base(r) }}
              </button>
            }
          </div>
        }
      </header>

      @if (listing(); as l) {
        <div class="where">
          <button mat-icon-button (click)="browse(l.parent)" [disabled]="!l.parent" aria-label="The folder above">
            <mat-icon>arrow_upward</mat-icon>
          </button>
          <button mat-icon-button (click)="browse(place()?.home)" aria-label="The home folder"><mat-icon>home</mat-icon></button>
          <code class="path" [title]="l.path"><bdi>{{ l.path }}</bdi></code>
        </div>
        <div class="dirs" role="list">
          @for (d of l.dirs; track d.path) {
            <div class="dir" role="listitem" [class.repo]="d.repo">
              <button class="go" (click)="browse(d.path)">
                <mat-icon>{{ d.repo ? 'inventory_2' : 'folder' }}</mat-icon>
                <span class="name">{{ d.name }}</span>
                @if (d.repo) { <span class="scope">wits</span> }
              </button>
              @if (d.repo) {
                <button mat-button (click)="open(d.path)" [disabled]="busy() || d.path === place()?.path">
                  {{ d.path === place()?.path ? 'Open now' : 'Open' }}
                </button>
              }
            </div>
          } @empty {
            <p class="dim">No folders in here.</p>
          }
        </div>
        <div class="row">
          @if (l.repo) {
            <button mat-flat-button (click)="open(l.path)" [disabled]="busy() || l.path === place()?.path">
              <mat-icon>folder_open</mat-icon>Open this repository
            </button>
          } @else {
            <button mat-stroked-button (click)="open(l.path, true)" [disabled]="busy()">
              <mat-icon>create_new_folder</mat-icon>Make a repository here
            </button>
          }
        </div>
      }
      @if (browseError(); as e) { <p class="bad">{{ e }}</p> }
    </section>

    <section class="glass pad">
      <span class="eyebrow">import a workbook</span>
      <p class="dim">A tracking spreadsheet, each worksheet a fill and the grinds that followed it. It is read and
        reported first; nothing is written until you import, and only into a repository with an empty journal.</p>
      <div class="row">
        <input #picker type="file" hidden (change)="choose($event)"
          accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" />
        <button mat-stroked-button (click)="picker.click()" [disabled]="busy()">
          <mat-icon>upload_file</mat-icon>{{ file()?.name ?? 'Choose a workbook' }}
        </button>
      </div>

      @if (verdict(); as v) {
        <div class="figures">
          <div><span class="eyebrow">worksheets</span><span class="num v">{{ v.sheets }}</span></div>
          <div><span class="eyebrow">products</span><span class="num v">{{ v.products }}</span></div>
          <div><span class="eyebrow">entries</span><span class="num v">{{ v.entries | number }}</span></div>
          <div><span class="eyebrow">dispensed</span><span class="num v">{{ v.dispensed | number: '1.2-2' }} g</span></div>
          <div><span class="eyebrow">ground</span><span class="num v">{{ v.ground | number: '1.2-2' }} g</span></div>
          <div><span class="eyebrow">unaccounted</span><span class="num v">{{ v.dispensed - v.ground | number: '1.2-2' }} g</span></div>
        </div>
        @if (v.first) {
          <p>{{ v.fills }} fills and {{ v.grinds | number }} grinds, from {{ v.first }} to {{ v.last }}.</p>
        }
        @if (v.merged.length) {
          <details>
            <summary>{{ v.merged.length }} {{ v.merged.length === 1 ? 'product was' : 'products were' }} named more than one way, and become one</summary>
            <ul>
              @for (m of v.merged; track m.slug) { <li><code>{{ m.slug }}</code> {{ m.names.join(' · ') }}</li> }
            </ul>
          </details>
        }
        @if (v.anomalies.length) {
          <details>
            <summary>{{ v.anomalies.length }} {{ v.anomalies.length === 1 ? 'thing' : 'things' }} worth checking in the spreadsheet</summary>
            <ul>
              @for (a of v.anomalies; track $index) { <li>{{ a }}</li> }
            </ul>
          </details>
        } @else {
          <p class="dim">Nothing in the spreadsheet looks wrong.</p>
        }

        @if (v.committed) {
          <p class="good"><mat-icon>check_circle</mat-icon>Imported {{ v.products }} products and {{ v.entries | number }} entries.
            <a routerLink="/">See the dashboard</a></p>
        } @else {
          <div class="row">
            <button mat-flat-button (click)="read(true)" [disabled]="busy() || !empty()">
              <mat-icon>download_done</mat-icon>Import {{ v.entries | number }} entries
            </button>
            @if (!empty()) {
              <span class="dim">This repository already holds {{ place()?.entries | number }} entries. Importing twice
                would double every gram: make a repository in an empty folder above, and import there.</span>
            }
          </div>
        }
      }
      @if (importError(); as e) { <p class="bad">{{ e }}</p> }
    </section>
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .pad { padding: 20px 24px; display: grid; gap: 12px; }
    h1 { font: 600 28px var(--w-num); margin: 0; }
    p { margin: 0; }
    .path { font: 13px var(--w-mono); color: var(--w-dim); overflow-wrap: anywhere; }
    header { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; }
    .recent, .row { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
    .where { display: flex; align-items: center; gap: 4px; min-width: 0; }
    .where .path { flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; direction: rtl; text-align: left; }
    /* Clipped from the left, so the end of a long path shows; bdi keeps its slashes where they belong. */
    .where bdi { direction: ltr; unicode-bidi: isolate; }
    .dirs { display: grid; max-height: 360px; overflow-y: auto; border: var(--w-border) solid var(--w-glass-edge);
      border-radius: calc(var(--w-radius) / 2); }
    .dir { display: flex; align-items: center; gap: 8px; padding-right: 8px; border-top: 1px solid var(--w-glass-edge); }
    .dir:first-child { border-top: 0; }
    .go { flex: 1; min-width: 0; display: flex; align-items: center; gap: 12px; min-height: 48px; padding: 0 14px;
      border: 0; background: none; color: inherit; font: 14px var(--w-font); text-align: left; cursor: pointer; }
    .go:hover { background: color-mix(in srgb, var(--w-fg) 5%, transparent); }
    .go mat-icon { color: var(--w-dim); }
    .repo .go mat-icon { color: var(--w-primary); }
    .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .figures { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 14px; }
    .figures div { display: grid; gap: 2px; }
    .v { font-size: 22px; font-weight: 600; }
    details summary { cursor: pointer; }
    ul { margin: 8px 0 0; padding-left: 20px; font-size: 13px; display: grid; gap: 4px; }
    .bad { color: var(--w-warn); }
    .good { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; color: var(--w-good); }
    .good a { color: var(--w-primary); }
    @include bp.phone { .pad { padding: 16px; } h1 { font-size: 24px; } }
  `,
})
export class Repository {
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  private readonly dialog = inject(MatDialog);
  /** The repository being served: the service's, which the header's chip reads too. */
  protected readonly place = computed<Place | null>(() => this.wits.place.value() ?? null);
  protected readonly listing = signal<Listing | null>(null);
  protected readonly recent = signal(recalled());
  protected readonly busy = signal(false);
  protected readonly browseError = signal('');
  protected readonly file = signal<File | null>(null);
  protected readonly verdict = signal<Verdict | null>(null);
  protected readonly importError = signal('');
  protected readonly empty = computed(() => this.place()?.entries === 0);

  constructor() {
    // Each repository served goes first among the recent ones; the first one
    // seen is browsed from beside, where another is likeliest to be.
    effect(() => {
      const p = this.place();
      if (!p) {
        return;
      }
      untracked(() => {
        this.remember(p.path);
        if (!this.listing()) {
          void this.browse(p.path.slice(0, Math.max(p.path.lastIndexOf('/'), 1)));
        }
      });
    });
  }

  protected async browse(path?: string): Promise<void> {
    this.browseError.set('');
    try {
      this.listing.set(await this.wits.dirs(path));
    } catch (err) {
      this.browseError.set((err as Error).message);
    }
  }

  protected async open(path: string, create = false): Promise<void> {
    if (create && !(await this.sure(path))) {
      return;
    }
    this.busy.set(true);
    this.browseError.set('');
    try {
      const p = await this.wits.open(path, create);
      this.verdict.update((v) => (v?.committed ? null : v));
      this.snack.open(`${create ? 'Made and opened' : 'Opened'} ${p.name}`, undefined, { duration: 3000 });
      await this.browse(p.path);
    } catch (err) {
      this.browseError.set((err as Error).message);
    } finally {
      this.busy.set(false);
    }
  }

  /** Asks before a repository is made: a folder of its own, written into the one named. */
  private sure(path: string): Promise<boolean | undefined> {
    const ref = this.dialog.open<ConfirmDialog, Confirm, boolean>(ConfirmDialog, {
      data: {
        title: 'Make a repository here?',
        body: `A .wits folder goes into ${path}, with an empty journal, and the server serves it from then on.`,
        yes: 'Make it',
        no: 'Cancel',
      },
    });
    return firstValueFrom(ref.afterClosed());
  }

  protected async choose(ev: Event): Promise<void> {
    const input = ev.target as HTMLInputElement;
    const f = input.files?.[0] ?? null;
    input.value = '';
    if (!f) {
      return;
    }
    this.file.set(f);
    this.verdict.set(null);
    await this.read(false);
  }

  /** Sends the chosen workbook: a dry run, or the import itself. */
  protected async read(commit: boolean): Promise<void> {
    const f = this.file();
    if (!f) {
      return;
    }
    this.busy.set(true);
    this.importError.set('');
    try {
      this.verdict.set(await this.wits.importBook(f, commit));
    } catch (err) {
      this.importError.set((err as Error).message);
    } finally {
      this.busy.set(false);
    }
  }

  protected base(path: string): string {
    return path.split('/').filter(Boolean).pop() ?? path;
  }

  /** Puts a repository first among the recent ones, kept to six. */
  private remember(path: string): void {
    const next = [path, ...this.recent().filter((p) => p !== path)].slice(0, 6);
    this.recent.set(next);
    try {
      localStorage.setItem(RECENT, JSON.stringify(next));
    } catch {
      // Private windows may refuse storage; the list holds until reload.
    }
  }
}
