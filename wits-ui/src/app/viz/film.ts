import { FilmFrame } from '../data/film';

/** A film's size and pace: 720p, thirty frames a second. */
export const FILM = { width: 1280, height: 720, fps: 30 };

/**
 * The theme's colours, read off the page: a canvas cannot resolve a CSS
 * variable, so the film is painted in the theme that was showing when it was
 * made. Glass and its edge are the text colour at low alpha, as on the page.
 */
function colours() {
  const css = getComputedStyle(document.documentElement);
  const v = (name: string, fallback: string) => css.getPropertyValue(name).trim() || fallback;
  const ink = v('--w-fg', '#e6e9ef');
  return { bg: v('--w-ink', '#0b0e14'), ink, dim: v('--w-dim', '#7d879c'), mint: v('--w-accent', '#7cf5c4'),
    storage: v('--w-storage', '#6ea8ff'), stash: v('--w-stash', '#34d39a') };
}

/**
 * Draws one frame on a canvas: the day, the shelf's grams, and every jar on
 * it, its sealed grams blue under its ground green, filled to what it holds
 * against the most it ever held — the jars as the Jars page draws them,
 * flattened to a frame.
 */
export function drawFrame(ctx: CanvasRenderingContext2D, f: FilmFrame, names: Map<string, string>,
  capacity: Map<string, number>): void {
  const { width: W, height: H } = FILM;
  const pal = colours();
  ctx.fillStyle = pal.bg;
  ctx.fillRect(0, 0, W, H);
  ctx.fillStyle = pal.dim;
  ctx.font = '600 22px system-ui, sans-serif';
  ctx.fillText('wits · the shelf', 56, 64);
  ctx.fillStyle = pal.ink;
  ctx.font = '700 56px system-ui, sans-serif';
  ctx.fillText(new Date(`${f.day}T12:00`).toLocaleDateString('en-GB', { month: 'long', year: 'numeric' }), 56, 128);
  ctx.fillStyle = pal.mint;
  ctx.font = '600 30px system-ui, sans-serif';
  ctx.textAlign = 'right';
  ctx.fillText(`${f.shelf.toFixed(2)} g sealed`, W - 56, 120);
  ctx.textAlign = 'left';
  const jars = f.jars.slice(-10);
  const slot = (W - 112) / Math.max(jars.length, 5);
  jars.forEach((j, i) => {
    const x = 56 + i * slot + slot * 0.2;
    jar(ctx, pal, { x, y: 190, w: slot * 0.6, h: 400 }, j.storage, j.stash, capacity.get(j.slug) ?? 1);
    ctx.fillStyle = pal.ink;
    ctx.font = '500 15px system-ui, sans-serif';
    wrap(ctx, names.get(j.slug) ?? j.slug, x - slot * 0.15, 620, slot * 0.9);
    ctx.fillStyle = pal.dim;
    ctx.fillText(`${j.storage.toFixed(1)} g`, x, 600);
  });
}

/** Where a jar stands on the frame, and its size. */
interface Box { x: number; y: number; w: number; h: number }

/** One jar: glass, then its sealed grams, then its ground ones above them. */
function jar(ctx: CanvasRenderingContext2D, pal: ReturnType<typeof colours>, box: Box,
  storage: number, stash: number, cap: number): void {
  const { x, y, w, h } = box;
  ctx.fillStyle = pal.ink;
  ctx.strokeStyle = pal.ink;
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.roundRect(x, y, w, h, 14);
  ctx.globalAlpha = 0.07;
  ctx.fill();
  ctx.globalAlpha = 0.22;
  ctx.stroke();
  ctx.globalAlpha = 1;
  const s = (Math.min(storage, cap) / cap) * (h - 8);
  const g = (Math.min(stash, Math.max(cap - storage, 0)) / cap) * (h - 8);
  ctx.fillStyle = pal.storage;
  ctx.fillRect(x + 4, y + h - 4 - s, w - 8, s);
  ctx.fillStyle = pal.stash;
  ctx.fillRect(x + 4, y + h - 4 - s - g, w - 8, g);
}

/** Two lines of a name at most, cut where a word ends. */
function wrap(ctx: CanvasRenderingContext2D, text: string, x: number, y: number, width: number): void {
  const words = text.split(' ');
  const lines: string[] = [];
  let line = '';
  for (const w of words) {
    const tried = line ? `${line} ${w}` : w;
    if (ctx.measureText(tried).width > width && line) {
      lines.push(line);
      line = w;
    } else {
      line = tried;
    }
  }
  lines.push(line);
  lines.slice(0, 2).forEach((l, i) => {
    ctx.fillText(l, x, y + i * 19);
  });
}

/**
 * The container the browser records into: WebM in VP8, which every browser
 * but Safari records, and MP4 for Safari. MP4 first was tried: Chromium
 * without its proprietary codecs says it can record one and then fails part
 * way. VP9 was tried too: in real time on a slow machine its software encoder
 * fell behind and gave up twenty seconds in; VP8 is far lighter to encode.
 */
