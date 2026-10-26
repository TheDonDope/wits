// Fills a repository with the demo: the workbook already imported, this
// prices its fills from the pharmacy's invoices, puts every device preset in
// the catalog, records a year of sessions with their ratings and the AVB
// weighed out of each device, looks every jar's terpenes up on the web, and
// clears the stashes left behind — all through wits-server's API, as the
// browser would, so every entry passes the recorder's checks.
//
//   node demo-seed.mjs <wits-server URL> [invoice folder]
//
// Built with esbuild by demo-seed.sh, which imports the workbook first and
// bundles the result. The sessions come from a seeded generator, so a run
// over the same journal records the same ones; the terpenes are whatever the
// web says that day.
import { mkdtempSync, readdirSync, readFileSync, symlinkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

import { PRESETS } from '../src/app/data/devices';
import { readInvoice } from '../src/app/data/invoice';
import { Entry, InvoiceReading, Jar } from '../src/app/data/models';

const [base, invoices] = process.argv.slice(2);
if (!base) {
  throw new Error('usage: demo-seed <wits-server URL> [invoice folder]');
}

/** One API call; rejects with the server's words. */
async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(base + path, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const out = (await res.json()) as T & { error?: string };
  if (!res.ok) {
    throw new Error(`${method} ${path}: ${out.error ?? String(res.status)}`);
  }
  return out;
}

const sleep = (ms: number) => new Promise((go) => setTimeout(go, ms));

/** A seeded generator (mulberry32), so a run records the same sessions. */
function seeded(seed: number): () => number {
  let a = seed;
  return () => {
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rand = seeded(20261026);
const pick = <T>(xs: T[]): T => xs[Math.floor(rand() * xs.length)];
const between = (lo: number, hi: number) => lo + rand() * (hi - lo);

/** Every preset, as Add a device would make it; the habits learn their slugs. */
async function devices(): Promise<void> {
  for (const { preset: _, ...fields } of PRESETS) {
    await api('POST', '/api/devices', fields);
  }
  const made = await api<{ slug: string; name: string }[]>('GET', '/api/devices');
  for (const h of HABITS) {
    h.slug = made.find((d) => d.name === h.name)?.slug ?? '';
    if (!h.slug) {
      throw new Error(`no device named ${h.name}`);
    }
  }
  console.log(`devices: ${String(made.length)}`);
}

/**
 * Every invoice read as the browser reads it, set against its fill, and the
 * fill priced from it — the pharmacy kept, the invoice's number not: this
 * bundle is public, and the number belongs to the invoice, not the demo.
 */
async function prices(): Promise<void> {
  if (!invoices) {
    console.log('invoices: none given, no prices');
    return;
  }
  // pdf.js loads its worker from pdf/ beside the page; here, beside a shim.
  const dir = mkdtempSync(join(tmpdir(), 'wits-pdf-'));
  symlinkSync(join(process.cwd(), 'node_modules/pdfjs-dist/legacy/build'), join(dir, 'pdf'));
  (globalThis as { document?: unknown }).document = { baseURI: pathToFileURL(dir + '/').href };
  let priced = 0;
  for (const name of readdirSync(invoices).filter((f) => f.endsWith('.pdf')).sort()) {
    const text = await quietly(() => readInvoice(new Blob([readFileSync(join(invoices, name))])));
    const reading = await api<InvoiceReading>('POST', '/api/invoice', { text });
    const m = reading.match;
    const lines = m ? reading.lines.flatMap((l, i) => (m.lines[i] ? [{ purchase: m.lines[i], total: (l.total_cents / 100).toFixed(2) }] : [])) : [];
    if (!m || !lines.length || m.priced >= lines.length) {
      console.log(`  ${name}: ${m ? 'already priced' : 'no fill'}`);
      continue;
    }
    await api('POST', '/api/price/fill', { lines, vat: false, seller: reading.seller ?? '' });
    priced++;
  }
  console.log(`invoices: ${String(priced)} fills priced`);
}

/**
 * Runs work with pdf.js's warnings kept off the console: without a browser it
 * has no standard fonts, which drawing needs and reading text does not.
 */
async function quietly<T>(work: () => Promise<T>): Promise<T> {
  const log = console.log;
  console.log = (...args: unknown[]) => {
    if (!String(args[0]).startsWith('Warning:')) {
      log(...args);
    }
  };
  try {
    return await work();
  } finally {
    console.log = log;
  }
}

/** How a device is used in the demo: how often, at what heat, in which modes. */
interface Habit {
  name: string;
  slug: string;
  weight: number;
  temps: number[];
  modes?: { name: string; temp: number; timer?: number }[];
}

const HABITS: Habit[] = [
  { name: 'Storz & Bickel Venty', slug: '', weight: 45, temps: [175, 180, 185, 190, 195, 200] },
  { name: 'Storz & Bickel Mighty+', slug: '', weight: 30, temps: [180, 185, 190, 195] },
  { name: 'Limelight Herb Frolic', slug: '', weight: 25, temps: [],
    modes: [{ name: 'On demand', temp: 165 }, { name: 'Session', temp: 185, timer: 420 }] },
];

/** A device by habit, weighted. */
function device(): Habit {
  let r = rand() * HABITS.reduce((s, h) => s + h.weight, 0);
  for (const h of HABITS) {
    r -= h.weight;
    if (r < 0) {
      return h;
    }
  }
  return HABITS[0];
}

/** When in the day a session falls: mostly the evening. */
function hour(): number {
  const r = rand();
  return r < 0.1 ? Math.floor(between(8, 11)) : r < 0.3 ? Math.floor(between(13, 18)) : Math.floor(between(19, 24));
}

/** A product's own quality, the same every time it is asked. */
const quality = new Map<string, number>();
function qualityOf(product: string): number {
  if (!quality.has(product)) {
    quality.set(product, between(2.9, 4.5));
  }
  return quality.get(product) ?? 3.5;
}

/**
 * What a session did: overall from the product, the heat and the hour, and
 * two or three of what it was for — sleep at night, focus in the morning.
 */
function scores(product: string, temp: number, h: number): Record<string, number> {
  const heat = temp >= 183 && temp <= 197 ? 0.4 : temp > 197 ? -0.3 : -0.1;
  const clamp = (v: number) => Math.min(5, Math.max(1, Math.round(v + between(-0.8, 0.8))));
  const q = qualityOf(product) + heat;
  const out: Record<string, number> = { overall: clamp(q) };
  const night = h >= 19;
  const morning = h < 12;
  const asks = night ? ['sleep', 'calm', 'pain'] : morning ? ['focus', 'mood', 'pain'] : ['pain', 'mood', 'appetite', 'calm'];
  for (const d of asks.slice(0, 2 + Math.floor(rand() * 2))) {
    const lift = (d === 'sleep' && night) || (d === 'focus' && morning) ? 0.7 : d === 'focus' && night ? -0.8 : 0;
    out[d] = clamp(q + lift + (d === 'calm' && temp < 183 ? 0.5 : 0));
  }
  return out;
}

/** A date and time as the API takes it. */
function stamp(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${String(d.getFullYear())}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}

/** What has gone through a device since its AVB was last weighed out. */
const since = new Map<string, { grams: number; sessions: number; last: Date }>();

/** Weighs a device's AVB out now and then: about half of what went in. */
async function collect(slug: string, force = false): Promise<boolean> {
  const s = since.get(slug);
  if (!s || (!force && s.sessions < 10 + Math.floor(rand() * 5))) {
    return false;
  }
  const at = new Date(Math.min(s.last.getTime() + 3_600_000, Date.now() - 60_000));
  const grams = Math.round(s.grams * between(0.42, 0.58) * 100) / 100;
  await api('POST', '/api/avb/collect', { grams, device: slug, date: stamp(at) });
  since.delete(slug);
  return true;
}

/**
 * A year of sessions: after every grind of the last twelve months, two to four
 * sessions of what was ground over the next days, most of them rated, each
 * device's AVB weighed out every dozen or so. A session the recorder refuses
 * is left out and counted.
 */
async function sessions(): Promise<void> {
  const grinds = (await api<{ entries: Entry[] }>('GET', '/api/journal?type=grind&limit=100000')).entries
    .filter((e) => Date.parse(e.occurred_at) > Date.now() - 365 * 86_400_000)
    .reverse();
  let made = 0, rated = 0, refused = 0, collected = 0;
  for (const g of grinds) {
    const product = g.product ?? '';
    const n = Math.max(1, Math.min(4, Math.round((g.grams * between(0.55, 0.85)) / 0.25)));
    for (let i = 0; i < n; i++) {
      const at = new Date(g.occurred_at);
      at.setDate(at.getDate() + Math.floor(rand() * 3));
      at.setHours(hour(), Math.floor(rand() * 60), 0, 0);
      if (at.getTime() > Date.now() - 60_000) {
        continue;
      }
      const d = device();
      const mode = d.modes ? pick(d.modes) : undefined;
      const temp = mode ? mode.temp : pick(d.temps);
      const grams = Math.round(between(0.15, 0.32) * 100) / 100;
      try {
        const out = await api<{ entry: Entry }>('POST', '/api/sesh', {
          product, grams, device: d.slug, temperature: mode ? 0 : temp, mode: mode?.name ?? '', timer: mode?.timer ?? 0, date: stamp(at),
        });
        made++;
        const s = since.get(d.slug) ?? { grams: 0, sessions: 0, last: at };
        since.set(d.slug, { grams: s.grams + grams, sessions: s.sessions + 1, last: at > s.last ? at : s.last });
        if (rand() < 0.82) {
          await api('POST', '/api/rate', { session: out.entry.hash, scores: scores(product, temp, at.getHours()), note: '' });
          rated++;
        }
        if (await collect(d.slug)) {
          collected++;
        }
      } catch {
        refused++;
      }
    }
  }
  console.log(`sessions: ${String(made)} recorded, ${String(rated)} rated, ${String(collected)} AVB collections, ${String(refused)} refused`);
}

/** Every jar's terpenes looked up, and what most pages name kept. */
async function terpenes(): Promise<void> {
  const jars = (await api<Jar[]>('GET', '/api/jars')).filter((j) => j.bought > 0 && !j.terpenes.length);
  let kept = 0;
  const none: string[] = [];
  for (const j of jars) {
    let found: string[] = [];
    for (let attempt = 0; attempt < 3 && !found.length; attempt++) {
      try {
        found = (await api<{ suggested: string[] }>('GET', `/api/jars/${encodeURIComponent(j.slug)}/research`)).suggested;
        if (!found.length) {
          break;
        }
      } catch {
        await sleep(20_000);
      }
    }
    if (found.length) {
      await api('PUT', `/api/jars/${encodeURIComponent(j.slug)}/terpenes`, { terpenes: found });
      kept++;
    } else {
      none.push(j.name);
    }
    await sleep(1500);
  }
  console.log(`terpenes: ${String(kept)} of ${String(jars.length)} jars${none.length ? `; none found for ${none.join(', ')}` : ''}`);
}

/** The stashes of jars with nothing sealed left, cleared as I forgot to clears them. */
async function stashes(): Promise<void> {
  const left = (await api<Jar[]>('GET', '/api/jars')).filter((j) => j.storage <= 0.004 && j.stash > 0);
  if (left.length) {
    await api('POST', '/api/flush', { products: left.map((j) => j.slug) });
  }
  console.log(`stashes: ${String(left.length)} cleared`);
}

await devices();
await prices();
await sessions();
await terpenes();
await stashes();
