import { HttpClient, HttpErrorResponse, httpResource } from '@angular/common/http';
import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import { Cycle, Day, Device, Entry, Jar, Overview, Written } from './models';

/**
 * The repository, as wits-server answers for it.
 *
 * Every view reads through a resource keyed on `version`: a write here bumps
 * it, and so does a poll that finds the journal grown — an entry made with the
 * command line in another terminal shows up within a few seconds, the way the
 * terminal interface follows it.
 */
@Injectable({ providedIn: 'root' })
export class Wits {
  private readonly http = inject(HttpClient);
  readonly version = signal(0);

  readonly overview = httpResource<Overview>(() => `/api/overview?v=${this.version()}`);
  readonly jars = httpResource<Jar[]>(() => `/api/jars?v=${this.version()}`, { defaultValue: [] });
  readonly cycles = httpResource<Cycle[]>(() => `/api/cycles?v=${this.version()}`, { defaultValue: [] });
  readonly devices = httpResource<Device[]>(() => `/api/devices?v=${this.version()}`, { defaultValue: [] });
  readonly days = httpResource<Day[]>(() => `/api/days?v=${this.version()}`, { defaultValue: [] });
  readonly journal = httpResource<{ entries: Entry[]; total: number }>(
    () => `/api/journal?limit=100000&v=${this.version()}`,
    { defaultValue: { entries: [], total: 0 } },
  );

  private seen = -1;

  constructor() {
    const timer = setInterval(() => void this.poll(), 4000);
    inject(DestroyRef).onDestroy(() => clearInterval(timer));
  }

  /** Reloads every view when the journal has grown or shrunk elsewhere. */
  private async poll(): Promise<void> {
    try {
      const { total } = await firstValueFrom(
        this.http.get<{ total: number }>('/api/journal?limit=1'),
      );
      if (this.seen >= 0 && total !== this.seen) {
        this.version.update((v) => v + 1);
      }
      this.seen = total;
    } catch {
      // The server is away; the next poll will find it again.
    }
  }

  grind(body: { product: string; grams: number; date?: string }) {
    return this.write('grind', body);
  }

  sesh(body: { product: string; grams: number; device?: string; temperature?: number; note?: string; date?: string }) {
    return this.write('sesh', body);
  }

  buy(body: { name: string; grams: number; slug?: string; date?: string }) {
    return this.write('buy', body);
  }

  weigh(body: { product: string; account: string; weighed: number }) {
    return this.write('weigh', body);
  }

  undo(hash: string) {
    return this.write('undo', { hash });
  }

  /** Writes an entry and refreshes everything; rejects with the server's words. */
  private async write(what: string, body: object): Promise<Written> {
    try {
      const out = await firstValueFrom(this.http.post<Written>(`/api/${what}`, body));
      this.version.update((v) => v + 1);
      return out;
    } catch (err) {
      const e = err as HttpErrorResponse;
      throw new Error(e.error?.error ?? e.message);
    }
  }
}
