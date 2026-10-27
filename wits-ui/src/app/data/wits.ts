import { HttpClient, HttpErrorResponse, httpResource } from '@angular/common/http';
import { DestroyRef, Injectable, Injector, Signal, effect, inject, runInInjectionContext, signal, untracked } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import { onElsewhere } from './engine';
import { Entries, JournalView } from './entries';
import { ENGINE } from './mode';

/** How long after this page writes the server's word of a change is taken for this page's own write. */
const CHANGE_ECHO = 1500;


import {
  Avb, Costs, Cycle, Findings, Strain, StrainRating, Day, Terpene, Device, DeviceFields, Dimension, EffectLog, Entry, FillLine, InvoiceReading, Jar, LabelReading, Listing, Overview, Place, PriceLine, ProductLabel, UnpricedEntry, Verdict, Written,
} from './models';

/**
 * The repository, as wits-server answers for it.
 *
 * Every view reads through a resource keyed on `version`, made when a page
 * first reads it — a page that never shows the costs never asks for them. A
 * write here bumps the version, and so does the server saying the repository
 * changed (`/api/changes`, a stream): an entry made with the command line or
 * the terminal interface shows up within a second. On the phone the engine
 * runs in the page, and says so itself when another tab has recorded. The
 * journal is kept here and read from where it was left (entries.ts).
 */
@Injectable({ providedIn: 'root' })
export class Wits {
  private readonly http = inject(HttpClient);
  private readonly injector = inject(Injector);
  readonly version = signal(0);
  /** The views made so far, by name. */
  private readonly made = new Map<string, unknown>();
  private entries?: Entries;
  /** When this page last wrote: the change it streams back is its own, already read. */
  private wrote = 0;

  get overview() {
    return this.view('overview', () => httpResource<Overview>(() => `/api/overview?v=${String(this.version())}`));
  }
  get jars() {
    return this.view('jars', () => httpResource<Jar[]>(() => `/api/jars?v=${String(this.version())}`, { defaultValue: [] }));
  }
  get cycles() {
    return this.view('cycles', () => httpResource<Cycle[]>(() => `/api/cycles?v=${String(this.version())}`, { defaultValue: [] }));
  }
  get devices() {
    return this.view('devices', () => httpResource<Device[]>(() => `/api/devices?v=${String(this.version())}`, { defaultValue: [] }));
  }
  get days() {
    return this.view('days', () => httpResource<Day[]>(() => `/api/days?v=${String(this.version())}`, { defaultValue: [] }));
  }
  get effects() {
    return this.view('effects', () => httpResource<EffectLog>(() => `/api/effects?v=${String(this.version())}`));
  }
  get avb() {
    return this.view('avb', () => httpResource<Avb>(() => `/api/avb?v=${String(this.version())}`));
  }
  get costs() {
    return this.view('costs', () => httpResource<Costs>(() => `/api/costs?v=${String(this.version())}`));
  }
  /** The purchases without a price whose jars are still in storage: what is still to be priced. */
  get toPrice() {
    return this.view('toPrice', () => httpResource<UnpricedEntry[]>(() => `/api/unpriced?v=${String(this.version())}`, { defaultValue: [] }));
  }
  /** The repository open: the server's, or the phone's, on its own file system. */
  get place() {
    return this.view('place', () => httpResource<Place>(() => `/api/repo?v=${String(this.version())}`));
  }
  get strains() {
    return this.view('strains', () => httpResource<Strain[]>(() => `/api/strains?v=${String(this.version())}`, { defaultValue: [] }));
  }
  /** Every terpene the shelf names, with its taste and what its products' sessions were rated. */
  get shelfTerpenes() {
    return this.view('terpenes', () => httpResource<Terpene[]>(() => `/api/terpenes?v=${String(this.version())}`, { defaultValue: [] }));
  }
  /** The entries that stand, newest first. */
  get journal(): { value: Signal<JournalView> } {
    return { value: this.kept().standing };
  }
  /** Every entry, corrections included — what a replay folds. Newest first. */
  get everything(): { value: Signal<JournalView> } {
    return { value: this.kept().everything };
  }

