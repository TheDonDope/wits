import { describe, expect, it } from 'vitest';

import { withDuration } from './film';

describe('withDuration', () => {
  // EBML header, then a Segment of unknown size holding an Info element of
  // seven bytes: TimecodeScale = 1000000 (ms), as a browser writes it.
  const header = [0x1a, 0x45, 0xdf, 0xa3, 0x80];
  const segment = [0x18, 0x53, 0x80, 0x67, 0x01, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff];
  const info = [0x15, 0x49, 0xa9, 0x66, 0x87, 0x2a, 0xd7, 0xb1, 0x83, 0x0f, 0x42, 0x40];
  const webm = new Uint8Array([...header, ...segment, ...info, 0x16, 0x54, 0xae, 0x6b]);

  it('writes the duration into Info, and grows Info by it', () => {
    const out = withDuration(webm, 30000);
    const at = header.length + segment.length;
    expect(out.length).toBe(webm.length + 11);
    expect(out[at + 4]).toBe(0x87 + 11);
    expect([...out.subarray(at + 5, at + 8)]).toEqual([0x44, 0x89, 0x88]);
    expect(new DataView(out.buffer, at + 8, 8).getFloat64(0)).toBe(30000);
    expect([...out.subarray(out.length - 4)]).toEqual([0x16, 0x54, 0xae, 0x6b]);
  });

  it('leaves a file it does not understand, or that has one, as it was', () => {
    const odd = new Uint8Array([1, 2, 3]);
    expect(withDuration(odd, 1)).toBe(odd);
    const once = withDuration(webm, 30000);
    expect(withDuration(once, 30000)).toBe(once);
  });
});
