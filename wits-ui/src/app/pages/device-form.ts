import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';

import { CATEGORIES, PRESETS, SIGNATURES, USES, blankDevice } from '../data/devices';
import { Device, DeviceCategory, DeviceFields, DeviceMode } from '../data/models';
import { Wits } from '../data/wits';

/**
 * Adding a device, or changing one: from a preset of what is known about it,
 * or from nothing. Only what the category has is asked: a vaporizer's power,
 * heating, use, temperatures and modes; an induction heater's settings; glass
 * with water or dry. A manual vaporizer is used on demand — heated once,
 * inhaled, its AVB tossed — so its use is not asked.
 */
@Component({
  selector: 'w-device-form',
  imports: [FormsModule, MatButtonModule, MatDialogModule, MatFormFieldModule, MatIconModule, MatInputModule, MatSelectModule,
    MatSlideToggleModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h2 mat-dialog-title>{{ editing ? 'Change ' + editing.name : 'Add a device' }}</h2>
    <mat-dialog-content>
      <div class="form">
        @if (!editing) {
          <mat-form-field appearance="outline" class="wide preset" subscriptSizing="dynamic">
            <mat-label>Start from</mat-label>
            <mat-select [ngModel]="preset()" (ngModelChange)="fromPreset($event)">
              <mat-option value="">Nothing — fill it in</mat-option>
              @for (p of presets; track p.preset) {
                <mat-option [value]="p.preset">{{ p.preset }}</mat-option>
              }
            </mat-select>
            <mat-hint>What is known of it, filled in; what could not be found stays empty.</mat-hint>
          </mat-form-field>
        }
        <mat-form-field appearance="outline" class="wide">
          <mat-label>Name</mat-label>
          <input matInput [ngModel]="d().name" (ngModelChange)="set('name', $event)" required />
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>What it is</mat-label>
          <mat-select [ngModel]="d().category" (ngModelChange)="category($event)">
            @for (c of categories; track c.id) {
              <mat-option [value]="c.id">{{ c.label }}</mat-option>
            }
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>Manufacturer</mat-label>
          <input matInput [ngModel]="d().maker ?? ''" (ngModelChange)="set('maker', $event)" />
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>Model</mat-label>
          <input matInput [ngModel]="d().model ?? ''" (ngModelChange)="set('model', $event)" />
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>The maker's country</mat-label>
          <input matInput [ngModel]="d().country ?? ''" (ngModelChange)="set('country', $event)" />
        </mat-form-field>

        @if (d().category === 'vaporizer' || d().category === 'grinder') {
          <mat-form-field appearance="outline">
            <mat-label>Power</mat-label>
            <mat-select [ngModel]="d().power ?? ''" (ngModelChange)="power($event)">
              <mat-option value="">Not said</mat-option>
              <mat-option value="electric">Electric</mat-option>
              <mat-option value="manual">Manual</mat-option>
            </mat-select>
          </mat-form-field>
        }
        @if (d().category === 'vaporizer' || d().category === 'glass') {
          <mat-form-field appearance="outline" subscriptSizing="dynamic">
            <mat-label>A load holds</mat-label>
            <input matInput type="number" min="0" step="0.01" [ngModel]="d().load ?? null"
              (ngModelChange)="set('load', $event || undefined)" />
            <span matTextSuffix>g</span>
            <mat-hint>As you usually fill it. Sizes a session you forgot to record.</mat-hint>
          </mat-form-field>
        }
        @if (d().category === 'vaporizer') {
          <mat-form-field appearance="outline">
            <mat-label>Heats by</mat-label>
            <mat-select [ngModel]="d().signature ?? ''" (ngModelChange)="set('signature', $event)">
              <mat-option value="">Not said</mat-option>
              @for (s of signatures; track s.id) {
                <mat-option [value]="s.id">{{ s.label }}</mat-option>
              }
            </mat-select>
          </mat-form-field>
          <mat-form-field appearance="outline" subscriptSizing="dynamic">
            <mat-label>Used</mat-label>
            <mat-select [ngModel]="d().use ?? ''" (ngModelChange)="set('use', $event)" [disabled]="d().power === 'manual'">
              <mat-option value="">Not said</mat-option>
              @for (u of uses; track u.id) {
                <mat-option [value]="u.id">{{ u.label }}</mat-option>
              }
            </mat-select>
            @if (d().power === 'manual') {
              <mat-hint>Manual: heated once, inhaled, what was vaped tossed.</mat-hint>
            }
          </mat-form-field>
          @if (d().power === 'electric') {
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Turns itself off after</mat-label>
              <input matInput type="number" min="0" step="0.5" [ngModel]="autoOff()" (ngModelChange)="setAutoOff($event)" />
              <span matTextSuffix>min</span>
              <mat-hint>Unused this long, it ends a session itself.</mat-hint>
            </mat-form-field>
            <div class="temps">
              <mat-form-field appearance="outline">
                <mat-label>From</mat-label>
                <input matInput type="number" [ngModel]="d().min_temp ?? null" (ngModelChange)="set('min_temp', $event ?? undefined)" />
                <span matTextSuffix>°C</span>
              </mat-form-field>
              <mat-form-field appearance="outline">
                <mat-label>To</mat-label>
                <input matInput type="number" [ngModel]="d().max_temp ?? null" (ngModelChange)="set('max_temp', $event ?? undefined)" />
                <span matTextSuffix>°C</span>
              </mat-form-field>
              <mat-form-field appearance="outline">
                <mat-label>Usually</mat-label>
                <input matInput type="number" [ngModel]="d().default_temp ?? null" (ngModelChange)="set('default_temp', $event ?? undefined)" />
                <span matTextSuffix>°C</span>
              </mat-form-field>
            </div>
          }
        }
        @if (d().category === 'glass') {
          <mat-slide-toggle class="wide" [checked]="d().water ?? false" (change)="set('water', $event.checked)">With water — a bubbler, a rig with water</mat-slide-toggle>
        }

        @if (d().category === 'vaporizer' || d().category === 'induction-heater') {
          <div class="modes wide">
            <span class="eyebrow">{{ d().category === 'vaporizer' ? 'modes' : 'settings' }}</span>
            <p class="dim">{{ d().category === 'vaporizer'
              ? 'What it is run in — a boost, a session mode — each heating its own way if it does, at a temperature it is usually run at, and with a session timer if it runs one. A session with it asks which, and starts from those.'
              : 'What it is set to — a wattage, a time. A session it heats asks which.' }}</p>
            @for (m of d().modes; track $index; let i = $index) {
              <div class="mode">
                <mat-form-field appearance="outline" subscriptSizing="dynamic">
                  <mat-label>{{ d().category === 'vaporizer' ? 'Mode' : 'Setting' }}</mat-label>
                  <input matInput [ngModel]="m.name" (ngModelChange)="editMode(i, { name: $event })" />
                </mat-form-field>
                @if (d().category === 'vaporizer') {
                  <mat-form-field appearance="outline" subscriptSizing="dynamic">
                    <mat-label>Heats by</mat-label>
                    <mat-select [ngModel]="m.signature ?? ''" (ngModelChange)="editMode(i, { signature: $event })">
                      <mat-option value="">As the device</mat-option>
                      @for (s of signatures; track s.id) {
                        <mat-option [value]="s.id">{{ s.label }}</mat-option>
                      }
                    </mat-select>
                  </mat-form-field>
                  <mat-form-field appearance="outline" subscriptSizing="dynamic" class="num-field">
                    <mat-label>Usually</mat-label>
                    <input matInput type="number" [ngModel]="m.temp ?? null" (ngModelChange)="editMode(i, { temp: $event || undefined })" />
                    <span matTextSuffix>°C</span>
                  </mat-form-field>
                  <mat-form-field appearance="outline" subscriptSizing="dynamic" class="num-field">
                    <mat-label>Timer</mat-label>
                    <input matInput type="number" min="0" [ngModel]="m.timer ?? null" (ngModelChange)="editMode(i, { timer: $event || undefined })" />
                    <span matTextSuffix>s</span>
                  </mat-form-field>
                }
                <button mat-icon-button type="button" (click)="removeMode(i)" aria-label="Remove"><mat-icon>close</mat-icon></button>
              </div>
            }
            <button mat-button type="button" (click)="addMode()"><mat-icon>add</mat-icon>{{ d().category === 'vaporizer' ? 'Add a mode' : 'Add a setting' }}</button>
          </div>
        }

        <mat-form-field appearance="outline">
          <mat-label>Price</mat-label>
          <input matInput type="number" min="0" step="0.01" [ngModel]="euros()" (ngModelChange)="euros.set($event)" />
          <span matTextSuffix>€</span>
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>Bought at</mat-label>
          <input matInput [ngModel]="d().shop ?? ''" (ngModelChange)="set('shop', $event)" />
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>Bought on</mat-label>
          <input matInput type="date" [ngModel]="d().bought_on ?? ''" (ngModelChange)="set('bought_on', $event)" />
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>Link</mat-label>
          <input matInput type="url" [ngModel]="d().link ?? ''" (ngModelChange)="set('link', $event)" />
        </mat-form-field>
        <mat-form-field appearance="outline" class="wide">
          <mat-label>Notes</mat-label>
          <textarea matInput rows="3" [ngModel]="d().notes ?? ''" (ngModelChange)="set('notes', $event)"></textarea>
        </mat-form-field>
        @if (editing) {
          <mat-slide-toggle class="wide" [checked]="d().retired ?? false" (change)="set('retired', $event.checked)">
            Retired — kept, since its sessions name it, but no longer offered</mat-slide-toggle>
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
    .form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 14px; padding-top: 6px; }
    .wide { grid-column: 1 / -1; }
    .preset { margin-bottom: 18px; }
    .temps { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 0 10px; }
    .modes { display: grid; gap: 8px; margin-bottom: 14px; }
    .modes p { margin: 0; font-size: 12px; }
    .mode { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr) 96px 96px auto; gap: 8px; align-items: center; }
    .mode:has(> mat-form-field:only-of-type) { grid-template-columns: minmax(0, 1fr) auto; }
    .dim { color: var(--w-dim); }
    .bad { color: var(--w-warn); margin: 8px 0 0; }
    mat-slide-toggle { margin-bottom: 14px; }
    @include bp.phone {
      .form { grid-template-columns: minmax(0, 1fr); }
      .temps { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .mode { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); padding-bottom: 6px;
        border-bottom: 1px solid var(--w-glass-edge); }
      .mode button { grid-column: 1 / -1; justify-self: end; margin-top: -4px; }
    }
  `,
})
export class DeviceForm {
  private readonly wits = inject(Wits);
  private readonly snack = inject(MatSnackBar);
  private readonly ref = inject(MatDialogRef<DeviceForm>);
  /** The device changed, or none when one is being added. */
  protected readonly editing = inject<{ device?: Device } | null>(MAT_DIALOG_DATA, { optional: true })?.device ?? null;

  protected readonly categories = CATEGORIES;
  protected readonly signatures = SIGNATURES;
  protected readonly uses = USES;
  protected readonly presets = PRESETS;

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
