import { DestroyRef, Injectable, computed, signal } from '@angular/core';

/** One key and what it does: the keys that press it, the label the help shows. */
export interface Binding {
  /** As KeyboardEvent.key gives them; ' ' is space. */
  keys: string[];
  /** How the help writes the keys, when not just the first of them. */
  label?: string;
  help: string;
  run: () => void;
  /** Kept off the footer, only in the full help. */
  quiet?: boolean;
}

interface Layer {
  id: number;
  name: string;
  bindings: () => Binding[];
}

let next = 0;

/**
 * Every key the interface honours, declared once — the terminal's rule, so the
 * help can never advertise a key nothing answers. The shell registers the
 * global ones, each page its own while it is open; the newest layer wins a key
 * both claim. A binding list is a function, so the help follows the state —
 * "pause" while playing, "play" when not.
 */
@Injectable({ providedIn: 'root' })
export class Keys {
  private readonly layers = signal<Layer[]>([]);
  readonly showAll = signal(false);
  /**
   * Whether keys have been used here: the help line along the foot shows from
   * the first, and stays out of the way of whoever only clicks. Kept in this
   * browser.
   */
  readonly used = signal(remembered());

  /** The layers as the help draws them, page first. */
  readonly sections = computed(() =>
    [...this.layers()].reverse().map((l) => ({ name: l.name, bindings: l.bindings() })),
  );

  /** What the footer shows: every layer's loud keys, page first, no key twice. */
  readonly footer = computed(() => {
    const seen = new Set<string>();
    const out: Binding[] = [];
    for (const s of this.sections()) {
      for (const b of s.bindings) {
        const fresh = b.keys.filter((k) => !seen.has(k));
        b.keys.forEach((k) => seen.add(k));
        if (fresh.length && !b.quiet) {
          out.push(b);
        }
      }
    }
    return out;
  });

  /** Adds a layer of keys for as long as its owner lives. */
  register(name: string, bindings: () => Binding[], destroy: DestroyRef): void {
    const layer: Layer = { id: ++next, name, bindings };
    this.layers.update((ls) => [...ls, layer]);
    destroy.onDestroy(() => {
      this.layers.update((ls) => ls.filter((l) => l.id !== layer.id));
    });
  }

  /** Notes that keys are used here, so the help line shows from now on. */
  private remember(): void {
    if (this.used()) {
      return;
    }
    this.used.set(true);
    try {
      localStorage.setItem(USED, '1');
    } catch {
      // Storage refused: the help line shows for this visit only.
    }
  }

  /** Runs the binding for a key, if one answers it and nothing else should. */
  handle(ev: KeyboardEvent): void {
    if (ev.ctrlKey || ev.metaKey || ev.altKey || typing(ev.target)) {
      return;
    }
    if (document.querySelector('.cdk-overlay-container .mat-mdc-dialog-container')) {
      return; // a dialog has the keyboard
    }
    if (ev.key === 'Escape' && this.showAll()) {
      this.showAll.set(false);
      ev.preventDefault();
      return;
    }
    for (const layer of [...this.layers()].reverse()) {
      const hit = layer.bindings().find((b) => b.keys.includes(ev.key));
      if (hit) {
        ev.preventDefault();
        hit.run();
        this.remember();
        return;
      }
    }
  }
}

const USED = 'wits.keys';

function remembered(): boolean {
  try {
    return localStorage.getItem(USED) === '1';
  } catch {
    return false;
  }
}

/** Whether a key went to something being typed into. */
function typing(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  return !!el?.closest('input, textarea, select, mat-select, [contenteditable="true"]');
}
