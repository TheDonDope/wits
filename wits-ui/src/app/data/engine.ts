import { HttpErrorResponse, HttpInterceptorFn, HttpRequest, HttpResponse } from '@angular/common/http';
import { from, mergeMap, of, throwError } from 'rxjs';

import { MemFS } from './memfs';
import { ENGINE } from './mode';

/**
 * wits on the phone: wits-wasm — the repository, the recorder and the API
 * wits-server runs — compiled to WebAssembly and run in the page, on MemFS,
 * with every file it writes kept in IndexedDB. The interceptor below answers
 * `/api/` from it, so every page reads and records exactly as it does against
 * the server, refused the same things in the same words.
 */

/** What a pull brought in, as wits-wasm reports it. */
export interface Pulled {
  fetched: number;
  joined: boolean;
  ratings: number;
  prices: number;
  products: string;
  devices: string;
  clashes: { hash: string; text: string }[];
}

/** A sync: what was pulled, what was pushed, and the file to save back — none when there is nothing yet. */
export interface Synced {
  pulled: Pulled;
  pushed: { events: number; ratings: number; prices: number };
  file: Uint8Array | null;
}

interface GoEngine {
  request(
    method: string,
    url: string,
    body: string | Uint8Array,
    type?: string,
  ): Promise<{ status: number; type: string; body: string; bytes?: Uint8Array }>;
  sync(file: Uint8Array | null, key: string): Promise<Synced>;
  key(): Promise<string>;
}

interface GoRuntime {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

/** What wasm_exec.js and wits-wasm put on the page. */
interface Globals {
  fs?: MemFS;
  process?: { cwd(): string };
  Go?: new () => GoRuntime;
  witsEngine?: GoEngine;
  witsReady?: () => void;
  witsReadWorkbook?: (bytes: Uint8Array) => { workbook?: string; error?: string };
  witsXLSXReady?: () => void;
}

/** A .xlsx file, as a request's body is typed. */
const XLSX = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
/** A workbook already read, as the engine takes it (pkg/api/desk.go). */
const WORKBOOK = 'application/vnd.wits.workbook+json';
let reader: Promise<NonNullable<Globals['witsReadWorkbook']>> | undefined;

/**
 * The phone's .xlsx reader (cmd/wits-xlsx), started the first time a
 * workbook is imported: the engine is built without one, 0.9 MB gzipped
 * lighter on every start, and this is fetched only for the import.
 */
function xlsxReader(): Promise<NonNullable<Globals['witsReadWorkbook']>> {
  reader ??= (async () => {
    if (!g.Go) {
      throw new Error('wasm_exec.js is not loaded');
    }
    const ready = new Promise<void>((resolve) => {
      g.witsXLSXReady = resolve;
    });
    const go = new g.Go();
    const { instance } = await WebAssembly.instantiateStreaming(fetch(new URL('wits-xlsx.wasm', document.baseURI)), go.importObject);
    void go.run(instance);
    await ready;
    if (!g.witsReadWorkbook) {
      throw new Error('wits-xlsx did not start');
    }
    return g.witsReadWorkbook;
  })();
  return reader;
}

/** A workbook's bytes read into what the engine takes, by the reader module. */
async function workbookOf(file: Blob): Promise<string> {
  const read = await xlsxReader();
  const out = read(new Uint8Array(await file.arrayBuffer()));
  if (out.error !== undefined || out.workbook === undefined) {
    throw new Error(`not a workbook wits can read: ${out.error ?? 'nothing read'}`);
  }
  return out.workbook;
}

const g = globalThis as unknown as Globals;
const DB = 'wits';
const STORE = 'files';
/** The lock the one page that records holds for as long as it is open. */
const LOCK = 'wits-engine';

/**
 * Whether this page records. Two pages — the installed app and a tab, say —
 * each run an engine on their own copy of the files, and the one that saved
 * last would write the other's entries out of IndexedDB. So one page holds
 * the lock and records; any other reads, is told when the first has recorded,
 * and takes over when it closes.
 */
let writer = true;
const changed = new Set<() => void>();
const channel = typeof BroadcastChannel === 'undefined' ? null : new BroadcastChannel('wits');

/** Calls back whenever another page has recorded, or this one has taken over; returns the way to stop. */
export function onElsewhere(fn: () => void): () => void {
  changed.add(fn);
  return () => changed.delete(fn);
}

/** Why a page that does not hold the lock refuses to record. */
export const ELSEWHERE = 'wits is open in another tab or window, which records. Record there, or close it and this one takes over.';

/** Whether this page records. */
export function records(): boolean {
  return writer;
}

/** Tells every page listening that the files changed, and reads them again. */
function reread(db: IDBDatabase, fs: MemFS): void {
  void load(db, fs).then(() => {
    for (const fn of changed) {
      fn();
    }
  });
}

/**
 * Takes the lock if no page holds it, and holds it for the page's life; if
 * one does, waits for it to close, then reads the files again and records.
 * A browser without Web Locks has one writer by trust, as before.
 */
async function claim(db: IDBDatabase, fs: MemFS): Promise<boolean> {
  const locks = (navigator as { locks?: LockManager }).locks;
  if (!locks) {
    return true;
  }
  const forever = () => new Promise<never>(() => undefined);
  const got = await new Promise<boolean>((resolve) => {
    void locks.request(LOCK, { ifAvailable: true }, (lock) => {
      resolve(lock !== null);
      return lock ? forever() : undefined;
    });
  });
  if (!got) {
    void locks.request(LOCK, () => {
      writer = true;
      reread(db, fs);
      return forever();
    });
  }
  return got;
}

/** Opens the IndexedDB the repository's files are kept in. */
function database(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const open = indexedDB.open(DB, 1);
    open.onupgradeneeded = () => open.result.createObjectStore(STORE);
    open.onsuccess = () => {
      resolve(open.result);
    };
    open.onerror = () => {
      reject(open.error ?? new Error('IndexedDB would not open'));
    };
  });
}

