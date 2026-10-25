import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';

import { euros } from '../data/costs';
import { CATEGORIES, SIGNATURES, USES, heats, perUse } from '../data/devices';
import { Device } from '../data/models';
import { Wits } from '../data/wits';
import { DeviceForm } from './device-form';

const SIGNATURE = Object.fromEntries(SIGNATURES.map((s) => [s.id, s.label])) as Record<string, string>;
const USE = Object.fromEntries(USES.map((u) => [u.id, u.label])) as Record<string, string>;

/**
 * Every device, by what it is for: the vaporizers and glass sessions are had
 * with, the induction heaters and torches that heat the manual ones, the
 * grinders. Each says what it is, what it cost and where it was bought, and
 * what has been done with it — its sessions and their grams, the sessions it
 * heated, when it was last used, and what each use came to.
 */
@Component({
  selector: 'w-devices',
  imports: [DatePipe, MatButtonModule, MatIconModule, MatSlideToggleModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="glass head">
      <div>
        <span class="eyebrow">devices</span>
        <h1 class="num">{{ active().length }} <span class="dim">in use{{ retired() ? ' · ' + retired() + ' retired' : '' }}</span></h1>
      </div>
      <div class="actions">
        @if (retired()) {
          <mat-slide-toggle [checked]="showRetired()" (change)="showRetired.set($event.checked)">retired too</mat-slide-toggle>
        }
        <button mat-flat-button (click)="add()"><mat-icon>add</mat-icon>Add a device</button>
      </div>
    </section>

    @for (g of groups(); track g.id) {
      <section class="group">
        <span class="eyebrow"><mat-icon>{{ g.icon }}</mat-icon>{{ g.plural }} · {{ g.devices.length }}</span>
        <div class="cards">
          @for (d of g.devices; track d.slug) {
            <article class="glass card" [class.retired]="d.retired">
              <header>
                <div>
                  <strong>{{ d.name }}</strong>
                  <span class="dim small">{{ about(d) }}</span>
                </div>
                <button mat-icon-button (click)="edit(d)" [attr.aria-label]="'Change ' + d.name"><mat-icon>edit</mat-icon></button>
              </header>
              <div class="chips">
                @for (c of chips(d); track c) {
                  <span class="chip">{{ c }}</span>
                }
              </div>
              @if (d.modes.length) {
                <div class="modes">
                  <span class="dim small">{{ d.category === 'vaporizer' ? 'modes' : 'settings' }}</span>
                  @for (m of d.modes; track m.name) {
                    <span class="mode">{{ m.name }}<span class="dim">{{ modeDetail(m) }}</span></span>
                  }
                </div>
              }
              <div class="facts">
                @if (d.cents) {
                  <span><b class="num">{{ money(d.cents) }}</b>{{ d.shop ? ' at ' + d.shop : '' }}{{ d.bought_on ? ', ' + (d.bought_on | date: 'd MMM y') : '' }}</span>
                } @else if (d.shop || d.bought_on) {
                  <span>{{ d.shop ? 'bought at ' + d.shop : 'bought' }}{{ d.bought_on ? ', ' + (d.bought_on | date: 'd MMM y') : '' }}</span>
                }
                @if (uses(d)) {
                  <span class="dim">{{ usage(d) }}@if (d.last_used) { · last {{ d.last_used | date: 'd MMM y' }} }</span>
                }
                @if (perUse(d)) {
                  <span class="dim">{{ money(perUse(d)) }} a {{ isHeater(d) ? 'session heated' : 'session' }}, so far</span>
                }
              </div>
              @if (d.notes) {
                <p class="notes">{{ d.notes }}</p>
              }
              @if (d.link) {
                <a class="link" [href]="d.link" target="_blank" rel="noopener noreferrer"><mat-icon>open_in_new</mat-icon>{{ host(d.link) }}</a>
              }
            </article>
          }
        </div>
      </section>
    } @empty {
      <section class="glass empty">
        <mat-icon>air</mat-icon>
        <p>No devices yet. Add the vaporizers you use, and what heats and grinds for them — a session then says what it was had with.</p>
      </section>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .head { padding: 18px 24px; display: flex; flex-wrap: wrap; gap: 14px; align-items: center; justify-content: space-between; }
    h1 { font: 600 28px var(--w-num); margin: 4px 0 0; }
    h1 .dim { font: 400 15px var(--w-font); }
    .actions { display: flex; gap: 14px; align-items: center; flex-wrap: wrap; }
    .dim { color: var(--w-dim); }
    .small { font-size: 12px; }
    .group { display: grid; gap: 12px; }
    .group > .eyebrow { display: inline-flex; align-items: center; gap: 6px; }
    .group > .eyebrow mat-icon { font-size: 16px; width: 16px; height: 16px; }
    .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(320px, 100%), 1fr)); gap: 16px; }
    .card { padding: 16px 18px; display: grid; gap: 10px; align-content: start; }
    .card.retired { opacity: 0.6; }
    .card header { display: flex; justify-content: space-between; align-items: flex-start; gap: 8px; }
    .card header div { display: grid; gap: 2px; }
    .chips { display: flex; flex-wrap: wrap; gap: 6px; }
    .chip { font-size: 11px; padding: 2px 9px; border-radius: 99px; border: 1px solid var(--w-glass-edge);
      background: color-mix(in srgb, var(--w-fg) 4%, transparent); }
    .modes { display: flex; flex-wrap: wrap; gap: 6px; align-items: baseline; }
    .mode { font-size: 12px; padding: 2px 8px; border-radius: 6px; background: color-mix(in srgb, var(--w-primary) 12%, transparent); }
    .mode .dim { margin-left: 4px; }
    .facts { display: grid; gap: 2px; font-size: 13px; }
    .notes { margin: 0; font-size: 13px; color: var(--w-dim); white-space: pre-line; }
    .link { display: inline-flex; align-items: center; gap: 4px; font-size: 13px; color: var(--w-primary); text-decoration: none; }
    .link mat-icon { font-size: 15px; width: 15px; height: 15px; }
    .empty { padding: 32px; display: grid; justify-items: center; text-align: center; gap: 6px; }
    .empty mat-icon { font-size: 40px; width: 40px; height: 40px; color: var(--w-primary); }
    @include bp.phone { .head { padding: 16px; } }
  `,
})
export class Devices {
  private readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  protected readonly money = euros;
  protected readonly perUse = perUse;
  protected readonly isHeater = heats;
  protected readonly showRetired = signal(false);

  protected readonly active = computed(() => this.wits.devices.value().filter((d) => !d.retired));
  protected readonly retired = computed(() => this.wits.devices.value().length - this.active().length);
  protected readonly groups = computed(() => {
    const shown = this.showRetired() ? this.wits.devices.value() : this.active();
    return CATEGORIES.map((c) => ({ ...c, devices: shown.filter((d) => d.category === c.id) })).filter((g) => g.devices.length);
  });

  protected about(d: Device): string {
    const what = [d.maker, d.model && d.model !== d.name ? d.model : '', d.country].filter((x) => x);
    return what.join(' · ');
  }

  protected chips(d: Device): string[] {
    const out: string[] = [];
    if (d.power) {
      out.push(d.power === 'electric' ? 'Electric' : 'Manual');
    }
    if (d.signature) {
      out.push(SIGNATURE[d.signature]);
    }
    if (d.use) {
      out.push(USE[d.use]);
    }
    if (d.category === 'glass') {
      out.push(d.water ? 'With water' : 'Dry');
    }
    if (d.max_temp) {
      out.push(`${d.min_temp ?? '?'}–${d.max_temp} °C`);
    }
    if (d.auto_off) {
      out.push(`Off after ${d.auto_off % 60 ? `${d.auto_off} s` : `${d.auto_off / 60} min`}`);
    }
    if (d.retired) {
      out.push('Retired');
    }
    return out;
  }

  protected modeDetail(m: Device['modes'][number]): string {
    const parts = [m.signature ? SIGNATURE[m.signature].toLowerCase() : '', m.temp ? `${m.temp} °C` : '', m.timer ? `${m.timer} s` : ''];
    const text = parts.filter((x) => x).join(', ');
    return text ? ` · ${text}` : '';
  }

  protected uses(d: Device): number {
    return heats(d) ? d.heated : d.sessions;
  }

  protected usage(d: Device): string {
    if (heats(d)) {
      return `heated ${d.heated} ${d.heated === 1 ? 'session' : 'sessions'}`;
    }
    return `${d.sessions} ${d.sessions === 1 ? 'session' : 'sessions'}, ${d.grams.toFixed(2)} g`;
  }

  protected host(link: string): string {
    try {
      return new URL(link).host;
    } catch {
      return link;
    }
  }

  protected add(): void {
    this.dialog.open(DeviceForm, { maxWidth: 'min(720px, 94vw)', autoFocus: 'first-tabbable' });
  }

  protected edit(d: Device): void {
    this.dialog.open(DeviceForm, { maxWidth: 'min(720px, 94vw)', data: { device: d } });
  }
}
