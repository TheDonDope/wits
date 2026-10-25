import { describe, expect, it } from 'vitest';

import { PRESETS, blankDevice, heats, needsHeat, perUse, sessionWith } from './devices';
import { Device } from './models';

describe('devices', () => {
  it('says what a session is had with, what heats one, and what needs heating', () => {
    expect(sessionWith({ category: 'vaporizer' })).toBe(true);
    expect(sessionWith({ category: 'glass' })).toBe(true);
    expect(sessionWith({ category: 'grinder' })).toBe(false);
    expect(heats({ category: 'torch' })).toBe(true);
    expect(needsHeat({ category: 'vaporizer', power: 'manual' })).toBe(true);
    expect(needsHeat({ category: 'vaporizer', power: 'electric' })).toBe(false);
    expect(needsHeat({ category: 'glass', power: 'manual' })).toBe(true);
  });

  it('prices a use over the sessions it had, or a heater over those it heated', () => {
    const d = { cents: 20800, sessions: 4, heated: 0, category: 'vaporizer' } as Device;
    expect(perUse(d)).toBe(5200);
    expect(perUse({ ...d, category: 'induction-heater', heated: 2 })).toBe(10400);
    expect(perUse({ ...d, sessions: 0 })).toBe(0);
  });

  it('starts glass and grinders manual', () => {
    expect(blankDevice('glass').power).toBe('manual');
    expect(blankDevice('vaporizer').power).toBe('');
  });

  it('has a manual vaporizer used on demand, and the Frolic’s two modes as used', () => {
    for (const p of PRESETS.filter((x) => x.category === 'vaporizer' && x.power === 'manual')) {
      expect(p.use, p.preset).toBe('on-demand');
    }
    const frolic = PRESETS.find((p) => p.preset === 'Limelight Herb Frolic');
    expect(frolic?.modes).toEqual([
      { name: 'On demand', signature: 'convection', temp: 165 },
      { name: 'Session', signature: 'hybrid', temp: 185, timer: 420 },
    ]);
    expect(PRESETS.find((p) => p.preset === 'Storz & Bickel Mighty+')).toMatchObject({ use: 'session', signature: 'hybrid' });
    expect(new Set(PRESETS.map((p) => p.preset)).size).toBe(PRESETS.length);
  });
});
