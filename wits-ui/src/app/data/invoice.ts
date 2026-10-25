/**
 * Reading a pharmacy's invoice. The PDF is read here, on this device, by
 * pdf.js — its worker a file this app serves itself, under pdf/ — so an
 * invoice, which names the patient, leaves the machine for nothing. What
 * comes out is text, a row of the invoice a line and its cells apart by tabs,
 * and `/api/invoice` makes a fill of it, as `/api/label` does of a label.
 */

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
