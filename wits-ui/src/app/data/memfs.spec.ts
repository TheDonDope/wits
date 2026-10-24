import { describe, expect, it } from 'vitest';

import { kept } from './engine';
import { FsError, MemFS, clean, constants } from './memfs';

/** Calls a callback-style MemFS method and returns what it answered, typed as the caller expects. */
// eslint-disable-next-line @typescript-eslint/no-unnecessary-type-parameters -- T names what the caller expects back
function call<T>(run: (cb: (err: FsError | null, result?: unknown) => void) => void): { err: FsError | null; result: T } {
  let out: { err: FsError | null; result: T } | undefined;
  run((err, result) => {
    out = { err, result: result as T };
  });
  if (!out) {
    throw new Error('the callback was not called at once');
  }
  return out;
}

const bytes = (s: string) => new TextEncoder().encode(s);
const text = (b: Uint8Array | undefined) => new TextDecoder().decode(b);
const { O_WRONLY, O_RDWR, O_CREAT, O_EXCL, O_TRUNC, O_APPEND } = constants;

function written(fs: MemFS, path: string, s: string, flags = O_WRONLY | O_CREAT | O_TRUNC): void {
  const fd = call<number>((cb) => {
    fs.open(path, flags, 0o600, cb);
  }).result;
  const b = bytes(s);
  call((cb) => {
    fs.write(fd, b, 0, b.length, null, cb);
  });
  call((cb) => {
    fs.close(fd, cb);
  });
}

describe('MemFS', () => {
  it('answers no error as null, which is what Go reads as success', () => {
    const fs = new MemFS();
    expect(call((cb) => {
      fs.mkdir('/wits', 0o700, cb);
    }).err).toBeNull();
  });

  it('writes, appends and reads back', () => {
    const fs = new MemFS();
    fs.put('/wits/.wits/journal.ndjson', bytes('one\n'));
    written(fs, '/wits/.wits/journal.ndjson', 'two\n', O_WRONLY | O_APPEND);
    expect(text(fs.get('/wits/.wits/journal.ndjson'))).toBe('one\ntwo\n');

    const fd = call<number>((cb) => {
      fs.open('/wits/.wits/journal.ndjson', O_RDWR, 0, cb);
    }).result;
    const buf = new Uint8Array(3);
    expect(call<number>((cb) => {
      fs.read(fd, buf, 0, 3, 4, cb);
    }).result).toBe(3);
    expect(text(buf)).toBe('two');
  });

  it('refuses what Node refuses, with codes Go knows', () => {
    const fs = new MemFS();
    expect(call((cb) => {
      fs.open('/nowhere/file', O_WRONLY | O_CREAT, 0o600, cb);
    }).err?.code).toBe('ENOENT');
    fs.put('/a/b', bytes('x'));
    expect(call((cb) => {
      fs.open('/a/b', O_WRONLY | O_CREAT | O_EXCL, 0o600, cb);
    }).err?.code).toBe('EEXIST');
    expect(call((cb) => {
      fs.rmdir('/a', cb);
    }).err?.code).toBe('ENOTEMPTY');
    expect(call((cb) => {
      fs.readdir('/a/b', cb);
    }).err?.code).toBe('ENOTDIR');
  });

  it('renames over a file, the way a write-and-rename relies on', () => {
    const fs = new MemFS();
    fs.put('/wits/.wits/products.yml', bytes('old'));
    written(fs, '/wits/.wits/.products.yml.123', 'new');
    call((cb) => {
      fs.rename('/wits/.wits/.products.yml.123', '/wits/.wits/products.yml', cb);
    });
    expect(text(fs.get('/wits/.wits/products.yml'))).toBe('new');
    expect(fs.get('/wits/.wits/.products.yml.123')).toBeUndefined();
  });

  it('lists a folder and says what is one', () => {
    const fs = new MemFS();
    fs.put('/wits/.wits/config.yml', bytes(''));
    fs.put('/wits/.wits/index/x', bytes(''));
    expect(call<string[]>((cb) => {
      fs.readdir('/wits/.wits', cb);
    }).result).toEqual(['config.yml', 'index']);
    const stat = call<{ isDirectory(): boolean; size: number }>((cb) => {
      fs.stat('/wits/.wits', cb);
    }).result;
    expect(stat.isDirectory()).toBe(true);
  });

  it('truncates a write cut off before its newline', () => {
    const fs = new MemFS();
    fs.put('/j', bytes('whole\ncut'));
    call((cb) => {
      fs.truncate('/j', 6, cb);
    });
    expect(text(fs.get('/j'))).toBe('whole\n');
  });

  it('reports every change, for the engine to keep', () => {
    const fs = new MemFS();
    const changed: string[] = [];
    fs.onChange = (p) => changed.push(p);
    fs.put('/wits/x', bytes(''));
    written(fs, '/wits/x', 'y', O_WRONLY | O_APPEND);
    call((cb) => {
      fs.unlink('/wits/x', cb);
    });
    expect(changed).toEqual(['/wits/x', '/wits/x']);
  });

  it('cleans paths', () => {
    expect(clean('wits//.wits/./a/../journal.ndjson')).toBe('/wits/.wits/journal.ndjson');
    expect(clean('/')).toBe('/');
  });
});

describe('kept', () => {
  it('keeps the repository and nothing else', () => {
    expect(kept('/wits/.wits/journal.ndjson')).toBe(true);
    expect(kept('/wits/.wits/journal.ndjson.lock')).toBe(false);
    expect(kept('/remote/wits.age')).toBe(false);
    expect(kept('/tmp/remote.key')).toBe(false);
  });
});
