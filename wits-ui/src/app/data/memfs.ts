/**
 * The file system wits-wasm runs on in a browser: the handful of Node's `fs`
 * calls Go's js/wasm port makes (syscall/fs_js.go), held in memory.
 *
 * Go waits on each call's callback, which may run before the call returns —
 * its channel is buffered — so every call here completes at once. "No error"
 * must be `null`: Go reads anything else as an error, and an error whose
 * `code` it has no errno for panics the program, so only codes it knows are
 * used. What changes is reported through `onChange`, for the engine to keep
 * in IndexedDB.
 */

type Callback = (err: FsError | null, result?: unknown) => void;

/** An error as Node reports one: what Go maps to an errno. */
export class FsError extends Error {
  readonly code: 'ENOENT' | 'EEXIST' | 'ENOTDIR' | 'EISDIR' | 'ENOTEMPTY' | 'EBADF' | 'EINVAL' | 'ENOSYS';

  constructor(code: FsError['code'], path: string) {
    super(`${code}: ${path}`);
    this.code = code;
  }
}

/** Node's open flags, on Linux: what Go translates its own into. */
export const constants = {
  O_RDONLY: 0,
  O_WRONLY: 1,
  O_RDWR: 2,
  O_CREAT: 0o100,
  O_EXCL: 0o200,
  O_TRUNC: 0o1000,
  O_APPEND: 0o2000,
  O_DIRECTORY: 0o200000,
};

const S_IFDIR = 0o040000;
const S_IFREG = 0o100000;

interface File {
  data: Uint8Array;
  size: number;
  mtime: number;
}

interface Handle {
  path: string;
  flags: number;
  pos: number;
}

/** A file's or a folder's attributes, as Node's Stats carries them. */
export interface Stat {
  dev: number;
  ino: number;
  mode: number;
  nlink: number;
  uid: number;
  gid: number;
  rdev: number;
  size: number;
  blksize: number;
  blocks: number;
  atimeMs: number;
  mtimeMs: number;
  ctimeMs: number;
  isDirectory(): boolean;
}

/** Makes a path absolute and plain: no `.`, no `..`, no doubled slashes. */
export function clean(path: string): string {
  const parts: string[] = [];
  for (const part of path.split('/')) {
    if (part === '..') {
      parts.pop();
    } else if (part !== '' && part !== '.') {
      parts.push(part);
    }
  }
  return '/' + parts.join('/');
}

function parent(path: string): string {
  return clean(path + '/..');
}

export class MemFS {
  readonly constants = constants;
  /** Called with each path whose contents changed or went. */
  onChange: (path: string) => void = () => undefined;

  private readonly files = new Map<string, File>();
  private readonly dirs = new Set<string>(['/']);
  private readonly handles = new Map<number, Handle>();
  private nextFd = 100;

  /** Puts a file in place, its folders made, as a load from storage does. */
  put(path: string, data: Uint8Array): void {
    path = clean(path);
    this.mkdirs(parent(path));
    this.files.set(path, { data: data.slice(), size: data.length, mtime: Date.now() });
  }

  /** A file's contents, or undefined. */
  get(path: string): Uint8Array | undefined {
    const f = this.files.get(clean(path));
    return f?.data.slice(0, f.size);
  }

  /** Every file's path. */
  paths(): string[] {
    return [...this.files.keys()];
  }

  open(path: string, flags: number, _mode: number, cb: Callback): void {
    path = clean(path);
    if (this.dirs.has(path)) {
      cb(null, this.handle(path, flags));
      return;
    }
    const f = this.files.get(path);
    if (f && flags & constants.O_CREAT && flags & constants.O_EXCL) {
      cb(new FsError('EEXIST', path));
    } else if (!f && !(flags & constants.O_CREAT)) {
      cb(new FsError('ENOENT', path));
    } else if (!this.dirs.has(parent(path))) {
      cb(new FsError('ENOENT', path));
    } else {
      if (!f || flags & constants.O_TRUNC) {
        this.files.set(path, { data: new Uint8Array(0), size: 0, mtime: Date.now() });
        this.onChange(path);
      }
      cb(null, this.handle(path, flags));
    }
  }

  close(fd: number, cb: Callback): void {
    cb(this.handles.delete(fd) ? null : new FsError('EBADF', String(fd)));
  }

  read(fd: number, buf: Uint8Array, offset: number, length: number, position: number | null, cb: Callback): void {
    const h = this.handles.get(fd);
    const f = h && this.files.get(h.path);
    if (!h || !f) {
      cb(new FsError(h ? 'EISDIR' : 'EBADF', String(fd)));
      return;
    }
    const at = position ?? h.pos;
    const n = Math.max(0, Math.min(length, f.size - at));
    buf.set(f.data.subarray(at, at + n), offset);
    if (position === null) {
      h.pos += n;
    }
    cb(null, n);
  }

  write(fd: number, buf: Uint8Array, offset: number, length: number, position: number | null, cb: Callback): void {
    const h = this.handles.get(fd);
    const f = h && this.files.get(h.path);
    if (!h || !f) {
      cb(new FsError(h ? 'EISDIR' : 'EBADF', String(fd)));
      return;
    }
    const at = h.flags & constants.O_APPEND ? f.size : (position ?? h.pos);
    this.fit(f, at + length);
    f.data.set(buf.subarray(offset, offset + length), at);
    f.size = Math.max(f.size, at + length);
    f.mtime = Date.now();
    if (position === null) {
      h.pos = at + length;
    }
    this.onChange(h.path);
    cb(null, length);
  }

  fstat(fd: number, cb: Callback): void {
    const h = this.handles.get(fd);
    if (h) {
      this.stat(h.path, cb);
    } else {
      cb(new FsError('EBADF', String(fd)));
    }
  }

