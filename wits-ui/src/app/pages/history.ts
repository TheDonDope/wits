import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatIconModule } from '@angular/material/icon';

import { Wits } from '../data/wits';
import { CountUp } from '../viz/count-up';
import { FillRiver } from '../viz/fill-river';
import { Heatmap } from '../viz/heatmap';

/** The whole record: every day, every fill, every device. */
@Component({
  selector: 'w-history',
  imports: [CountUp, DecimalPipe, FillRiver, Heatmap, MatButtonToggleModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="strip">
      @for (s of totals(); track s.label) {
        <div class="glass stat">
          <span class="eyebrow">{{ s.label }}</span>
          <span class="v num"><w-count [value]="s.value" [decimals]="s.decimals" />{{ s.unit }}</span>
        </div>
      }
    </section>

    <section class="glass pad">
      <header>
        <span class="eyebrow">every day since the first fill</span>
        <mat-button-toggle-group [value]="metric()" (change)="metric.set($event.value)" aria-label="What to colour by">
          <mat-button-toggle value="ground">Grinding</mat-button-toggle>
          <mat-button-toggle value="seshed">Sessions</mat-button-toggle>
        </mat-button-toggle-group>
      </header>
      <w-heatmap [days]="wits.days.value()" [metric]="metric()"
        [tone]="metric() === 'ground' ? 'var(--w-stash)' : 'var(--w-sesh)'" />
    </section>

    <section class="glass pad">
      <header>
        <div>
          <span class="eyebrow">every fill, from pickup to empty</span>
          <p class="dim">A fill closes when its own jars are empty, not when the next one comes — so they overlap.</p>
        </div>
        <span class="scope">the fill</span>
      </header>
      <w-fill-river [cycles]="wits.cycles.value()" />
    </section>

    @if (wits.devices.value().length) {
      <section class="glass pad">
        <span class="eyebrow">devices</span>
        <div class="devices">
          @for (d of wits.devices.value(); track d.slug) {
            <div class="device">
              <mat-icon>local_fire_department</mat-icon>
              <div>
                <strong>{{ d.name }}</strong>
                <span class="dim">{{ d.sessions }} sessions · {{ d.grams | number: '1.2-2' }} g
                  @if (d.default_temp) { · {{ d.default_temp }}°C }</span>
              </div>
              <div class="use"><i [style.width.%]="(d.grams / mostDevice()) * 100"></i></div>
            </div>
          }
        </div>
      </section>
    }
  `,
  styles: `
    :host { display: grid; gap: 20px; }
    .strip { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 18px; }
    .stat { padding: 18px 22px; display: grid; gap: 6px; }
    /* Room under the line: text clipped to a gradient loses its descenders otherwise. */
    .v { font-size: 30px; font-weight: 600; line-height: 1.3; padding-bottom: 2px;
      background: linear-gradient(120deg, #fff, var(--w-mint));
      -webkit-background-clip: text; background-clip: text; color: transparent; }
    .pad { padding: 22px 26px; display: grid; gap: 14px; }
    header { display: flex; justify-content: space-between; gap: 16px; flex-wrap: wrap; align-items: flex-start; }
    header p { margin: 6px 0 0; font-size: 13px; }
    .devices { display: grid; gap: 14px; }
    .device { display: grid; grid-template-columns: 32px minmax(180px, 280px) 1fr; gap: 14px; align-items: center; }
    .device div:nth-child(2) { display: grid; }
    .device mat-icon { color: var(--w-sesh); }
    .use { height: 8px; border-radius: 99px; background: rgba(255,255,255,.06); overflow: hidden; }
    .use i { display: block; height: 100%; background: linear-gradient(90deg, var(--w-violet), var(--w-sesh));
      box-shadow: 0 0 12px var(--w-sesh); border-radius: 99px; }
  `,
})
export class History {
  protected readonly wits = inject(Wits);
  protected readonly metric = signal<'ground' | 'seshed'>('ground');

  protected readonly totals = computed(() => {
    const cycles = this.wits.cycles.value();
    const days = this.wits.days.value();
    const dispensed = cycles.reduce((s, c) => s + c.dispensed, 0);
    const ground = days.reduce((s, d) => s + d.ground, 0);
    const active = days.filter((d) => d.ground > 0).length;
    return [
      { label: 'fills', value: cycles.length, decimals: 0, unit: '' },
      { label: 'dispensed', value: dispensed, decimals: 0, unit: ' g' },
      { label: 'ground', value: ground, decimals: 0, unit: ' g' },
      { label: 'days with a grind', value: active, decimals: 0, unit: '' },
      { label: 'per active day', value: active ? ground / active : 0, decimals: 2, unit: ' g' },
    ];
  });

  protected readonly mostDevice = computed(() => Math.max(...this.wits.devices.value().map((d) => d.grams), 1));
}