  constructor() {
    const stop = ENGINE ? onElsewhere(() => { this.changed(); }) : this.listen();
    inject(DestroyRef).onDestroy(stop);
  }

  /**
   * A view, made the first time a page reads it, in this service's injection
   * context and outside whatever reactive context first read it — a template,
   * a computed — which may not make one.
   */
  private view<T>(name: string, make: () => T): T {
    let v = this.made.get(name) as T | undefined;
    if (v === undefined) {
      v = untracked(() => runInInjectionContext(this.injector, make));
      this.made.set(name, v);
    }
    return v;
  }

  /** The journal, read the first time a page asks for it, and again from where it was left at every change. */
  private kept(): Entries {
    if (!this.entries) {
      const entries = new Entries(this.http);
      this.entries = entries;
      untracked(() => {
        runInInjectionContext(this.injector, () => {
          effect(() => {
            this.version();
            void entries.refresh();
          });
        });
      });
    }
    return this.entries;
  }

  /** Listens to the server for changes, and returns the way to stop. */
  private listen(): () => void {
    if (typeof EventSource === 'undefined') {
      return () => undefined;
    }
    const stream = new EventSource('/api/changes');
    let first = true;
    stream.onmessage = () => {
      // The first event says the stream is open; every one after it is a change.
      if (!first && Date.now() - this.wrote > CHANGE_ECHO) {
        this.changed();
      }
      first = false;
    };
    return () => {
      stream.close();
    };
  }

  /** Reads every view again. */
  private changed(): void {
    this.version.update((v) => v + 1);
  }

  grind(body: { product: string; grams: number; date?: string }) {
    return this.write('grind', body);
  }

  sesh(body: { product: string; grams: number; device?: string; temperature?: number; mode?: string; heater?: string;
    setting?: string; timer?: number; note?: string; date?: string }) {
    return this.write('sesh', body);
  }

  /** What a label's text says about a fill. It records nothing. */
  label(text: string): Promise<LabelReading> {
    return this.post<LabelReading>('label', { text });
  }

  /** A fill of one product or several, each with its price; VAT is added to the prices a gram when asked. */
  fill(body: { lines: FillLine[]; vat: boolean; date?: string; note?: string }) {
    return this.write<{ message: string }>('fill', body);
  }

  /** What an invoice's text says about a fill. It records nothing. */
  invoice(text: string): Promise<InvoiceReading> {
    return this.post<InvoiceReading>('invoice', { text });
  }

  /** Records an entry of the kind named, as the API takes it. */
  submit<T = Written>(what: string, body: object): Promise<T> {
    return this.write<T>(what, body);
  }

  weigh(body: { product: string; account: string; weighed: number }) {
    return this.write('weigh', body);
  }

  /** Records what a reading short of the ledger is put down to: a grind forgotten, or sessions. */
  weighForgotten(body: { product: string; account: string; weighed: number }) {
    return this.write<{ message: string; entries: Entry[] }>('weigh/forgotten', body);
  }

  undo(hash: string) {
    return this.write('undo', { hash });
  }

  /** Corrects an entry's amount: undone, and recorded again as it should have been. */
  amend(hash: string, grams: number, note: string) {
    return this.write<{ message: string; entries: Entry[] }>('amend', { hash, grams, note });
  }

