import { Device, DeviceCategory, DeviceFields } from './models';

/** The categories, in the order the page shows them, with what each is called. */
export const CATEGORIES: { id: DeviceCategory; label: string; plural: string; icon: string }[] = [
  { id: 'vaporizer', label: 'Vaporizer', plural: 'Vaporizers', icon: 'air' },
  { id: 'induction-heater', label: 'Induction heater', plural: 'Induction heaters', icon: 'bolt' },
  { id: 'torch', label: 'Torch', plural: 'Torches', icon: 'local_fire_department' },
  { id: 'grinder', label: 'Grinder', plural: 'Grinders', icon: 'settings' },
  { id: 'glass', label: 'Glass', plural: 'Glass', icon: 'science' },
];

export const SIGNATURES = [
  { id: 'convection', label: 'Convection' },
  { id: 'conduction', label: 'Conduction' },
  { id: 'hybrid', label: 'Hybrid' },
] as const;

export const USES = [
  { id: 'on-demand', label: 'On demand' },
  { id: 'session', label: 'Session' },
  { id: 'both', label: 'Both' },
] as const;

/** Whether a session is had with the device: a vaporizer, or glass. */
export function sessionWith(d: Pick<Device, 'category'>): boolean {
  return d.category === 'vaporizer' || d.category === 'glass';
}

/** Whether the device heats another: an induction heater or a torch. */
export function heats(d: Pick<Device, 'category'>): boolean {
  return d.category === 'induction-heater' || d.category === 'torch';
}

/** Whether a session with the device is heated by another: a manual vaporizer, or glass. */
export function needsHeat(d: Pick<Device, 'category' | 'power'>): boolean {
  return (d.category === 'vaporizer' && d.power === 'manual') || d.category === 'glass';
}

/** What a session with a device cost, in cents: its price over the sessions it was used for. */
export function perUse(d: Device): number {
  const uses = heats(d) ? d.heated : d.sessions;
  return d.cents && uses ? d.cents / uses : 0;
}

/** An empty device of a category: a manual vaporizer is on demand, heated once and its AVB tossed. */
export function blankDevice(category: DeviceCategory = 'vaporizer'): DeviceFields {
  return { name: '', category, modes: [], power: category === 'grinder' || category === 'glass' ? 'manual' : '' };
}

/**
 * Devices known, to start from: what each is, as far as could be found out —
 * a field left empty is one that could not be, and is to be filled in. What
 * one cost and where it was bought are always yours to fill.
 */
export const PRESETS: (DeviceFields & { preset: string })[] = [
  {
    preset: 'Storz & Bickel Venty', name: 'Storz & Bickel Venty', category: 'vaporizer', maker: 'Storz & Bickel', model: 'Venty',
    country: 'Germany', power: 'electric', signature: 'hybrid', use: 'both', min_temp: 40, max_temp: 210, default_temp: 180,
    modes: [{ name: 'Base' }, { name: 'Boost' }, { name: 'Superboost' }, { name: 'Permanent boost' }],
  },
  {
    preset: 'Storz & Bickel Mighty+', name: 'Storz & Bickel Mighty+', category: 'vaporizer', maker: 'Storz & Bickel',
    model: 'Mighty+', country: 'Germany', power: 'electric', signature: 'hybrid', use: 'session', min_temp: 40, max_temp: 210,
    default_temp: 180, modes: [{ name: 'Base' }, { name: 'Super Boost' }],
  },
  {
    preset: 'Limelight Herb Frolic', name: 'Limelight Herb Frolic', category: 'vaporizer', maker: 'Limelight Herb', model: 'Frolic',
    power: 'electric', signature: 'convection', use: 'both',
    modes: [{ name: 'On demand', signature: 'convection', temp: 165 }, { name: 'Session', signature: 'hybrid', temp: 185, timer: 420 }],
  },
  {
    preset: 'Vestratto Tornado', name: 'Vestratto Tornado', category: 'vaporizer', maker: 'Vestratto', model: 'Tornado',
    power: 'manual', signature: 'convection', use: 'on-demand', modes: [],
    notes: 'Heated by induction: a copper coil inside heats the air drawn through.',
  },
  { preset: 'Vestratto Anvil', name: 'Vestratto Anvil', category: 'vaporizer', maker: 'Vestratto', model: 'Anvil', power: 'manual', use: 'on-demand', modes: [] },
  { preset: 'Mad Heaters Tempest', name: 'Mad Heaters Tempest', category: 'vaporizer', maker: 'Mad Heaters', model: 'Tempest', power: 'manual', use: 'on-demand', modes: [] },
  {
    preset: 'The Simrell Collection — The Simrell', name: 'The Simrell', category: 'vaporizer', maker: 'The Simrell Collection',
    model: 'The Simrell', country: 'USA', power: 'manual', signature: 'hybrid', use: 'on-demand', modes: [],
  },
  {
    preset: 'The Simrell Collection — Hydro', name: 'Simrell Hydro', category: 'vaporizer', maker: 'The Simrell Collection',
    model: 'Hydro', country: 'USA', power: 'manual', use: 'on-demand', modes: [],
  },
  {
    preset: 'YLL 3.0', name: 'YLL 3.0', category: 'induction-heater', maker: 'YLL', model: '3.0', power: 'electric', modes: [],
    notes: 'Adjustable wattage, a 60 s timeout. Add the settings you use as modes, and a session asks which.',
  },
  { preset: 'Blazer PT-4000', name: 'Blazer PT-4000', category: 'torch', maker: 'Blazer', model: 'PT-4000', modes: [] },
  { preset: 'Blazer PB-207', name: 'Blazer PB-207', category: 'torch', maker: 'Blazer', model: 'PB-207', modes: [] },
  {
    preset: 'Brilliant Cut Grinder', name: 'Brilliant Cut Grinder', category: 'grinder', maker: 'Grinders For Life',
    model: 'Brilliant Cut', country: 'Canada', power: 'manual', modes: [],
  },
  { preset: 'Old Mate Designs Aroma 3', name: 'Aroma 3', category: 'grinder', maker: 'Old Mate Designs', model: 'Aroma 3', power: 'manual', modes: [] },
  { preset: 'Glass pipe, dry', name: 'Glass pipe', category: 'glass', power: 'manual', water: false, modes: [] },
  { preset: 'Rig with water', name: 'Rig', category: 'glass', power: 'manual', water: true, modes: [] },
];
