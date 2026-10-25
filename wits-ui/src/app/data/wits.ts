import { HttpClient, HttpErrorResponse, httpResource } from '@angular/common/http';
import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import { ENGINE } from './mode';

import {
  Avb, Costs, Cycle, Strain, Day, Device, Dimension, EffectLog, Entry, Jar, LabelReading, Listing, Overview, Place, Verdict, Written,
} from './models';

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
  readonly effects = httpResource<EffectLog>(() => `/api/effects?v=${this.version()}`);
  readonly avb = httpResource<Avb>(() => `/api/avb?v=${this.version()}`);
  readonly costs = httpResource<Costs>(() => `/api/costs?v=${this.version()}`);
  /** The repository being served; the phone has the one, and no such endpoint. */
  readonly place = httpResource<Place>(() => (ENGINE ? undefined : `/api/repo?v=${this.version()}`));
  readonly strains = httpResource<Strain[]>(() => `/api/strains?v=${this.version()}`, { defaultValue: [] });
  readonly journal = httpResource<{ entries: Entry[]; total: number }>(
    () => `/api/journal?limit=100000&v=${this.version()}`,
    { defaultValue: { entries: [], total: 0 } },
  );
  /** Every entry, corrections included — what a replay folds. Newest first. */
  readonly everything = httpResource<{ entries: Entry[]; total: number }>(
    () => `/api/journal?limit=100000&all=1&v=${this.version()}`,
    { defaultValue: { entries: [], total: 0 } },
  );

  private seen = -1;

  constructor() {
    const timer = setInterval(() => void this.poll(), 4000);
    inject(DestroyRef).onDestroy(() => {
      clearInterval(timer);
    });
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

  /** What a label's text says about a fill. It records nothing. */
  label(text: string): Promise<LabelReading> {
    return this.post<LabelReading>('label', { text });
  }

  weigh(body: { product: string; account: string; weighed: number }) {
    return this.write('weigh', body);
  }

  undo(hash: string) {
    return this.write('undo', { hash });
  }

  /** Rates what a session did. */
  rate(session: string, scores: Partial<Record<Dimension, number>>, note: string) {
    return this.write<{ message: string }>('rate', { session, scores, note });
  }

  /** Records the AVB weighed out of a device, shared between its sessions' products. */
  collectAVB(body: { grams: number; device?: string; note?: string; date?: string }) {
    return this.write<{ message: string }>('avb/collect', body);
  }

  /** Records AVB used up, drawn from the jar as a whole. */
  useAVB(body: { grams: number; note?: string; date?: string }) {
    return this.write<{ message: string }>('avb/use', body);
  }

  /** The purchases without a price: those still in storage, or every one. */
  unpriced(all: boolean): Promise<Entry[]> {
    return firstValueFrom(this.http.get<Entry[]>(`/api/unpriced${all ? '?all=1' : ''}`));
  }

  /** Writes down what a purchase cost, in euros as typed. */
  price(purchase: string, euros: string) {
    return this.write<{ message: string }>('price', { purchase, euros });
  }

  /** The page for the prescriber, as a PDF, for the days from and to. */
  report(from: string, to: string): Promise<Blob> {
    return firstValueFrom(this.http.get(`/api/report.pdf?from=${from}&to=${to}`, { responseType: 'blob' }));
  }

  /** Records the named stashes as consumed, as one batch. */
  async flush(products: string[]): Promise<{ message: string }> {
    return this.write<{ message: string }>('flush', { products });
  }

  /** The repository being served. */
  where(): Promise<Place> {
    return this.get<Place>('/api/repo');
  }

  /** The folders in a folder, the home folder when none is named. */
  dirs(path?: string): Promise<Listing> {
    return this.get<Listing>(`/api/dirs${path ? `?path=${encodeURIComponent(path)}` : ''}`);
  }

  /** Serves the repository in a folder, or a new one made there; every view then reads it. */
  async open(path: string, create = false): Promise<Place> {
    const place = await this.post<Place>('repo', { path, create });
    this.refresh();
    return place;
  }

  /**
   * What a workbook holds; with commit, imports it into the repository, which
   * must be empty. Sent as the file itself, in the type only a spreadsheet has.
   */
  async importBook(file: Blob, commit: boolean): Promise<Verdict> {
    const type = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
    const out = await this.send<Verdict>(`/api/import${commit ? '?commit=1' : ''}`, new Blob([file], { type }), type);
    if (out.committed) {
      this.refresh();
    }
    return out;
  }

  /** Every view reads again, and the poll starts counting afresh. */
  private refresh(): void {
    this.seen = -1;
    this.version.update((v) => v + 1);
  }

  private async get<T>(url: string): Promise<T> {
    try {
      return await firstValueFrom(this.http.get<T>(url));
    } catch (err) {
      throw said(err);
    }
  }

  private async send<T>(url: string, body: Blob, type: string): Promise<T> {
    try {
      return await firstValueFrom(this.http.post<T>(url, body, { headers: { 'Content-Type': type } }));
    } catch (err) {
      throw said(err);
    }
  }

  /** Writes an entry and refreshes everything; rejects with the server's words. */
  private async write<T = Written>(what: string, body: object): Promise<T> {
    const out = await this.post<T>(what, body);
    this.version.update((v) => v + 1);
    return out;
  }

  /** Posts to the API; rejects with the server's words. */
  private async post<T>(what: string, body: object): Promise<T> {
    try {
      return await firstValueFrom(this.http.post<T>(`/api/${what}`, body));
    } catch (err) {
      throw said(err);
    }
  }
}

/** The server says why in {"error": "…"}; anything else says it itself. */
function said(err: unknown): Error {
  const e = err as HttpErrorResponse;
  const body = e.error as { error?: string } | null;
  return new Error(body?.error ?? e.message, { cause: err });
}
