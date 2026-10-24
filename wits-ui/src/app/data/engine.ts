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
  rechained: number;
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
  request(method: string, url: string, body: string): Promise<{ status: number; body: string }>;
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
}

const g = globalThis as unknown as Globals;
const DB = 'wits';
const STORE = 'files';

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

/** Only the repository is kept: the stand-in folder and temporary files are not, nor the empty locks. */
export function kept(path: string): boolean {
  return path.startsWith('/wits/') && !path.endsWith('.lock');
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
      await save(db, fs, paths);
    }
  };
  return {
    request: (method, url, body) => flush(inner.request(method, url, body)),
    sync: (file, key) => flush(inner.sync(file, key)),
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
  const body = req.body === null ? '' : JSON.stringify(req.body);
  const res = await e.request(req.method, req.urlWithParams, body);
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
