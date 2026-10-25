/**
 * Reading a pharmacy's invoice. The PDF is read here, on this device, by
 * pdf.js — its worker a file this app serves itself, under pdf/ — so an
 * invoice, which names the patient, leaves the machine for nothing. What
 * comes out is text, a row of the invoice a line and its cells apart by tabs,
 * and `/api/invoice` makes a fill of it, as `/api/label` does of a label.
 */

import { InvoiceReading } from './models';

/** Text items closer than this, up or down, are one row of the invoice. */
const ROW = 2;

interface Item {
  str: string;
  x: number;
  y: number;
}

/** The text of a PDF, a row a line, its cells apart by tabs. */
export async function readInvoice(file: Blob): Promise<string> {
  // Loaded on first use: the page that never reads an invoice never fetches it.
  // The legacy build: the modern one needs Promise.try, which Safari has only from iOS 18.2.
  const pdfjs = await import('pdfjs-dist/legacy/build/pdf.mjs');
  pdfjs.GlobalWorkerOptions.workerSrc = new URL('pdf/pdf.worker.min.mjs', document.baseURI).href;
  const task = pdfjs.getDocument({ data: new Uint8Array(await file.arrayBuffer()) });
  try {
    const doc = await task.promise;
    const pages: string[] = [];
    for (let n = 1; n <= doc.numPages; n++) {
      const page = await doc.getPage(n);
      const content = await page.getTextContent();
      const items: Item[] = [];
      for (const it of content.items) {
        if ('str' in it && it.str.trim()) {
          items.push({ str: it.str.trim(), x: it.transform[4] as number, y: it.transform[5] as number });
        }
      }
      pages.push(rows(items));
    }
    return pages.join('\n');
  } finally {
    await task.destroy();
  }
}

/** Items gathered into rows by their baseline, top to bottom, each row left to right. */
export function rows(items: Item[]): string {
  const byRow: { y: number; items: Item[] }[] = [];
  for (const it of items) {
    const row = byRow.find((r) => Math.abs(r.y - it.y) <= ROW);
    if (row) {
      row.items.push(it);
    } else {
      byRow.push({ y: it.y, items: [it] });
    }
  }
  return byRow
    .sort((a, b) => b.y - a.y)
    .map((r) => r.items.sort((a, b) => a.x - b.x).map((i) => i.str).join('\t'))
    .join('\n');
}

/**
 * The invoice's first page as a picture, drawn here like its text is read, to
 * set beside the figures taken from it. An object URL: revoke it when done.
 */
export async function pictureOf(file: Blob, width = 1200): Promise<string> {
  const pdfjs = await import('pdfjs-dist/legacy/build/pdf.mjs');
  pdfjs.GlobalWorkerOptions.workerSrc = new URL('pdf/pdf.worker.min.mjs', document.baseURI).href;
  const task = pdfjs.getDocument({ data: new Uint8Array(await file.arrayBuffer()) });
  try {
    const page = await (await task.promise).getPage(1);
    const viewport = page.getViewport({ scale: width / page.getViewport({ scale: 1 }).width });
    const canvas = document.createElement('canvas');
    canvas.width = Math.round(viewport.width);
    canvas.height = Math.round(viewport.height);
    await page.render({ canvas, viewport }).promise;
    const blob = await new Promise<Blob | null>((done) => {
      canvas.toBlob(done, 'image/png');
    });
    if (!blob) {
      throw new Error('the page could not be drawn');
    }
    return URL.createObjectURL(blob);
  } finally {
    await task.destroy();
  }
}

/** What an invoice charged for besides the flower, in cents: left out of the fill. */
export function leftOut(r: InvoiceReading): number {
  return r.other.reduce((sum, o) => sum + o.total_cents, 0);
}

/** Whether the strains read, and what was left out, come to the invoice's own total. */
export function addsUp(r: InvoiceReading): boolean {
  return !r.gross_cents || r.sum_cents + leftOut(r) === r.gross_cents;
}
