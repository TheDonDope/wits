/**
 * Reading a pharmacy label. The photograph is read here, on this device, by
 * Tesseract compiled to WebAssembly — its worker, its engine and its German
 * are files this app serves itself, under ocr/ — so a photograph of a
 * prescription leaves the machine for nothing. What comes out is text, and
 * `/api/label` makes a fill of it: the server's catalog, or wits-wasm's on the
 * phone, knows which product it already is.
 */

/** The longest side a photograph is read at: a phone's 4000 px is slower and reads no better. */
const LONGEST = 2400;

/** The text on a photograph of a label. `progress` hears how far the reading is, 0 to 1. */
export async function readLabel(photo: Blob, progress: (done: number) => void): Promise<string> {
  // Loaded on first use: the page that never reads a label never fetches it.
  const loaded = await import('tesseract.js');
  // A CommonJS module: the bundle hands its exports over as the default.
  const { createWorker, OEM } = loaded.default;
  const here = (path: string) => new URL(path, document.baseURI).href;
  const worker = await createWorker('deu', OEM.LSTM_ONLY, {
    workerPath: here('ocr/worker.min.js'),
    corePath: here('ocr/core'),
    langPath: here('ocr/lang'),
    // The service worker and the browser cache keep the files; Tesseract's own
    // cache would keep a second copy in IndexedDB beside the repository's.
    cacheMethod: 'none',
    workerBlobURL: false,
    logger: (m) => {
      if (m.status === 'recognizing text') {
        progress(m.progress);
      }
    },
  });
  try {
    const { data } = await worker.recognize(await scaled(photo));
    return data.text;
  } finally {
    // The engine holds a hundred megabytes; a phone wants them back.
    await worker.terminate();
  }
}

/** The photograph upright, as its camera meant it, and no larger than it needs to be read. */
async function scaled(photo: Blob): Promise<HTMLCanvasElement> {
  const bitmap = await createImageBitmap(photo, { imageOrientation: 'from-image' });
  const k = Math.min(1, LONGEST / Math.max(bitmap.width, bitmap.height));
  const canvas = document.createElement('canvas');
  canvas.width = Math.round(bitmap.width * k);
  canvas.height = Math.round(bitmap.height * k);
  canvas.getContext('2d')?.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close();
  return canvas;
}