  stat(path: string, cb: Callback): void {
    path = clean(path);
    const f = this.files.get(path);
    if (!f && !this.dirs.has(path)) {
      cb(new FsError('ENOENT', path));
      return;
    }
    const dir = !f;
    const mtime = f?.mtime ?? 0;
    const stat: Stat = {
      dev: 1, ino: 0, mode: dir ? S_IFDIR | 0o700 : S_IFREG | 0o600, nlink: 1, uid: 0, gid: 0, rdev: 0,
      size: f?.size ?? 0, blksize: 4096, blocks: Math.ceil((f?.size ?? 0) / 512),
      atimeMs: mtime, mtimeMs: mtime, ctimeMs: mtime,
      isDirectory: () => dir,
    };
    cb(null, stat);
  }

  lstat(path: string, cb: Callback): void {
    this.stat(path, cb);
  }

  readdir(path: string, cb: Callback): void {
    path = clean(path);
    if (!this.dirs.has(path)) {
      cb(new FsError(this.files.has(path) ? 'ENOTDIR' : 'ENOENT', path));
      return;
    }
    cb(null, this.children(path));
  }

  mkdir(path: string, _perm: number, cb: Callback): void {
    path = clean(path);
    if (this.dirs.has(path) || this.files.has(path)) {
      cb(new FsError('EEXIST', path));
    } else if (!this.dirs.has(parent(path))) {
      cb(new FsError('ENOENT', path));
    } else {
      this.dirs.add(path);
      cb(null);
    }
  }

  rmdir(path: string, cb: Callback): void {
    path = clean(path);
    if (!this.dirs.has(path)) {
      cb(new FsError(this.files.has(path) ? 'ENOTDIR' : 'ENOENT', path));
    } else if (this.children(path).length > 0) {
      cb(new FsError('ENOTEMPTY', path));
    } else {
      this.dirs.delete(path);
      cb(null);
    }
  }

  unlink(path: string, cb: Callback): void {
    path = clean(path);
    if (this.files.delete(path)) {
      this.onChange(path);
      cb(null);
    } else {
      cb(new FsError(this.dirs.has(path) ? 'EISDIR' : 'ENOENT', path));
    }
  }

  /** Renames a file over another, the way a write-and-rename relies on. */
  rename(from: string, to: string, cb: Callback): void {
    from = clean(from);
    to = clean(to);
    const f = this.files.get(from);
    if (!f) {
      cb(new FsError(this.dirs.has(from) ? 'EINVAL' : 'ENOENT', from));
      return;
    }
    if (!this.dirs.has(parent(to))) {
      cb(new FsError('ENOENT', to));
      return;
    }
    this.files.delete(from);
    this.files.set(to, f);
    for (const h of this.handles.values()) {
      if (h.path === from) {
        h.path = to;
      }
    }
    this.onChange(from);
    this.onChange(to);
    cb(null);
  }

  ftruncate(fd: number, length: number, cb: Callback): void {
    const h = this.handles.get(fd);
    if (h) {
      this.truncate(h.path, length, cb);
    } else {
      cb(new FsError('EBADF', String(fd)));
    }
  }

  truncate(path: string, length: number, cb: Callback): void {
    path = clean(path);
    const f = this.files.get(path);
    if (!f) {
      cb(new FsError('ENOENT', path));
      return;
    }
    this.fit(f, length);
    f.data.fill(0, length);
    f.size = length;
    this.onChange(path);
    cb(null);
  }

  fsync(fd: number, cb: Callback): void {
    cb(this.handles.has(fd) ? null : new FsError('EBADF', String(fd)));
  }

  // Ownership, permissions and times mean nothing here; they succeed.
  chmod(_path: string, _mode: number, cb: Callback): void {
    cb(null);
  }
  fchmod(_fd: number, _mode: number, cb: Callback): void {
    cb(null);
  }
  chown(_path: string, _uid: number, _gid: number, cb: Callback): void {
    cb(null);
  }
  fchown(_fd: number, _uid: number, _gid: number, cb: Callback): void {
    cb(null);
  }
  lchown(_path: string, _uid: number, _gid: number, cb: Callback): void {
    cb(null);
  }
  utimes(_path: string, _atime: number, _mtime: number, cb: Callback): void {
    cb(null);
  }

  // Links are never made by wits.
  link(from: string, _to: string, cb: Callback): void {
    cb(new FsError('ENOSYS', from));
  }
  symlink(from: string, _to: string, cb: Callback): void {
    cb(new FsError('ENOSYS', from));
  }
  readlink(path: string, cb: Callback): void {
    cb(new FsError('EINVAL', path));
  }

  private handle(path: string, flags: number): number {
    const fd = this.nextFd++;
    this.handles.set(fd, { path, flags, pos: 0 });
    return fd;
  }

  private children(dir: string): string[] {
    const prefix = dir === '/' ? '/' : dir + '/';
    const names = new Set<string>();
    for (const p of [...this.files.keys(), ...this.dirs]) {
      if (p !== dir && p.startsWith(prefix) && !p.slice(prefix.length).includes('/')) {
        names.add(p.slice(prefix.length));
      }
    }
    return [...names].sort();
  }

  private mkdirs(path: string): void {
    for (let p = clean(path); !this.dirs.has(p); p = parent(p)) {
      this.dirs.add(p);
    }
  }

  /** Grows a file's buffer to hold at least n bytes, doubling. */
  private fit(f: File, n: number): void {
    if (n <= f.data.length) {
      return;
    }
    const grown = new Uint8Array(Math.max(n, f.data.length * 2, 1024));
    grown.set(f.data.subarray(0, f.size));
    f.data = grown;
  }
}
