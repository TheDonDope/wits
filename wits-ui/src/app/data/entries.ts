import { HttpClient } from '@angular/common/http';
import { Signal, computed, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import { Entry } from './models';

/** The journal from where this page left it, as the server answers `/api/journal?after=`. */
interface Since {
  entries: Entry[];
  head: string;
  reset: boolean;
  total: number;
  names: Record<string, string>;
}

/** The journal as a page reads it: its entries, newest first, and how many it holds. */
export interface JournalView {
  entries: Entry[];
  total: number;
}

/**
 * The journal, kept in the page and brought up to date with what was
 * recorded since it was last read — not read whole again on every change, as
 * it was at 0.9 MB a time. Entries are kept in the order they were recorded,
 * since that is the order the journal grows in; what pages read is ordered
 * as the fold orders them, newest first. Which entries a correction undid is
 * worked out here from the corrections themselves, since an undo recorded
 * today changes an entry read last week; and every entry's product is named
 * as the catalog names it now.
 */
export class Entries {
  private readonly recorded = signal<Entry[]>([]);
  private readonly names = signal<Record<string, string>>({});
  private head = '';
  private busy: Promise<void> | null = null;
  private again = false;

  /** Every entry, corrections included — what a replay folds. Newest first. */
  readonly everything: Signal<JournalView>;
  /** The entries that stand, an undone entry and its undo left out. Newest first. */
  readonly standing: Signal<JournalView>;

  constructor(private readonly http: HttpClient) {
    const all = computed(() => {
      const names = this.names();
      const list = this.recorded();
      // An undo names what it undid; both halves are left out of what stands.
      const undone = new Set<string>();
      for (const e of list) {
        if (e.reverts) {
          undone.add(e.reverts);
          undone.add(e.hash);
        }
      }
      const entries = list.map((e, i) => ({
        e: { ...e, product_name: e.product ? (names[e.product] ?? e.product_name) : e.product_name,
          corrected: undone.has(e.hash) && !e.reverts },
        i,
        undone: undone.has(e.hash),
      }));
      // Newest first as the fold orders them: by when it happened, then by when it was recorded.
      entries.sort((a, b) => b.e.occurred_at.localeCompare(a.e.occurred_at) || b.i - a.i);
      return entries;
    });
    this.everything = computed(() => ({ entries: all().map((x) => x.e), total: this.recorded().length }));
    this.standing = computed(() => ({ entries: all().filter((x) => !x.undone).map((x) => x.e), total: this.recorded().length }));
  }

  /** Reads what was recorded since; a read asked for while one runs is made once it is done. */
  refresh(): Promise<void> {
    if (this.busy) {
      this.again = true;
      return this.busy;
    }
    this.busy = this.read().finally(() => {
      this.busy = null;
      if (this.again) {
        this.again = false;
        void this.refresh();
      }
    });
    return this.busy;
  }

  private async read(): Promise<void> {
    try {
      const since = await firstValueFrom(this.http.get<Since>(`/api/journal?after=${encodeURIComponent(this.head)}`));
      this.names.set(since.names);
      this.recorded.update((list) => (since.reset ? since.entries : [...list, ...since.entries]));
      this.head = since.head;
    } catch {
      // The server is away; the next change reads again from where this left off.
    }
  }
}
