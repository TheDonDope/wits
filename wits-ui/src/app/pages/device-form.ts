import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';

import { CATEGORIES, PRESETS, SIGNATURES, USES, blankDevice } from '../data/devices';
import { Device, DeviceCategory, DeviceFields, DeviceMode } from '../data/models';
import { Wits } from '../data/wits';
import { Choice, ChoiceInput } from '../inputs/choice';
import { TempDial } from '../inputs/temp-dial';
import { TempRange } from '../inputs/temp-range';
import { isoDay } from '../inputs/when';

/**
 * Adding a device, or changing one: from a preset of what is known about it,
 * or from nothing. Only what the category has is asked: a vaporizer's power,
 * heating, use, temperatures and modes; an induction heater's settings; glass
 * with water or dry. A manual vaporizer is used on demand — heated once,
 * inhaled, its AVB tossed — so its use is not asked.
 */
@Component({
  selector: 'w-device-form',
  imports: [ChoiceInput, MatButtonModule, MatDialogModule, MatIconModule, TempDial, TempRange],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h2 mat-dialog-title>{{ editing ? 'Change ' + editing.name : 'Add a device' }}</h2>
    <mat-dialog-content>
      <div class="form">
        @if (!editing) {
          <w-choice label="start from" [tiles]="true" [fold]="7" [options]="presetChoices" [value]="preset()" (valueChange)="fromPreset($event)" />
          <p class="dim note">What is known of it, filled in; what could not be found stays empty.</p>
        }
        <label class="w-field name">
          <span class="eyebrow">name</span>
          <input [value]="d().name" (input)="set('name', text($event))" placeholder="Storz & Bickel Venty" required />
        </label>
        <w-choice label="what it is" [tiles]="true" [options]="categoryChoices" [value]="d().category" (valueChange)="pickCategory($event)" />
        <div class="three">
          <label class="w-field"><span class="eyebrow">manufacturer</span>
            <span class="w-box"><input [value]="d().maker ?? ''" (input)="set('maker', text($event))" /></span></label>
          <label class="w-field"><span class="eyebrow">model</span>
            <span class="w-box"><input [value]="d().model ?? ''" (input)="set('model', text($event))" /></span></label>
          <label class="w-field"><span class="eyebrow">the maker's country</span>
            <span class="w-box"><input [value]="d().country ?? ''" (input)="set('country', text($event))" /></span></label>
        </div>

        @if (d().category === 'vaporizer' || d().category === 'grinder') {
          <w-choice label="power" [options]="powerChoices" [value]="d().power ?? ''" (valueChange)="pickPower($event)" />
        }
        @if (d().category === 'vaporizer') {
          <div class="pair">
            <w-choice label="heats by" [options]="signatureChoices" [value]="d().signature ?? ''" (valueChange)="set('signature', asSignature($event))" />
            @if (d().power === 'manual') {
              <div class="w-field"><span class="eyebrow">used</span>
                <p class="dim manual">On demand: heated once, inhaled, what was vaped tossed.</p></div>
            } @else {
              <w-choice label="used" [options]="useChoices" [value]="d().use ?? ''" (valueChange)="set('use', asUse($event))" />
            }
          </div>
        }
        @if (d().category === 'vaporizer' || d().category === 'glass') {
          <div class="w-field">
            <span class="eyebrow">a load holds</span>
            <div class="row">
              @for (g of loads; track g) {
                <button type="button" class="chip" [class.on]="d().load === g" (click)="set('load', g)">{{ g }} g</button>
              }
              <label class="w-box small"><input type="number" min="0" step="0.01" placeholder="—" aria-label="A load holds, in grams"
                [value]="d().load ?? ''" (input)="set('load', figure($event))" /><span class="unit">g</span></label>
            </div>
            <p class="dim note">As you usually fill it. Sizes a session you forgot to record.</p>
          </div>
        }
        @if (d().category === 'glass') {
          <w-choice label="water" [options]="waterChoices" [value]="d().water ? 'water' : 'dry'" (valueChange)="set('water', $event === 'water')" />
        }
        @if (d().category === 'vaporizer' && d().power === 'electric') {
          <div class="heat">
            <w-temp-range [from]="d().min_temp" [to]="d().max_temp" (fromChange)="set('min_temp', $event)" (toChange)="set('max_temp', $event)" />
            @if (d().default_temp !== undefined) {
              <div class="usually">
                <w-temp-dial [min]="d().min_temp ?? 40" [max]="d().max_temp ?? 230" [marks]="marks()" [value]="d().default_temp ?? 185"
                  (valueChange)="set('default_temp', $event)" />
                <button mat-button type="button" (click)="set('default_temp', undefined)">Not said how it is usually set</button>
              </div>
            } @else {
              <button mat-stroked-button type="button" class="say" (click)="set('default_temp', usual())">
                <mat-icon>thermostat</mat-icon>Say how it is usually set</button>
            }
          </div>
          <div class="w-field">
            <span class="eyebrow">turns itself off after</span>
            <div class="row">
              @for (m of offs; track m) {
                <button type="button" class="chip" [class.on]="autoOff() === m" (click)="setAutoOff(m)">{{ m }} min</button>
              }
              <label class="w-box small"><input type="number" min="0" step="0.5" placeholder="—" aria-label="Turns itself off after, minutes"
                [value]="autoOff() ?? ''" (input)="setAutoOff(figure($event) ?? null)" /><span class="unit">min</span></label>
            </div>
            <p class="dim note">Unused this long, it ends a session itself.</p>
          </div>
        }

        @if (d().category === 'vaporizer' || d().category === 'induction-heater') {
          <div class="modes">
            <span class="eyebrow">{{ d().category === 'vaporizer' ? 'modes' : 'settings' }}</span>
            <p class="dim note">{{ d().category === 'vaporizer'
              ? 'What it is run in — a boost, a session mode — each heating its own way if it does, at a temperature it is usually run at, and with a session timer if it runs one. A session with it asks which, and starts from those.'
              : 'What it is set to — a wattage, a time. A session it heats asks which.' }}</p>
            @for (m of d().modes; track $index; let i = $index) {
              <div class="mode">
                <label class="w-box mname"><input [value]="m.name" (input)="editMode(i, { name: text($event) })"
                  [placeholder]="d().category === 'vaporizer' ? 'Boost' : '45 W'" [attr.aria-label]="d().category === 'vaporizer' ? 'Mode' : 'Setting'" /></label>
                @if (d().category === 'vaporizer') {
                  <label class="w-box num"><input type="number" placeholder="—" aria-label="Usually, °C" [value]="m.temp ?? ''"
                    (input)="editMode(i, { temp: figure($event) })" /><span class="unit">°C</span></label>
                  <label class="w-box num"><input type="number" min="0" placeholder="—" aria-label="Timer, seconds" [value]="m.timer ?? ''"
                    (input)="editMode(i, { timer: figure($event) })" /><span class="unit">s</span></label>
                }
                <button mat-icon-button type="button" (click)="removeMode(i)" aria-label="Remove"><mat-icon>close</mat-icon></button>
                @if (d().category === 'vaporizer') {
                  <w-choice class="msig" [options]="modeSignatures" [value]="m.signature ?? ''" (valueChange)="editMode(i, { signature: asSignature($event) })" />
                }
              </div>
            }
            <button mat-stroked-button type="button" class="add" (click)="addMode()"><mat-icon>add</mat-icon>{{ d().category === 'vaporizer' ? 'Add a mode' : 'Add a setting' }}</button>
          </div>
        }

        <div class="bought">
          <span class="eyebrow">bought</span>
          <div class="four">
            <label class="w-field"><span class="eyebrow">price</span>
              <span class="w-box"><input type="number" min="0" step="0.01" placeholder="—" [value]="euros() ?? ''" (input)="euros.set(figure($event) ?? null)" />
                <span class="unit">€</span></span></label>
            <label class="w-field"><span class="eyebrow">at</span>
              <span class="w-box"><input [value]="d().shop ?? ''" (input)="set('shop', text($event))" placeholder="the shop" /></span></label>
            <label class="w-field"><span class="eyebrow">on</span>
              <span class="w-box"><input type="date" [max]="today" [value]="d().bought_on ?? ''" (change)="set('bought_on', text($event))" /></span></label>
            <label class="w-field"><span class="eyebrow">link</span>
              <span class="w-box"><input type="url" [value]="d().link ?? ''" (input)="set('link', text($event))" placeholder="https://" /></span></label>
          </div>
        </div>
        <label class="w-field">
          <span class="eyebrow">notes</span>
          <span class="w-box"><textarea rows="3" [value]="d().notes ?? ''" (input)="set('notes', text($event))"></textarea></span>
        </label>
        @if (editing) {
          <w-choice label="in use" [options]="retiredChoices" [value]="d().retired ? 'retired' : 'used'" (valueChange)="set('retired', $event === 'retired')" />
          @if (d().retired) {
            <p class="dim note">Kept, since its sessions name it, but no longer offered.</p>
          }
        }
      </div>
      @if (error()) {
        <p class="bad">{{ error() }}</p>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Cancel</button>
      <button mat-flat-button [disabled]="busy() || !d().name.trim()" (click)="save()">{{ editing ? 'Save' : 'Add' }}</button>
    </mat-dialog-actions>
  `,
  styles: `
    @use 'breakpoints' as bp;
    .form { display: grid; gap: 18px; padding-top: 6px; width: min(640px, calc(94vw - 52px)); }
    .eyebrow { font-size: 11px; }
    .dim { color: var(--w-dim); }
    .note { margin: -10px 0 0; font-size: 12px; }
    .w-field .note { margin: 4px 0 0; }
    .name input { width: 100%; border: 0; border-bottom: 1px solid var(--w-glass-edge); background: none; color: inherit; outline: 0;
      font: 600 22px var(--w-head); padding: 4px 0 8px; }
    .name input:focus { border-bottom-color: var(--w-primary); }
    .name input::placeholder { color: color-mix(in srgb, var(--w-fg) 22%, transparent); font-weight: 400; }
    .three { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
    .pair { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
    .manual { margin: 4px 0 0; font-size: 13px; }
    .row { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
    .chip { padding: 5px 12px; border-radius: 999px; border: 1px solid var(--w-glass-edge); background: none; color: inherit;
      font: 500 12px var(--w-num); cursor: pointer; }
    .chip:hover { border-color: var(--w-primary); }
    .chip.on { border-color: var(--w-primary); background: color-mix(in srgb, var(--w-primary) 18%, transparent); }
    .small { width: 9ch; padding: 3px 8px; }
    .small input[type='number'] { font-size: 14px; }
    .heat { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 18px 24px; align-items: center; padding: 14px;
      border-radius: 16px; background: color-mix(in srgb, var(--w-fg) 3%, transparent); }
    .usually { display: grid; justify-items: center; gap: 2px; }
    .say { justify-self: center; }
    .modes { display: grid; gap: 10px; }
    .modes .note { margin: -6px 0 0; }
    .mode { display: grid; grid-template-columns: minmax(0, 1fr) 11ch 10ch auto; gap: 8px; align-items: center; padding: 10px;
      border-radius: 14px; border: 1px solid var(--w-glass-edge); }
    .mode:not(:has(.num)) { grid-template-columns: minmax(0, 1fr) auto; }
    .msig { grid-column: 1 / -1; }
    .add { justify-self: start; }
    .bought { display: grid; gap: 8px; padding: 14px; border-radius: 4px 4px 14px 14px; border-top: 2px dashed var(--w-glass-edge);
      background: color-mix(in srgb, var(--w-fg) 3%, transparent); }
    .four { display: grid; grid-template-columns: 1fr 1.4fr 1.2fr 1.6fr; gap: 10px; }
    .bad { color: var(--w-warn); margin: 8px 0 0; }
    @include bp.phone {
      .three, .pair, .heat { grid-template-columns: minmax(0, 1fr); }
      .four { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .mode { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; }
      .mname { grid-column: 1 / -1; }
    }
  `,
})
export class DeviceForm {
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  private readonly ref = inject(MatDialogRef<DeviceForm>);
  /** The device changed, or none when one is being added. */
  protected readonly editing = inject<{ device?: Device } | null>(MAT_DIALOG_DATA, { optional: true })?.device ?? null;

  protected readonly today = isoDay(new Date());
  /** The loads a fill usually holds, and the minutes a device usually waits, a tap away. */
  protected readonly loads = [0.1, 0.15, 0.2, 0.3];
  protected readonly offs = [1, 2, 3, 5, 10];
  protected readonly presetChoices: Choice[] = [
    { value: '', label: 'Nothing', icon: 'edit_note', hint: 'fill it in' },
    ...PRESETS.map((p) => ({ value: p.preset, label: p.model ?? p.name, icon: iconOf(p.category), hint: p.maker ?? labelOf(p.category) })),
  ];
  protected readonly categoryChoices: Choice[] = CATEGORIES.map((c) => ({ value: c.id, label: c.label, icon: c.icon }));
  protected readonly powerChoices: Choice[] = [
    { value: '', label: 'Not said' },
    { value: 'electric', label: 'Electric', icon: 'power' },
    { value: 'manual', label: 'Manual', icon: 'back_hand' },
  ];
  protected readonly signatureChoices: Choice[] = [{ value: '', label: 'Not said' }, ...SIGNATURES.map((x) => ({ value: x.id, label: x.label }))];
  protected readonly modeSignatures: Choice[] = [{ value: '', label: 'Heats as the device' }, ...SIGNATURES.map((x) => ({ value: x.id, label: x.label }))];
  protected readonly useChoices: Choice[] = [{ value: '', label: 'Not said' }, ...USES.map((x) => ({ value: x.id, label: x.label }))];
  protected readonly waterChoices: Choice[] = [
    { value: 'dry', label: 'Dry' },
    { value: 'water', label: 'With water', icon: 'water_drop', hint: 'a bubbler, a rig' },
  ];
  protected readonly retiredChoices: Choice[] = [
    { value: 'used', label: 'In use', icon: 'check_circle' },
    { value: 'retired', label: 'Retired', icon: 'inventory' },
  ];

  protected readonly d = signal<DeviceFields>(this.editing ? { ...this.editing, modes: [...this.editing.modes] } : blankDevice());
  protected readonly preset = signal('');
  protected readonly euros = signal<number | null>(this.editing?.cents ? this.editing.cents / 100 : null);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly cents = computed(() => Math.round((this.euros() ?? 0) * 100));
  /** The auto shut-off as the form asks it, in minutes; kept in seconds. */
  protected readonly autoOff = computed(() => {
    const seconds = this.d().auto_off;
    return seconds ? seconds / 60 : null;
  });

  protected setAutoOff(minutes: number | null): void {
    this.set('auto_off', minutes ? Math.round(minutes * 60) : undefined);
  }

  protected set<K extends keyof DeviceFields>(key: K, value: DeviceFields[K]): void {
    this.d.update((d) => ({ ...d, [key]: value }));
  }

  protected fromPreset(name: string): void {
    this.preset.set(name);
    const p = PRESETS.find((x) => x.preset === name);
    if (!p) {
      this.d.set(blankDevice(this.d().category));
      return;
    }
    const fields: DeviceFields & { preset?: string } = { ...p, modes: p.modes.map((m) => ({ ...m })) };
    delete fields.preset;
    this.d.set(fields);
  }

  /** The modes' usual temperatures, marked on the dial. */
  protected readonly marks = computed(() => this.d().modes.filter((m) => m.temp).map((m) => ({ temp: m.temp ?? 0, label: m.name })));

  /** Where the usual temperature starts when first said: the middle of the range, to the five degrees. */
  protected usual(): number {
    const lo = this.d().min_temp ?? 40;
    const hi = this.d().max_temp ?? 230;
    return Math.round((lo + hi) / 10) * 5;
  }

  protected text(e: Event): string {
    return (e.target as HTMLInputElement | HTMLTextAreaElement).value;
  }

  /** A number typed, or nothing; half of one — "0." — keeps what was there. */
  protected figure(e: Event): number | undefined {
    const el = e.target as HTMLInputElement;
    if (el.validity.badInput || el.value === '') {
      return undefined;
    }
    return Number(el.value);
  }

  protected pickCategory(id: string): void {
    const c = CATEGORIES.find((x) => x.id === id);
    if (c) {
      this.category(c.id);
    }
  }

  protected pickPower(p: string): void {
    this.power(p === 'electric' || p === 'manual' ? p : '');
  }

  protected asSignature(v: string): DeviceMode['signature'] {
    return SIGNATURES.find((x) => x.id === v)?.id ?? '';
  }

  protected asUse(v: string): DeviceFields['use'] {
    return USES.find((x) => x.id === v)?.id ?? '';
  }

  protected category(c: DeviceCategory): void {
    this.d.update((d) => ({ ...d, category: c, power: c === 'glass' ? 'manual' : d.power }));
  }

  /** Manual means used on demand: heated once, inhaled, the AVB tossed. */
  protected power(p: DeviceFields['power']): void {
    this.d.update((d) => ({ ...d, power: p, use: p === 'manual' && d.category === 'vaporizer' ? 'on-demand' : d.use }));
  }

  protected editMode(i: number, patch: Partial<DeviceMode>): void {
    this.d.update((d) => ({ ...d, modes: d.modes.map((m, n) => (n === i ? { ...m, ...patch } : m)) }));
  }

  protected addMode(): void {
    this.d.update((d) => ({ ...d, modes: [...d.modes, { name: '' }] }));
  }

  protected removeMode(i: number): void {
    this.d.update((d) => ({ ...d, modes: d.modes.filter((_, n) => n !== i) }));
  }

  protected async save(): Promise<void> {
    this.busy.set(true);
    this.error.set('');
    const device: DeviceFields = { ...this.d(), cents: this.cents() || undefined, modes: this.d().modes.filter((m) => m.name.trim()) };
    try {
      const out = this.editing ? await this.wits.editDevice(this.editing.slug, device) : await this.wits.addDevice(device);
      this.snack.open(out.message, undefined, { duration: 3000 });
      this.ref.close(true);
    } catch (e) {
      this.error.set(e instanceof Error ? e.message : String(e));
    } finally {
      this.busy.set(false);
    }
  }
}

/** A category's icon. */
function iconOf(category: DeviceCategory): string {
  return CATEGORIES.find((c) => c.id === category)?.icon ?? 'devices';
}

/** A category's name. */
function labelOf(category: DeviceCategory): string {
  return CATEGORIES.find((c) => c.id === category)?.label ?? category;
}
