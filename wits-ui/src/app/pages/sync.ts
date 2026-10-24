import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { Synced, engine } from '../data/engine';
import { Wits } from '../data/wits';

/**
 * Sync, on the phone: open the remote's wits.age from the shared folder, and
 * save back what comes out. In between it is wits pull and wits push, run in
 * the page: this phone's entries chained on top of the remote's, a clash
 * named, nothing written over. Nothing leaves the phone but the encrypted file
 * handed back to save.
 */
@Component({
  selector: 'w-sync',
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="glass">
      <h1>Sync</h1>
      <p class="dim">
        Open <code>wits.age</code> from the folder your devices share, and save back what comes out. The first time,
        paste the key from <code>.wits/remote.key</code> on the device that made the folder. It stays on this phone.
      </p>

      <div class="row">
        <input #picker type="file" accept=".age" hidden (change)="choose($event)" />
        <button mat-stroked-button (click)="picker.click()">
          <mat-icon>file_open</mat-icon>{{ file()?.name ?? 'Open wits.age' }}
        </button>
      </div>

      @if (!hasKey()) {
        <mat-form-field appearance="outline" class="key">
          <mat-label>The remote's key</mat-label>
          <textarea matInput rows="2" placeholder="AGE-SECRET-KEY-1…" [value]="key()"
            (input)="key.set(value($event))"></textarea>
        </mat-form-field>
      }

      <div class="row">
        <button mat-flat-button [disabled]="busy()" (click)="sync()">
          <mat-icon>sync</mat-icon>{{ file() ? 'Sync' : 'Write this phone’s copy' }}
        </button>
        @if (hasKey()) {
          <button mat-button (click)="reveal()"><mat-icon>key</mat-icon>Show the key</button>
        }
      </div>

      @if (error(); as e) {
        <p class="bad">{{ e }}</p>
      }
      @if (shown(); as k) {
        <p class="dim">Bring this to every other device, and nowhere else:</p>
        <code class="secret">{{ k }}</code>
      }

      @if (done(); as d) {
        <div class="report">
          <p>
            Pulled {{ count(d.pulled.fetched, 'entry', 'entries') }} and {{ count(d.pulled.ratings, 'rating', 'ratings') }}@if (d.pulled.prices) {, with {{ count(d.pulled.prices, 'price', 'prices') }}}.
            @if (d.pulled.rechained) {
              Chained {{ count(d.pulled.rechained, 'entry', 'entries') }} of this phone’s on top, with new hashes.
            }
            Pushed {{ count(d.pushed.events, 'entry', 'entries') }}.
          </p>
          @for (c of d.pulled.clashes; track c.hash) {
            <p class="bad">🚨 {{ c.hash.slice(0, 7) }} {{ c.text }}</p>
          }
          @if (d.pulled.clashes.length) {
            <p class="dim">Two devices recorded the same grams: undo one of them in the journal. The jar takes nothing
              else until then.</p>
          }
          @if (d.file; as f) {
            <button mat-flat-button (click)="save(f)"><mat-icon>save</mat-icon>Save wits.age</button>
            <p class="dim">Save it over the one in the shared folder.</p>
          } @else {
            <p class="dim">Nothing recorded yet, so there is nothing to save.</p>
          }
        </div>
      }
    </section>
  `,
  styles: `
    :host { display: block; max-width: 720px; margin: 0 auto; }
    section { padding: 20px; border-radius: 16px; }
    h1 { font: 600 24px var(--w-num); margin: 0 0 8px; }
    .dim { color: var(--w-dim); }
    .bad { color: var(--w-warn); }
    .row { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin: 16px 0; }
    .key { width: 100%; }
    .key textarea { font-family: var(--w-mono); }
    .secret { display: block; overflow-wrap: anywhere; font-family: var(--w-mono); user-select: all; }
    .report { margin-top: 16px; border-top: 1px solid var(--w-glass-edge); padding-top: 12px; }
  `,
})
export class Sync {
  private readonly wits = inject(Wits);
  protected readonly file = signal<File | null>(null);
  protected readonly key = signal('');
  protected readonly hasKey = signal<boolean | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly shown = signal('');
  protected readonly done = signal<Synced | null>(null);

  constructor() {
    void this.checkKey();
  }

  private async checkKey(): Promise<void> {
    try {
      await (await engine()).key();
      this.hasKey.set(true);
    } catch {
      this.hasKey.set(false);
    }
  }

  protected value(event: Event): string {
    return (event.target as HTMLTextAreaElement).value;
  }

  protected choose(event: Event): void {
    this.file.set((event.target as HTMLInputElement).files?.[0] ?? null);
    this.done.set(null);
  }

  protected async sync(): Promise<void> {
    this.busy.set(true);
    this.error.set('');
    try {
      const f = this.file();
      const opened = f ? new Uint8Array(await f.arrayBuffer()) : null;
      this.done.set(await (await engine()).sync(opened, this.key()));
      this.key.set('');
      this.hasKey.set(true);
      this.wits.version.update((v) => v + 1);
    } catch (e) {
      this.error.set(e instanceof Error ? e.message : String(e));
    } finally {
      this.busy.set(false);
    }
  }

  protected async reveal(): Promise<void> {
    this.shown.set(await (await engine()).key());
  }

  /** Hands the file to another app to save — the folder's — or downloads it. */
  protected async save(data: Uint8Array): Promise<void> {
    const file = new File([data.slice()], 'wits.age', { type: 'application/octet-stream' });
    // Firefox has no canShare; there, the file is downloaded.
    if ('canShare' in navigator && navigator.canShare({ files: [file] })) {
      try {
        await navigator.share({ files: [file] });
        return;
      } catch {
        // Dismissed, or nothing to share to: download it instead.
      }
    }
    const a = document.createElement('a');
    a.href = URL.createObjectURL(file);
    a.download = 'wits.age';
    a.click();
    URL.revokeObjectURL(a.href);
  }

  protected count(n: number, one: string, many: string): string {
    return `${n} ${n === 1 ? one : many}`;
  }
}
