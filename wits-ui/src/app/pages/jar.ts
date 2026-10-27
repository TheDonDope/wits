import { DatePipe, DecimalPipe } from '@angular/common';
import { httpResource } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { RouterLink } from '@angular/router';

import { familyColor, terpeneKey } from '../data/aroma';
import { euros } from '../data/costs';
import { Dimension, JarDetail } from '../data/models';
import { Wits } from '../data/wits';
import { scoreColor } from '../viz/effect-matrix';
import { AromaWheel } from '../viz/aroma-wheel';
import { GlassJar } from '../viz/glass-jar';
import { MonthBars } from '../viz/month-bars';
import { DescribeDialog } from './describe';
import { StrainRatingForm } from './strain-rating';
import { Terpenes } from './terpenes';

/**
 * One jar in full, opened from the shelf or the dashboard: what the product
 * is and its terpenes, every fill of it with the pickups and what they cost,
 * its grinding month by month, and its sessions — on which devices, how hot,
 * and how they went. The jar's figures are its whole life; each fill's are
 * that fill's own.
 */
@Component({
  selector: 'w-jar-detail',
  imports: [AromaWheel, DatePipe, DecimalPipe, GlassJar, MatButtonModule, MatIconModule, MatProgressSpinnerModule, MonthBars, RouterLink, StrainRatingForm, Terpenes],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <a mat-button routerLink="/shelf/jars" class="back"><mat-icon>arrow_back</mat-icon>The jars</a>
    @if (jar.value(); as j) {
      <section class="glass head">
        <w-jar [capacity]="j.bought"
          [layers]="[
            { grams: j.storage, color: 'var(--w-storage)', label: 'sealed' },
            { grams: j.stash, color: 'var(--w-stash)', label: 'ground, ready to use' },
          ]" />
        <div class="who">
          <span class="eyebrow">{{ j.manufacturer || 'the jar' }} <span class="scope">the jar · its whole life</span></span>
          <h1>{{ j.name }}</h1>
          <div class="chips">
            @if (j.strain) { <span class="chip">{{ j.strain }}</span> }
            @if (j.thc) { <span class="chip thc">THC {{ j.thc }}%</span> }
            @if (j.cbd) { <span class="chip cbd">CBD {{ j.cbd }}%</span> }
            @if (j.country) { <span class="chip">{{ j.country }}</span> }
            @if (j.radiated) { <span class="chip">irradiated</span> }
            <span class="chip">{{ j.fills }} {{ j.fills === 1 ? 'prescription' : 'prescriptions' }}</span>
          </div>
          <dl class="figures">
            <div><dt>sealed</dt><dd class="num" style="color: var(--w-storage)">{{ j.storage | number: '1.2-2' }}</dd></div>
            <div><dt>ready to use</dt><dd class="num" style="color: var(--w-stash)">{{ j.stash | number: '1.2-2' }}</dd></div>
            <div><dt>already vaped</dt><dd class="num" style="color: var(--w-avb)">{{ j.avb | number: '1.2-2' }}</dd></div>
            <div><dt>dispensed</dt><dd class="num">{{ j.bought | number: '1.2-2' }}</dd></div>
            <div><dt>ground in all</dt><dd class="num">{{ j.ground | number: '1.2-2' }}</dd></div>
            <div><dt>vaped</dt><dd class="num" style="color: var(--w-sesh)">{{ j.seshed | number: '1.2-2' }}</dd></div>
          </dl>
          <p class="dim since">
            First {{ j.first_seen | date: 'd MMMM y' }} · last {{ j.last_seen | date: 'd MMMM y' }}
            @if (j.flushed > 0) { · {{ j.flushed | number: '1.2-2' }} g of what was ground cleared unlogged }
          </p>
          <button mat-button class="fix" (click)="describe(j)"><mat-icon>edit</mat-icon>Correct the label</button>
        </div>
      </section>

      <div class="cols">
        <section class="glass pad">
          <header><span class="eyebrow">terpene profile</span><span class="scope">the product</span></header>
          <w-terpenes [slug]="j.slug" [name]="j.name" [terpenes]="j.terpenes" />
          @if (j.taste) {
            <div class="tasting">
              <w-aroma-wheel [aroma]="j.aroma" [of]="j.name" />
              <div class="tastes">
                <p class="taste">{{ j.taste }}</p>
                <ul>
                  @for (t of tasting(); track t.name) {
                    <li><i class="dot" [style.background]="t.color"></i><b>{{ t.name }}</b>
                      <span class="dim">{{ t.flavors.join(', ') || 'no taste known' }}</span></li>
                  }
                </ul>
              </div>
            </div>
          }
        </section>

        <section class="glass pad">
          <header><span class="eyebrow">what it cost</span><span class="scope">the jar</span></header>
          @if (j.price.priced > 0) {
            <p class="big num">{{ money(j.price.per_gram) }} <small class="dim">a gram</small></p>
            <p class="dim note">{{ money(j.price.cents) }} for {{ j.price.priced | number: '1.2-2' }} g
              @if (j.price.unpriced) { · {{ j.price.unpriced }} {{ j.price.unpriced === 1 ? 'pickup' : 'pickups' }} not priced yet }</p>
          } @else {
            <p class="dim note">No pickup of it priced yet{{ j.price.unpriced ? '' : ', and none to be had' }}.</p>
          }
        </section>
      </div>

      <section class="glass pad">
        <header>
          <span class="eyebrow">your rating</span>
          <span class="scope">the strain as a whole{{ j.rating?.at ? ' · ' + (j.rating?.at | date: 'd MMM y') : '' }}</span>
        </header>
        <w-strain-rating [slug]="j.slug" [rating]="j.rating" />
      </section>

      <section class="glass pad">
        <header><span class="eyebrow">pickups, prescription by prescription</span><span class="scope">each prescription's own grams</span></header>
        @for (f of j.fill_list; track f.number) {
          <div class="fill">
            <div class="fill-head">
              <strong>Prescription {{ f.number }}</strong>
              <span class="dim">{{ f.start | date: 'd MMM y' }}</span>
              <span class="num">{{ f.sealed | number: '1.2-2' }} g <span class="dim">of {{ f.dispensed | number: '1.2-2' }} still sealed</span></span>
              <span class="state" [class.open]="f.sealed > 0">{{ f.sealed > 0 ? 'still sealed' : 'all ground' }}</span>
            </div>
            @for (p of f.pickups; track p.hash) {
              <div class="pickup">
                <mat-icon>local_pharmacy</mat-icon>
                <span>{{ p.at | date: 'EEE d MMM y' }}</span>
                <span class="num">{{ p.grams | number: '1.2-2' }} g</span>
                <span class="num price">
                  @if (p.priced) {
                    {{ money(p.cents ?? 0) }} <span class="dim">· {{ money(p.per_gram ?? 0) }}/g</span>
                  } @else if (p.unknown) {
                    <span class="dim">no price to be had</span>
                  } @else {
                    <span class="dim">not priced</span>
                  }
                </span>
                <span class="dim seller">{{ p.seller }}</span>
              </div>
            }
          </div>
        }
      </section>

      @if (j.months.length) {
        <section class="glass pad">
          <header>
            <span class="eyebrow">month by month</span>
            <span class="legend"><i style="background: var(--w-stash)"></i>ground <i style="background: var(--w-sesh)"></i>vaped</span>
          </header>
          <w-month-bars [months]="j.months" />
        </section>
      }

      <section class="glass pad">
        <header><span class="eyebrow">sessions</span><span class="scope">the jar</span></header>
        @if (j.sessions.count) {
          <div class="facts">
            <div><span class="big num">{{ j.sessions.count }}</span><span class="dim">sessions · {{ j.sessions.grams | number: '1.2-2' }} g</span></div>
            @if (j.sessions.min_temp) {
              <div><span class="big num">{{ j.sessions.min_temp }}{{ j.sessions.max_temp !== j.sessions.min_temp ? '–' + j.sessions.max_temp : '' }}°</span>
                <span class="dim">the temperatures set</span></div>
            }
            <div><span class="big num">{{ j.sessions.rated }}</span><span class="dim">rated</span></div>
          </div>
          <div class="devices">
            @for (d of j.sessions.devices; track d.slug) {
              <span class="chip">{{ d.name }} · {{ d.sessions }}× · {{ d.grams | number: '1.2-2' }} g</span>
            }
          </div>
          @if (scores().length) {
            <div class="scores">
              @for (s of scores(); track s.dimension) {
                <div class="score">
                  <span class="l">{{ s.dimension }}</span>
                  <div class="track"><div [style.width.%]="((s.mean - 1) / 4) * 100" [style.background]="s.color"></div></div>
                  <span class="num">{{ s.mean | number: '1.1-1' }}</span>
                </div>
              }
            </div>
          }
          <div class="recent">
            @for (r of j.sessions.recent; track r.session.hash) {
              <div class="sesh">
                <span>{{ r.session.occurred_at | date: (timed(r.session.occurred_at) ? 'd MMM y, HH:mm' : 'd MMM y') }}</span>
                <span class="num">{{ r.session.grams | number: '1.2-2' }} g</span>
                <span class="dim">{{ deviceName(r.session.device) }}{{ r.session.temperature ? ' · ' + r.session.temperature + '°' : '' }}</span>
                <span class="dim">{{ r.hash ? 'overall ' + (r.scores.overall ?? '–') : 'not rated' }}</span>
              </div>
            }
          </div>
        } @else {
          <p class="dim note">No session logged with it{{ j.flushed > 0 ? ' — what was ground of it was cleared unlogged' : '' }}.</p>
        }
      </section>

      @if (j.like.length) {
        <section class="glass pad">
          <header><span class="eyebrow">most like it</span><span class="scope">by terpene profile, then name</span></header>
          <ul class="likes">
            @for (l of j.like; track l.slug) {
              <li>
                <a [routerLink]="['/shelf/jars', l.slug]">
                  <span class="score num">{{ l.score * 100 | number: '1.0-0' }}%</span>
                  <span class="bar"><i [style.width.%]="l.score * 100"></i></span>
                  <span class="who">{{ l.name }}</span>
                  <span class="dim shares">{{ l.shared.length ? 'shares ' + l.shared.join(', ') : 'no terpene in common' }}</span>
                </a>
              </li>
            }
          </ul>
        </section>
      }
    } @else if (jar.error()) {
      <section class="glass pad"><p class="dim">No jar by that name in this repository.</p></section>
    } @else {
      <div class="loading"><mat-spinner diameter="42" /></div>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .back { justify-self: start; }
    .head { display: grid; grid-template-columns: 120px minmax(0, 1fr); gap: 28px; padding: 24px 28px; align-items: center; }
    .head w-jar { width: 120px; height: 168px; }
    .who { display: grid; gap: 10px; min-width: 0; }
    .fix { justify-self: start; margin-left: -12px; }
    h1 { font: 600 26px/1.2 var(--w-font); margin: 0; letter-spacing: -0.01em; }
    .chips, .devices { display: flex; gap: 6px; flex-wrap: wrap; }
    .chip { font: 600 11px/1.9 var(--w-font); padding: 0 10px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent);
      color: var(--w-dim); text-decoration: none; }
    .chip.thc { background: color-mix(in srgb, var(--w-accent) 12%, transparent); color: var(--w-accent); }
    .chip.cbd { background: color-mix(in srgb, var(--w-storage) 14%, transparent); color: var(--w-storage); }
    .chip.link { color: var(--w-primary); }
    .chip.link:hover { background: color-mix(in srgb, var(--w-primary) 14%, transparent); }
    .figures { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 8px 16px; margin: 4px 0 0; }
    .figures div { display: grid; gap: 1px; min-width: 0; }
    dt { font: 600 9px var(--w-font); letter-spacing: .1em; text-transform: uppercase; color: var(--w-dim); }
    dd { margin: 0; font-size: 18px; font-weight: 600; }
    .since { margin: 0; font-size: 12px; }
    .dim { color: var(--w-dim); }
    .cols { display: grid; grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); gap: 20px; align-items: start; }
    .pad { padding: 20px 24px; display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; align-content: start; }
    .pad header { display: flex; justify-content: space-between; align-items: center; gap: 10px; }
    .note { margin: 0; font-size: 13px; }
    .big { font: 600 26px var(--w-num); margin: 0; }
    .big small { font: 400 13px var(--w-font); }
    .legend { display: flex; gap: 6px; align-items: center; font-size: 12px; color: var(--w-dim); }
    .legend i { width: 10px; height: 10px; border-radius: 3px; display: inline-block; margin-left: 8px; }
    .fill { display: grid; gap: 4px; padding: 10px 0; border-top: 1px solid var(--w-glass-edge); }
    .fill-head { display: flex; flex-wrap: wrap; gap: 6px 14px; align-items: baseline; }
    .state { margin-left: auto; font-size: 11px; padding: 0 8px; border-radius: 99px; color: var(--w-dim);
      background: color-mix(in srgb, var(--w-fg) 6%, transparent); }
    .state.open { color: var(--w-storage); background: color-mix(in srgb, var(--w-storage) 14%, transparent); }
    .pickup { display: grid; grid-template-columns: auto 150px 80px minmax(0, 1fr) auto; gap: 12px; align-items: center; font-size: 13px; }
    .pickup mat-icon { font-size: 18px; width: 18px; height: 18px; color: var(--w-primary); }
    .seller { text-align: right; }
    .facts { display: flex; flex-wrap: wrap; gap: 28px; }
    .facts div { display: grid; }
    .scores { display: grid; gap: 6px; }
    .score { display: grid; grid-template-columns: 80px minmax(0, 1fr) 36px; gap: 10px; align-items: center; font-size: 12px; }
    .score .l { text-transform: capitalize; color: var(--w-dim); }
    .track { height: 6px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); overflow: hidden; }
    .track div { height: 100%; border-radius: 99px; min-width: 4px; }
    .recent { display: grid; }
    .sesh { display: grid; grid-template-columns: 160px 70px minmax(0, 1fr) auto; gap: 12px; padding: 6px 0;
      border-top: 1px solid var(--w-glass-edge); font-size: 13px; }
    .loading { display: grid; place-items: center; height: 50vh; }
    .tasting { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: 16px; align-items: center; }
    .taste { margin: 0 0 6px; font: 600 15px var(--w-font); }
    .tastes ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 4px; font-size: 13px; }
    .tastes li { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 8px; }
    .dot { width: 9px; height: 9px; border-radius: 50%; display: inline-block; align-self: center; }
    .likes { list-style: none; margin: 0; padding: 0; display: grid; }
    .likes a { display: grid; grid-template-columns: 48px 120px minmax(0, 1fr) minmax(0, 1.4fr); gap: 12px; align-items: center;
      padding: 7px 4px; border-top: 1px solid var(--w-glass-edge); color: inherit; text-decoration: none; font-size: 13px; }
    .likes a:hover { background: color-mix(in srgb, var(--w-primary) 8%, transparent); }
    .likes .score { font-weight: 600; color: var(--w-primary); }
    .likes .bar { height: 6px; border-radius: 99px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); overflow: hidden; }
    .likes .bar i { display: block; height: 100%; background: var(--w-primary); border-radius: 99px; }
    .likes .shares { font-size: 12px; }
    @include bp.phone {
      /* The jar small in the corner, so the name has the width. */
      .head { grid-template-columns: minmax(0, 1fr); padding: 16px; position: relative; }
      .head w-jar { position: absolute; top: 16px; right: 16px; width: 44px; height: 62px; }
      .who > .eyebrow, h1 { padding-right: 56px; }
      h1 { font-size: 20px; }
      .figures { grid-template-columns: repeat(3, minmax(0, 1fr)); }
      .pad header { flex-wrap: wrap; }
      .cols { grid-template-columns: minmax(0, 1fr); }
      .pad { padding: 16px; }
      .pickup { grid-template-columns: auto minmax(0, 1fr) auto; }
      .pickup .price { grid-column: 2 / -1; }
      .pickup .seller { display: none; }
      .sesh { grid-template-columns: minmax(0, 1fr) auto; }
      .tasting { grid-template-columns: minmax(0, 1fr); justify-items: center; }
      .tasting w-aroma-wheel { width: 240px; }
      .likes a { grid-template-columns: 44px minmax(0, 1fr); }
      .likes .bar { display: none; }
      .likes .shares { grid-column: 2; }
      .state { margin-left: 0; }
    }
  `,
})
export class JarPage {
  private readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  /** The jar's slug, off the route. */
  readonly slug = input.required<string>();

  protected readonly jar = httpResource<JarDetail>(
    () => `/api/jars/${encodeURIComponent(this.slug())}?v=${String(this.wits.version())}`,
  );

  protected readonly money = euros;

  /** Opens the label of the jar to be corrected: its name, maker, cultivar, THC and CBD. */
  protected describe(j: JarDetail): void {
    this.dialog.open(DescribeDialog, { data: j, autoFocus: 'first-tabbable' });
  }

  /** Each dimension rated, its mean, in the order the dimensions are asked. */
  protected readonly scores = computed(() => {
    const scores = this.jar.value()?.sessions.scores ?? {};
    const order: Dimension[] = ['overall', 'pain', 'sleep', 'calm', 'mood', 'focus', 'appetite'];
    return order.flatMap((dimension) => {
      const mean = scores[dimension];
      return mean === undefined ? [] : [{ dimension, mean, color: scoreColor(mean) }];
    });
  });

  /** Whether an entry was given a time of day: one dated only stands at midnight, and says no time. */
  protected timed(at: string): boolean {
    const d = new Date(at);
    return d.getHours() !== 0 || d.getMinutes() !== 0;
  }

  /** Each of the jar's terpenes with what it tastes of, from what the shelf's terpenes say. */
  protected readonly tasting = computed(() => {
    const guide = new Map(this.wits.shelfTerpenes.value().map((t) => [terpeneKey(t.name), t]));
    return (this.jar.value()?.terpenes ?? []).map((name) => {
      const t = guide.get(terpeneKey(name));
      return { name, flavors: t?.flavors ?? [], color: familyColor(t?.family) };
    });
  });

  /** A device's name, or what a session that named none is called. */
  protected deviceName(slug?: string): string {
    return slug ? (this.wits.devices.value().find((d) => d.slug === slug)?.name ?? slug) : 'no device named';
  }
}