  /** Corrects what a product's label says; its slug, terpenes and rating stay as they are. */
  async describe(slug: string, label: ProductLabel): Promise<{ message: string }> {
    try {
      const out = await firstValueFrom(this.http.put<{ message: string }>(`/api/jars/${encodeURIComponent(slug)}`, label));
      this.version.update((v) => v + 1);
      return out;
    } catch (err) {
      throw said(err);
    }
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
  unpriced(all: boolean): Promise<UnpricedEntry[]> {
    return firstValueFrom(this.http.get<UnpricedEntry[]>(`/api/unpriced${all ? '?all=1' : ''}`));
  }

  /**
   * Writes down a fill's prices: a price a gram for each purchase, VAT added
   * when asked for, or the invoice's line total. One left blank keeps no price.
   */
  priceFill(lines: PriceLine[], vat: boolean, seller = '') {
    return this.write<{ message: string }>('price/fill', { lines, vat, seller });
  }

  /** The page for the prescriber, as a PDF, for the days from and to. */
  report(from: string, to: string): Promise<Blob> {
    return firstValueFrom(this.http.get(`/api/report.pdf?from=${from}&to=${to}`, { responseType: 'blob' }));
  }

  /** Looks a product's terpenes up on the web. It writes nothing down. */
  research(slug: string): Promise<Findings> {
    return this.get<Findings>(`/api/jars/${encodeURIComponent(slug)}/research`);
  }

  /** Writes down a product's terpenes, dominant first; none clears them. */
  async terpenes(slug: string, terpenes: string[]): Promise<{ message: string; terpenes: string[] }> {
    try {
      const out = await firstValueFrom(this.http.put<{ message: string; terpenes: string[] }>(
        `/api/jars/${encodeURIComponent(slug)}/terpenes`, { terpenes }));
      this.version.update((v) => v + 1);
      return out;
    } catch (err) {
      throw said(err);
    }
  }

  /** Rates a strain as a whole, apart from any one session; null takes the rating away. */
  async rateStrain(slug: string, rating: StrainRating | null): Promise<{ message: string; rating?: StrainRating }> {
    try {
      const out = await firstValueFrom(this.http.put<{ message: string; rating?: StrainRating }>(
        `/api/jars/${encodeURIComponent(slug)}/rating`, rating ?? { clear: true }));
      this.version.update((v) => v + 1);
      return out;
    } catch (err) {
      throw said(err);
    }
  }

  /** Puts a device in the catalog. */
  addDevice(device: DeviceFields) {
    return this.write<{ message: string; slug: string }>('devices', device);
  }

  /** Changes what a device is; its slug, which its sessions name it by, stays. */
  async editDevice(slug: string, device: DeviceFields): Promise<{ message: string }> {
    try {
      const out = await firstValueFrom(this.http.put<{ message: string }>(`/api/devices/${encodeURIComponent(slug)}`, device));
      this.version.update((v) => v + 1);
      return out;
    } catch (err) {
      throw said(err);
    }
  }

  /** Takes a device out of the catalog; the sessions it was used for keep its name. */
  async removeDevice(slug: string): Promise<{ message: string }> {
    try {
      // A body, so the request is JSON as every write is: the server takes no other.
      const out = await firstValueFrom(this.http.delete<{ message: string }>(`/api/devices/${encodeURIComponent(slug)}`, { body: {} }));
      this.version.update((v) => v + 1);
      return out;
    } catch (err) {
      throw said(err);
    }
  }

  /** Records the named stashes as consumed, as one batch. */
  async flush(products: string[]): Promise<{ message: string }> {
    return this.write<{ message: string }>('flush', { products });
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

  /**
   * Opens the demo: the bundle this interface ships, restored into a
   * repository of its own beside yours (wits-demo in the home folder), or
   * that repository opened again as it was left.
   */
  async demo(): Promise<Place> {
    const sent = await firstValueFrom(this.http.get(new URL('demo-seed.wits', document.baseURI).href, { responseType: 'blob' }));
    const type = 'application/x-wits-bundle';
    const place = await this.send<Place>('/api/demo', new Blob([sent], { type }), type);
    this.refresh();
    return place;
  }

  /** Every view reads again: another repository is open. The journal kept here starts again by itself, its head not in the new one's. */
  private refresh(): void {
    this.wrote = Date.now();
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
    this.wrote = Date.now();
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