/** Puts every kept file back on the file system. */
async function load(db: IDBDatabase, fs: MemFS): Promise<void> {
  const store = db.transaction(STORE).objectStore(STORE);
  const [keys, values] = await Promise.all([request(store.getAllKeys()), request(store.getAll())]);
  keys.forEach((key, i) => {
    fs.put(key as string, values[i] as Uint8Array);
  });
}

/** Writes the files that changed, and forgets the ones that went, in one go. */
function save(db: IDBDatabase, fs: MemFS, paths: Iterable<string>): Promise<void> {
  const tx = db.transaction(STORE, 'readwrite');
  const store = tx.objectStore(STORE);
  for (const path of paths) {
    const data = fs.get(path);
    if (data) {
      store.put(data, path);
    } else {
      store.delete(path);
    }
  }
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => {
      resolve();
    };
    tx.onerror = () => {
      reject(tx.error ?? new Error('IndexedDB would not write'));
    };
  });
}

function request<T>(r: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    r.onsuccess = () => {
      resolve(r.result);
    };
    r.onerror = () => {
      reject(r.error ?? new Error('IndexedDB would not read'));
    };
  });
}

/**
 * Every repository is kept, wherever the Repository page made it, and which
 * one was open: the stand-in folder and temporary files are not, nor the
 * empty locks.
 */
export function kept(path: string): boolean {
  return !path.startsWith('/.remote/') && !path.startsWith('/.tmp/') && !path.endsWith('.lock');
}

function script(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const el = document.createElement('script');
    el.src = new URL(src, document.baseURI).href;
    el.onload = () => {
      resolve();
    };
    el.onerror = () => {
      reject(new Error(`${src} would not load`));
    };
    document.head.append(el);
  });
}

