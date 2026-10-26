import { HttpClient } from '@angular/common/http';
import { Observable, of } from 'rxjs';
import { describe, expect, it } from 'vitest';

import { Entries } from './entries';
import { Entry } from './models';

function entry(hash: string, day: string, extra: Partial<Entry> = {}): Entry {
  return {
    hash, type: 'grind', occurred_at: `2026-10-${day}T12:00:00Z`, recorded_at: '', product: 'w', product_name: 'Old name',
    grams: 1, from: 'storage', to: 'stash', ...extra,
  };
}

/** A stand-in for the server: answers each read with the next reply, and keeps what was asked. */
function server(replies: object[]): { http: HttpClient; asked: string[] } {
  const asked: string[] = [];
  const http = {
    get: (url: string): Observable<object> => {
      asked.push(url);
      return of(replies.shift() ?? {});
    },
  } as unknown as HttpClient;
  return { http, asked };
}

describe('Entries', () => {
  it('reads from where it left off, orders as the fold does, and leaves an undone pair out', async () => {
    const { http, asked } = server([
      { reset: true, head: 'b', total: 2, names: { w: 'Wedding Cake' }, entries: [entry('a', '20'), entry('b', '22')] },
      // Recorded after, but dated before: a backdated grind, and an undo of 'b'.
      { reset: false, head: 'd', total: 4, names: { w: 'Wedding Cake 22/1' }, entries: [entry('c', '21'), entry('d', '23', { type: 'adjust', reverts: 'b' })] },
    ]);
    const entries = new Entries(http);

    await entries.refresh();
    expect(entries.everything().entries.map((e) => e.hash)).toEqual(['b', 'a']);
    await entries.refresh();

    expect(asked).toEqual(['/api/journal?after=', '/api/journal?after=b']);
    expect(entries.everything().entries.map((e) => e.hash)).toEqual(['d', 'b', 'c', 'a']);
    expect(entries.everything().entries.find((e) => e.hash === 'b')?.corrected).toBe(true);
    expect(entries.standing().entries.map((e) => e.hash)).toEqual(['c', 'a']);
    expect(entries.standing().total).toBe(4);
    expect(entries.standing().entries[0].product_name).toBe('Wedding Cake 22/1');
  });

  it('starts again when the server says so', async () => {
    const { http } = server([
      { reset: true, head: 'a', total: 1, names: {}, entries: [entry('a', '20')] },
      { reset: true, head: 'z', total: 1, names: {}, entries: [entry('z', '21')] },
    ]);
    const entries = new Entries(http);
    await entries.refresh();
    await entries.refresh();
    expect(entries.everything().entries.map((e) => e.hash)).toEqual(['z']);
  });
});
