// Runs wits-wasm on MemFS under Node, the way the phone runs it, and syncs it
// with a real repository through the CLI: what the phone records, wits pull
// reads, and the other way round.
//
//   node scripts/engine-check.mjs <wits.wasm> <wasm_exec.js> <share dir> [key file]
//
// The engine's files are kept in <share dir>/../phone.json between runs, as
// the page keeps them in IndexedDB, so a second run is the same phone again.
//
// With a key, the share's wits.age is pulled first; without one, the engine
// is the first device. Either way it grinds once and pushes, leaving the
// share's file for the CLI to pull, and prints what it saw as JSON.
//
// With WITS_WORKBOOK naming a workbook, it then makes a second repository in a
// new folder, imports the workbook into it as the Repository page does, and
// opens the first again.
import { readFile, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';

import { MemFS } from '../src/app/data/memfs.ts';

const [wasmPath, execPath, share, keyFile] = process.argv.slice(2);
const memfs = new MemFS();
const state = join(dirname(share), 'phone.json');
try {
  for (const [path, data] of Object.entries(JSON.parse(await readFile(state, 'utf8')))) {
    memfs.put(path, Buffer.from(data, 'base64'));
  }
} catch {
  // The phone's first run.
}
globalThis.fs = memfs;
await import(execPath);

const ready = new Promise((resolve) => (globalThis.witsReady = resolve));
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(await readFile(wasmPath), go.importObject);
void go.run(instance);
await ready;
const engine = globalThis.witsEngine;

const call = async (method, url, body = '') => {
  const res = await engine.request(method, url, body);
  return { status: res.status, body: res.body ? JSON.parse(res.body) : null };
};

const out = {};
const key = keyFile ? (await readFile(keyFile, 'utf8')).trim() : '';
const opened = keyFile ? new Uint8Array(await readFile(join(share, 'wits.age'))) : null;
// A key the phone already holds need not be given again.
const first = await engine.sync(opened, key);
out.pulled = first.pulled;
if (!keyFile) {
  out.bought = (await call('POST', '/api/fill', JSON.stringify({ lines: [{ name: 'Enua 22/1 Wedding Cake', grams: 20 }] }))).status;
}
const jars = (await call('GET', '/api/jars')).body;
const jar = jars.find((j) => j.storage > 1);
out.ground = (await call('POST', '/api/grind', JSON.stringify({ product: jar.slug, grams: 1 }))).status;
out.refused = (await call('POST', '/api/grind', JSON.stringify({ product: jar.slug, grams: 100000 }))).body;
// The report crosses as bytes: read as text it would no longer be a PDF.
const pdf = await engine.request('GET', '/api/report.pdf', '');
out.report = { status: pdf.status, type: pdf.type, head: pdf.bytes ? Buffer.from(pdf.bytes.slice(0, 5)).toString() : '',
  tail: pdf.bytes ? Buffer.from(pdf.bytes.slice(-6)).toString().trim() : '' };
const second = await engine.sync(null, '');
out.pushed = second.pushed;
out.overview = (await call('GET', '/api/journal?limit=1')).body.total;
await writeFile(join(share, 'wits.age'), second.file);
if (process.env.WITS_WORKBOOK) {
  out.book = await book(process.env.WITS_WORKBOOK);
}
if (!keyFile) {
  await writeFile(join(share, 'phone.key'), (await engine.key()) + '\n', { mode: 0o600 });
}
out.files = memfs.paths().sort();
const kept = {};
// data/engine.ts's kept: what the page keeps in IndexedDB.
const keep = (p) => !p.startsWith('/.remote/') && !p.startsWith('/.tmp/') && !p.endsWith('.lock');
for (const path of out.files.filter(keep)) {
  kept[path] = Buffer.from(memfs.get(path)).toString('base64');
}
await writeFile(state, JSON.stringify(kept), { mode: 0o600 });
console.log(JSON.stringify(out, null, 1));
process.exit(0);

/** A repository in a new folder, the workbook read and imported into it, and the first opened again. */
async function book(path) {
  const xlsx = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
  const data = new Uint8Array(await readFile(path));
  const made = await call('POST', '/api/repo', JSON.stringify({ path: '/book', create: true }));
  const read = await engine.request('POST', '/api/import', data, xlsx);
  const imported = await engine.request('POST', '/api/import?commit=1', data, xlsx);
  const place = (await call('GET', '/api/repo')).body;
  const listed = (await call('GET', '/api/dirs')).body.dirs.map((d) => d.name);
  const back = (await call('POST', '/api/repo', JSON.stringify({ path: '/wits' }))).body;
  return { made: made.status, read: JSON.parse(read.body).entries, committed: JSON.parse(imported.body).committed,
    entries: place.entries, listed, back: back.path };
}