export function filmType(): string {
  for (const t of ['video/webm;codecs=vp8', 'video/webm', 'video/mp4']) {
    if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(t)) {
      return t;
    }
  }
  return '';
}

/**
 * Records the frames as a video, a frame each thirtieth of a second, the way
 * the browser records a canvas: in real time, so a thirty-second film takes
 * thirty seconds. progress is told the share done.
 */
export function record(frames: FilmFrame[], names: Map<string, string>, capacity: Map<string, number>,
  progress: (done: number) => void): Promise<Blob> {
  const type = filmType();
  if (!type) {
    return Promise.reject(new Error('this browser cannot record a video'));
  }
  const canvas = document.createElement('canvas');
  canvas.width = FILM.width;
  canvas.height = FILM.height;
  const ctx = canvas.getContext('2d');
  if (!ctx) {
    return Promise.reject(new Error('this browser cannot draw the frames'));
  }
  drawFrame(ctx, frames[0], names, capacity);
  const recorder = new MediaRecorder(canvas.captureStream(FILM.fps), { mimeType: type, videoBitsPerSecond: 2_500_000 });
  const chunks: Blob[] = [];
  recorder.ondataavailable = (e) => chunks.push(e.data);
  return new Promise((resolve, reject) => {
    recorder.onstop = () => {
      const kind = type.split(';')[0];
      const raw = new Blob(chunks, { type: kind });
      if (kind !== 'video/webm') {
        resolve(raw);
        return;
      }
      void raw.arrayBuffer().then((b) => {
        const ms = (frames.length / FILM.fps) * 1000;
        resolve(new Blob([withDuration(new Uint8Array(b), ms).slice()], { type: kind }));
      }, reject);
    };
    recorder.onerror = (e) => {
      const cause = (e as Event & { error?: DOMException }).error;
      reject(new Error(`the recording failed${cause ? `: ${cause.message}` : ''}`));
    };
    recorder.start(1000);
    const started = performance.now();
    const tick = () => {
      const i = Math.floor(((performance.now() - started) / 1000) * FILM.fps);
      if (i >= frames.length) {
        recorder.stop();
        return;
      }
      drawFrame(ctx, frames[i], names, capacity);
      progress(i / frames.length);
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });
}

/**
 * Writes the duration into a WebM the browser recorded without one. A
 * MediaRecorder cannot know how long it will record, so its WebM has no
 * duration, and players show 0:00 and cannot seek. The header's Info element
 * gets a Duration in its time scale's units — milliseconds, as browsers write
 * it — and its size grows by the element's eleven bytes. A file that is not
 * laid out that way is handed back as it was.
 */
export function withDuration(webm: Uint8Array, ms: number): Uint8Array {
  const info = find(webm, [0x15, 0x49, 0xa9, 0x66]);
  if (info < 0) {
    return webm;
  }
  const size = vint(webm, info + 4);
  if (!size || find(webm.subarray(info, info + 4 + size.length + size.value), [0x44, 0x89]) >= 0) {
    return webm;
  }
  const duration = new Uint8Array(11);
  duration.set([0x44, 0x89, 0x88]);
  new DataView(duration.buffer).setFloat64(3, ms);
  const grown = size.value + duration.length;
  if (grown >= 2 ** (7 * size.length) - 1) {
    return webm;
  }
  const header = new Uint8Array(size.length);
  for (let i = size.length - 1, v = grown; i >= 0; i--, v = Math.floor(v / 256)) {
    header[i] = v & 0xff;
  }
  header[0] |= 0x80 >> (size.length - 1);
  const start = info + 4 + size.length;
  const out = new Uint8Array(webm.length + duration.length);
  out.set(webm.subarray(0, info + 4));
  out.set(header, info + 4);
  out.set(duration, start);
  out.set(webm.subarray(start), start + duration.length);
  return out;
}

/** The first place the bytes occur, or -1. */
function find(data: Uint8Array, bytes: number[]): number {
  outer: for (let i = 0; i + bytes.length <= Math.min(data.length, 4096); i++) {
    for (let j = 0; j < bytes.length; j++) {
      if (data[i + j] !== bytes[j]) {
        continue outer;
      }
    }
    return i;
  }
  return -1;
}

/** An EBML variable-length size at i: its value, and how many bytes it takes. */
function vint(data: Uint8Array, i: number): { value: number; length: number } | null {
  const first = data[i];
  let length = 1;
  while (length <= 8 && !(first & (0x80 >> (length - 1)))) {
    length++;
  }
  if (length > 8) {
    return null;
  }
  let value = first & (0xff >> length);
  for (let k = 1; k < length; k++) {
    value = value * 256 + data[i + k];
  }
  return { value, length };
}
