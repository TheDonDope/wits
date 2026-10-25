import { Injectable, computed, effect, signal } from '@angular/core';

/** A theme as the picker lists it; its colours and style live in themes.scss. */
export interface ThemeInfo {
  id: string;
  label: string;
  /** What sets it apart, in a few words. */
  note: string;
}

/** Every theme, in the order the picker shows them and `t` steps through them. */
export const THEMES: readonly ThemeInfo[] = [
  { id: 'aurora', label: 'Aurora', note: 'ink and glass, violet to mint' },
  { id: 'nord', label: 'Nord', note: 'arctic, flat, quiet' },
  { id: 'dracula', label: 'Dracula', note: 'purple and pink at night' },
  { id: 'gruvbox', label: 'Gruvbox', note: 'warm retro, monospace' },
  { id: 'solarized', label: 'Solarized', note: 'teal dark, precise' },
  { id: 'tokyo-night', label: 'Tokyo Night', note: 'neon blue, glowing' },
  { id: 'mocha', label: 'Mocha', note: 'pastel, rounded' },
  { id: 'rose-pine', label: 'Rosé Pine', note: 'muted rose, serif figures' },
  { id: 'phosphor', label: 'Phosphor', note: 'green CRT, scanlines' },
  { id: 'synthwave', label: 'Synthwave', note: 'magenta sunset, cyan grid' },
  { id: 'void', label: 'Void', note: 'pure black, hairlines' },
  { id: 'forest', label: 'Forest', note: 'brass and moss, serif figures' },
  { id: 'ember', label: 'Ember', note: 'charcoal and flame' },
  { id: 'abyss', label: 'Abyss', note: 'deep water, heavy glass' },
  { id: 'bauhaus-dark', label: 'Bauhaus Dark', note: 'primaries on black, red-cast' },
  { id: 'blueprint', label: 'Blueprint', note: 'cyanotype, three-weight grid' },
  { id: 'deco', label: 'Deco', note: 'onyx and gold, sunburst' },
  { id: 'hazard', label: 'Hazard', note: 'warning tape, workshop' },
  { id: 'amber', label: 'Amber', note: 'orange CRT, scanlines' },
  { id: 'noir', label: 'Noir', note: 'light through the blinds' },
  { id: 'starfield', label: 'Starfield', note: 'deep space, nebulae' },
  { id: 'topo', label: 'Topo', note: 'survey map, contour lines' },
  { id: 'contrast', label: 'Contrast', note: 'black, white, loud' },
  { id: 'paper', label: 'Paper', note: 'cream and ink, all serif' },
  { id: 'solarized-light', label: 'Solarized Light', note: 'the same, by day' },
  { id: 'latte', label: 'Latte', note: 'pastel, light, rounded' },
  { id: 'sakura', label: 'Sakura', note: 'blossom pink, soft' },
  { id: 'bauhaus', label: 'Bauhaus', note: 'primaries, hard edges' },
  { id: 'daylight', label: 'Daylight', note: 'clean white, plain' },
  { id: 'dune', label: 'Dune', note: 'sand and terracotta' },
  { id: 'drafting', label: 'Drafting', note: 'vellum and red pencil' },
  { id: 'greenbar', label: 'Greenbar', note: 'tractor-feed ledger paper' },
  { id: 'risograph', label: 'Risograph', note: 'pink and blue, misregistered' },
  { id: 'swiss', label: 'Swiss', note: 'twelve columns, one red' },
  { id: 'chicago', label: 'Chicago', note: 'grey bevels, teal desktop' },
  { id: 'sumi', label: 'Sumi', note: 'ink wash and a seal' },
  { id: 'kraft', label: 'Kraft', note: 'brown paper, typewriter' },
  { id: 'dot-matrix', label: 'Dot Matrix', note: 'four greens, pixel grid' },
  { id: 'eink', label: 'E-Ink', note: 'grey on grey, no glow' },
  { id: 'glacier', label: 'Glacier', note: 'ice and frosted glass' },
  { id: 'memphis', label: 'Memphis', note: '1982 confetti, hard shadows' },
];

const KEY = 'wits.theme';

/**
 * The theme to start in: ?theme=… if the address names one (a link to show a
 * theme, or to photograph it), else the stored choice, else the first.
 */
function stored(): string {
  try {
    const id = new URLSearchParams(location.search).get('theme') ?? localStorage.getItem(KEY) ?? '';
    return THEMES.some((t) => t.id === id) ? id : THEMES[0].id;
  } catch {
    return THEMES[0].id;
  }
}

/**
 * The theme the page wears. It is this browser's choice, kept in local storage
 * — a look, not a fact about the ledger, so it travels with no repository.
 */
@Injectable({ providedIn: 'root' })
export class Theme {
  readonly current = signal(stored());
  readonly info = computed(() => THEMES.find((t) => t.id === this.current()) ?? THEMES[0]);

  constructor() {
    effect(() => {
      apply(this.current());
    });
  }

  set(id: string): void {
    this.current.set(id);
    try {
      localStorage.setItem(KEY, id);
    } catch {
      // Private windows may refuse storage; the theme still holds until reload.
    }
  }

  /** The next theme along, or the one before. */
  step(by: number): void {
    const i = THEMES.findIndex((t) => t.id === this.current());
    this.set(THEMES[(i + by + THEMES.length) % THEMES.length].id);
  }
}

/** Puts a theme on the page, and tells the browser's chrome its colour. */
export function apply(id: string = stored()): void {
  const root = document.documentElement;
  root.dataset['theme'] = id;
  const ink = getComputedStyle(root).getPropertyValue('--w-ink').trim();
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', ink || '#0b0e14');
}