/** Starts wits-wasm on the kept files, and keeps every file it changes. */
async function start(): Promise<GoEngine> {
  const db = await database();
  const fs = new MemFS();
  await load(db, fs);
  writer = await claim(db, fs);
  channel?.addEventListener('message', () => {
    if (!writer) {
      reread(db, fs);
    }
  });
  const dirty = new Set<string>();
  fs.onChange = (path) => {
    if (kept(path)) {
      dirty.add(path);
    }
  };
  g.fs = fs;
  await script('wasm_exec.js');
  if (g.process) {
    g.process.cwd = () => '/';
  }
  if (!g.Go) {
    throw new Error('wasm_exec.js did not define Go');
  }
  const ready = new Promise<void>((resolve) => {
    g.witsReady = resolve;
  });
  const go = new g.Go();
  const { instance } = await WebAssembly.instantiateStreaming(fetch(new URL('wits.wasm', document.baseURI)), go.importObject);
  void go.run(instance);
  await ready;
  // Health data: ask the browser not to clear it to make room.
  void navigator.storage.persist().catch(() => false);
  const inner = g.witsEngine;
  if (!inner) {
    throw new Error('wits-wasm did not start');
  }
  const flush = async <T>(work: Promise<T>): Promise<T> => {
    try {
      return await work;
    } finally {
      const paths = [...dirty];
      dirty.clear();
      // A page that reads keeps nothing: its files may be older than the writer's.
      if (writer && paths.length) {
        await save(db, fs, paths);
        channel?.postMessage('changed');
      }
    }
  };
  return {
    request: (method, url, body, type) => flush(inner.request(method, url, body, type)),
    sync: (file, key) => (writer ? flush(inner.sync(file, key)) : Promise.reject(new Error(ELSEWHERE))),
    key: () => inner.key(),
  };
}

let started: Promise<GoEngine> | undefined;

/** The engine, started on first use. */
export function engine(): Promise<GoEngine> {
  started ??= start();
  return started;
}

/** Answers one API call from the engine, the way wits-server would. */
async function answer(req: HttpRequest<unknown>): Promise<HttpResponse<unknown> | HttpErrorResponse> {
  const e = await engine();
  if (!writer && req.method !== 'GET') {
    return new HttpErrorResponse({ status: 409, error: { error: ELSEWHERE }, url: req.url });
  }
  // A workbook is read by the reader module first, and goes across as what it
  // read; any other bytes go as they are, in their own type; anything else is JSON.
  let res: Awaited<ReturnType<GoEngine['request']>>;
  if (req.body instanceof Blob && req.body.type === XLSX) {
    try {
      res = await e.request(req.method, req.urlWithParams, await workbookOf(req.body), WORKBOOK);
    } catch (err) {
      return new HttpErrorResponse({ status: 400, error: { error: (err as Error).message }, url: req.url });
    }
  } else if (req.body instanceof Blob) {
    res = await e.request(req.method, req.urlWithParams, new Uint8Array(await req.body.arrayBuffer()), req.body.type);
  } else {
    res = await e.request(req.method, req.urlWithParams, req.body === null ? '' : JSON.stringify(req.body));
  }
  if (res.bytes) {
    // The report: bytes, not JSON, handed over as the blob that was asked for.
    return new HttpResponse({ status: res.status, body: new Blob([res.bytes.slice()], { type: res.type }), url: req.url });
  }
  const parsed: unknown = res.body ? JSON.parse(res.body) : null;
  if (res.status >= 400) {
    return new HttpErrorResponse({ status: res.status, error: parsed, url: req.url });
  }
  return new HttpResponse({ status: res.status, body: parsed, url: req.url });
}

/** Sends `/api/` to the engine in the phone's build; the server's build is untouched. */
export const engineInterceptor: HttpInterceptorFn = (req, next) => {
  if (!ENGINE || !req.url.startsWith('/api/')) {
    return next(req);
  }
  // An answer of 400 or more fails the request, as one from the server does.
  return from(answer(req)).pipe(
    mergeMap((res): ReturnType<typeof next> => (res instanceof HttpErrorResponse ? throwError(() => res) : of(res))),
  );
};
