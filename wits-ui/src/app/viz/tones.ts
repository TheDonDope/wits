/**
 * Colours for things told apart by colour alone — a trip's jars, a day's
 * strains — taken in turn: the theme's own, so every theme speaks them.
 */
export const TONES = ['var(--w-stash)', 'var(--w-storage)', 'var(--w-sesh)', 'var(--w-avb)', 'var(--w-accent)', 'var(--w-primary)'];

/** The tone for the i-th thing. */
export function tone(i: number): string {
  return TONES[i % TONES.length];
}
