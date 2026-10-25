import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTooltipModule } from '@angular/material/tooltip';

import { Wits } from '../data/wits';
import { CountUp } from '../viz/count-up';
import { FillRing } from '../viz/fill-ring';
import { GlassJar } from '../viz/glass-jar';
import { Heatmap } from '../viz/heatmap';
import { ProjectionChart } from '../viz/projection-chart';

@Component({
  selector: 'w-dashboard',
  imports: [
    CountUp,
    DatePipe,
    DecimalPipe,
    FillRing,
    GlassJar,
    Heatmap,
    MatIconModule,
    MatProgressSpinnerModule,
    MatTooltipModule,
    ProjectionChart,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (wits.overview.value(); as o) {
      @if (o.fill; as f) {
        <section class="hero glass">
          <div class="ring">
            <w-fill-ring [dispensed]="f.dispensed" [sealed]="f.remaining" [stash]="ground().stash"
              [avb]="ground().avb" [carried]="f.carried.grams" />
          </div>
          <div class="facts">
            <div class="eyebrow">Fill {{ f.number }} · <span class="scope">the fill</span></div>
            <h1>
              Day <w-count [value]="f.day" [decimals]="0" />
              <span class="dim">since {{ f.start | date: 'd MMMM' }}</span>
            </h1>
            <div class="stats">
              <div class="stat">
                <span class="eyebrow">days' worth</span>
                <span class="v num"><w-count [value]="f.days_worth" [decimals]="0" /></span>
                <span class="dim">at {{ f.per_active_day | number: '1.2-2' }} g a day</span>
              </div>
              <div class="stat">
                <span class="eyebrow">still sealed</span>
                <span class="v num"><w-count [value]="f.remaining_pct * 100" [decimals]="0" />%</span>
                <span class="dim">of this fill</span>
              </div>
            </div>
            <div class="legend">
              <span><i style="background: var(--w-storage)"></i>sealed</span>
              <span><i style="background: var(--w-stash)"></i>ground</span>
              <span><i style="background: var(--w-avb)"></i>AVB</span>
              @if (f.carried.grams > 0) {
                <span><i style="background: var(--w-primary)"></i>older jars</span>
              }
            </div>
            @if (f.carried.grams > 0) {
              <p class="carried">
                <mat-icon>history</mat-icon>
                + {{ f.carried.grams | number: '1.2-2' }} g in {{ f.carried.jars }} older
                {{ f.carried.jars === 1 ? 'jar' : 'jars' }}, from {{ f.carried.cycles }} earlier
                {{ f.carried.cycles === 1 ? 'fill' : 'fills' }} still open
              </p>
            }
          </div>
        </section>

        <section class="shelf">
          @for (t of tiles(); track t.label) {
            <div class="tile glass" [style.--tone]="t.tone">
              <span class="eyebrow">{{ t.label }} <span class="scope">the shelf</span></span>
              <span class="tile-v num"><w-count [value]="t.grams" /><small> g</small></span>
              <div class="bar"><div [style.width.%]="t.share"></div></div>
              @if (t.label === 'AVB' && lastYield(); as y) {
                <span class="yield">last emptying gave back {{ y.ratio * 100 | number: '1.0-0' }}% —
                  {{ y.collected | number: '1.2-2' }} g of {{ y.from | number: '1.2-2' }} g</span>
              }
            </div>
          }
        </section>

        <section class="jars glass">
          <header>
            <span class="eyebrow">this fill, jar by jar</span>
            <span class="scope">the fill</span>
          </header>
          <div class="row">
            @for (p of f.products; track p.slug) {
              <figure>
                <w-jar [capacity]="p.dispensed"
                  [layers]="[
                    { grams: p.sealed, color: 'var(--w-storage)', label: 'sealed' },
                    { grams: p.stash, color: 'var(--w-stash)', label: 'ground' },
                    { grams: p.avb, color: 'var(--w-avb)', label: 'AVB' },
                  ]" />
                <figcaption>
                  <strong>{{ p.name }}</strong>
                  <span class="num">{{ p.sealed | number: '1.2-2' }} g <span class="dim">of {{ p.dispensed | number: '1.0-0' }}</span></span>
                </figcaption>
              </figure>
            }
          </div>
        </section>

        <section class="projection glass">
          <header>
            <div>
              <span class="eyebrow">supply projection</span>
              <h2>
                @if (f.shelf_projection.empty_on; as e) {
                  Empty around <span class="warn">{{ e | date: 'd MMMM' }}</span>
                } @else {
                  No rate to project from yet
                }
              </h2>
              @if (f.refill; as r) {
                <p class="refill" [class.warn]="r.overdue">
                  <mat-icon>event</mat-icon>
                  @if (r.overdue) {
                    Order the next prescription now — {{ r.order_by | date: 'd MMMM' }} has passed
                  } @else {
                    Order the next prescription by <b>{{ r.order_by | date: 'EEE d MMMM' }}</b>
                  }
                  <span class="dim">· {{ r.lead_days }} {{ r.lead_days === 1 ? 'day' : 'days' }} for the pharmacy</span>
                </p>
              }
            </div>
            <span class="scope" matTooltip="Everything standing when this fill arrived, less every grind since">the shelf</span>
          </header>
          <w-projection [history]="f.shelf_projection.history" [projected]="f.shelf_projection.projected" />
        </section>
      } @else {
        <section class="empty glass">
          <mat-icon>inventory_2</mat-icon>
          <h1>No fill in progress</h1>
          <p class="dim">Record a prescription fill to begin — the shelf is empty.</p>
        </section>
      }

      <section class="rhythm">
        <div class="glass pad">
          <span class="eyebrow">grinding · last 20 weeks</span>
          <w-heatmap [days]="wits.days.value()" metric="ground" [weeks]="20" tone="var(--w-stash)" />
        </div>
        <div class="glass pad">
          <span class="eyebrow">sessions · last 20 weeks</span>
          <w-heatmap [days]="wits.days.value()" metric="seshed" [weeks]="20" tone="var(--w-sesh)" />
        </div>
        <div class="glass pad">
          <span class="eyebrow">the journal</span>
          <div class="chain" [class.bad]="!o.chain_ok">
            <mat-icon>{{ o.chain_ok ? 'verified' : 'gpp_bad' }}</mat-icon>
            {{ o.chain_ok ? 'the journal’s hash chain verifies' : 'the journal was changed outside wits — run wits fsck' }}
          </div>
        </div>
      </section>
    } @else if (wits.overview.error()) {
      <section class="empty glass">
        <mat-icon>cloud_off</mat-icon>
        <h1>wits-server is not answering</h1>
        <p class="dim">Start it in your repository: <code>wits-server</code></p>
      </section>
    } @else {
      <div class="loading"><mat-spinner diameter="42" /></div>
    }
  `,
  styles: `
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .hero { display: grid; grid-template-columns: minmax(220px, 340px) 1fr; gap: 32px; padding: 28px 32px; align-items: center; }
    @media (max-width: 760px) { .hero { grid-template-columns: 1fr; justify-items: center; text-align: center; } }
    h1 { font: 600 34px/1.1 var(--w-num); margin: 8px 0 18px; letter-spacing: -0.03em; }
    h1 .dim { font: 400 16px var(--w-font); margin-left: 10px; letter-spacing: 0; }
    h2 { font: 600 22px var(--w-num); margin: 6px 0 0; }
    .warn { color: var(--w-warn); }
    .stats { display: flex; gap: 36px; flex-wrap: wrap; }
    .stat { display: grid; gap: 2px; }
    .stat .v { font-size: 40px; font-weight: 600; line-height: 1.05; }
    .legend { display: flex; gap: 16px; margin-top: 22px; flex-wrap: wrap; font-size: 12px; color: var(--w-dim); }
    .legend i { width: 10px; height: 10px; border-radius: 3px; display: inline-block; margin-right: 6px; }
    .carried { display: flex; gap: 8px; align-items: center; color: var(--w-primary); margin: 16px 0 0; font-size: 13px; }
    .shelf { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; }
    .tile { padding: 18px 22px; display: grid; gap: 6px; }
    .tile-v { font-size: 30px; font-weight: 600; color: var(--tone); }
    .tile-v small { font-size: 14px; color: var(--w-dim); }
    .bar { height: 6px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); overflow: hidden; }
    .bar div { height: 100%; background: var(--tone); border-radius: 99px; box-shadow: 0 0 12px var(--tone);
      transition: width 1s cubic-bezier(.2,.8,.2,1); }
    .jars, .projection { padding: 22px 26px; }
    .refill { display: flex; align-items: center; gap: 6px; margin: 6px 0 0; color: var(--mat-sys-on-surface); }
    .refill mat-icon { font-size: 18px; width: 18px; height: 18px; color: var(--w-accent); }
    .refill .dim { color: var(--w-dim); }
    .yield { display: block; margin-top: 8px; font-size: 12px; color: var(--w-dim); }
    header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
    .row { display: flex; gap: 28px; flex-wrap: wrap; justify-content: space-around; }
    figure { margin: 0; display: grid; justify-items: center; gap: 10px; width: 150px; }
    figure w-jar { width: 110px; height: 154px; }
    figcaption { display: grid; text-align: center; gap: 2px; font-size: 13px; }
    .rhythm { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
    .pad { padding: 18px 22px; display: grid; gap: 12px; align-content: start; }
    .chain { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--w-stash); margin-top: auto; }
    .chain.bad { color: var(--w-warn); }
    .empty { padding: 48px; display: grid; justify-items: center; text-align: center; }
    .empty mat-icon { font-size: 48px; width: 48px; height: 48px; color: var(--w-primary); }
    .loading { display: grid; place-items: center; height: 50vh; }
  `,
})
export class Dashboard {
  protected readonly wits = inject(Wits);

  /** The fill's ground grams, split as its rows split them. */
  protected readonly ground = computed(() => {
    const ps = this.wits.overview.value()?.fill?.products ?? [];
    return {
      stash: ps.reduce((s, p) => s + p.stash, 0),
      avb: ps.reduce((s, p) => s + p.avb, 0),
    };
  });

  /** The newest collection's yield, for the AVB tile. */
  protected readonly lastYield = computed(() => this.wits.avb.value()?.yields.at(-1));

  protected readonly tiles = computed(() => {
    const s = this.wits.overview.value()?.shelf;
    if (!s) {
      return [];
    }
    const top = Math.max(s.storage, s.stash, s.avb, 1);
    return [
      { label: 'in storage', grams: s.storage, tone: 'var(--w-storage)', share: (s.storage / top) * 100 },
      { label: 'in the stashes', grams: s.stash, tone: 'var(--w-stash)', share: (s.stash / top) * 100 },
      { label: 'AVB', grams: s.avb, tone: 'var(--w-avb)', share: (s.avb / top) * 100 },
    ];
  });
}
