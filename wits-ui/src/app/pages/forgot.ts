import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { RATE_DAYS, RATE_STRAIN_DAYS, Todo } from '../data/todo';
import { Wits } from '../data/wits';
import { PriceWalk } from './price-walk';
import { RateQueue } from './rate-queue';
import { openRecord } from './record';
import { StashClear } from './stash-clear';
import { StrainRatingForm } from './strain-rating';
import { TerpeneQueue } from './terpene-queue';

/**
 * Everything left undone, in one place: sessions to rate, pickups to price,
 * stashes to clear, AVB to collect, strains to rate — each shown only while there is some,
 * and counted on the tab — and the jars still sealed, to weigh now and then,
 * and those with no terpene profile, to look up.
 */
@Component({
  selector: 'w-forgot',
  imports: [DatePipe, DecimalPipe, MatButtonModule, MatIconModule, PriceWalk, RateQueue, RouterLink, StashClear, StrainRatingForm, TerpeneQueue],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="glass head">
      <div>
        <span class="eyebrow">i forgot to…</span>
        <h1 class="num">{{ todo.count() || 'Nothing' }} <span class="dim">{{ todo.count() === 1 ? 'thing waits' : 'things wait' }}</span></h1>
      </div>
      <p class="dim lede">What the ledger can tell was left undone. Each goes from here as soon as it is done.</p>
    </section>

    @if (todo.rate().length) {
      <section class="glass pad">
        <header><span class="eyebrow">rate a session</span><span class="chip num">{{ todo.rate().length }}</span></header>
        <p class="dim note">Sessions of the last {{ rateDays }} days, while you still remember how they went.</p>
        <w-rate-queue [sessions]="todo.rate()" [dimensions]="dimensions()" />
      </section>
    }

    @if (todo.strains().length) {
      <section class="glass pad">
        <header><span class="eyebrow">rate a strain</span><span class="chip num">{{ todo.strains().length }}</span></header>
        <p class="dim note">Strains used in the last {{ strainDays }} days, rated as a whole — its taste, its aroma, whether
          you would have it again — apart from any one session.</p>
        <div class="strains">
          @for (j of allStrains() ? todo.strains() : todo.strains().slice(0, 3); track j.slug) {
            <article class="strain glass">
              <header>
                <a [routerLink]="['/shelf/jars', j.slug]"><strong>{{ j.name }}</strong></a>
                <span class="dim">last used {{ j.last_used | date: 'd MMM' }}{{ j.terpenes.length ? ' · ' + j.terpenes.slice(0, 3).join(', ') : '' }}</span>
              </header>
              <w-strain-rating [slug]="j.slug" />
            </article>
          }
        </div>
        @if (todo.strains().length > 3) {
          <button mat-button class="more" (click)="allStrains.set(!allStrains())">
            {{ allStrains() ? 'Fewer' : 'All ' + todo.strains().length }}
          </button>
        }
      </section>
    }

    @if (unpricedAtAll()) {
      <section class="glass pad">
        <header><span class="eyebrow">price a pickup</span>
          @if (todo.pickups()) { <span class="chip num">{{ todo.pickups() }}</span> }</header>
        <w-price-walk />
      </section>
    }

    @if (todo.stashes().length) {
      <section class="glass pad">
        <header><span class="eyebrow">clear the stashes</span><span class="chip num">{{ todo.stashes().length }}</span></header>
        <p class="dim note">Jars with nothing sealed left whose stash still holds grams: ground, and the sessions never logged.</p>
        <w-stash-clear [jars]="todo.stashes()" />
      </section>
    }

    @if (todo.avb().length) {
      <section class="glass pad">
        <header><span class="eyebrow">collect the AVB</span><span class="chip num">{{ todo.avb().length }}</span></header>
        @for (w of todo.avb(); track w.device) {
          <div class="row">
            <mat-icon>recycling</mat-icon>
            <span class="name">{{ deviceName(w.device) }}</span>
            <span class="dim">{{ w.grams | number: '1.2-2' }} g in {{ w.sessions }} {{ w.sessions === 1 ? 'session' : 'sessions' }}</span>
            <button mat-stroked-button (click)="collect(w.device)">Collect</button>
          </div>
        }
      </section>
    }

    @if (todo.sealed().length) {
      <section class="glass pad">
        <header><span class="eyebrow">weigh against the scale</span></header>
        <p class="dim note">Now and then, so the ledger and the jar agree. A weighing that agrees writes nothing, so these are offered, never counted.</p>
        @for (j of todo.sealed(); track j.slug) {
          <div class="row">
            <mat-icon>scale</mat-icon>
            <span class="name">{{ j.name }}</span>
            <span class="dim num">{{ j.storage | number: '1.2-2' }} g sealed</span>
            <button mat-stroked-button (click)="weigh(j.slug)">Weigh</button>
          </div>
        }
      </section>
    }

    @if (todo.terpenes().length) {
      <section class="glass pad">
        <header><span class="eyebrow">terpene profile</span><span class="chip num">{{ todo.terpenes().length }}</span></header>
        <p class="dim note">Jars with no terpenes written down. Each is looked up on the web — a search for the product as the
          pharmacy sells it, and its strain on Leafly — and what is found waits here to be kept. Offered, never counted.</p>
        <w-terpene-queue [jars]="todo.terpenes()" />
      </section>
    }

    @if (done().length) {
      <p class="dim done"><mat-icon>check_circle</mat-icon> {{ done().join(' · ') }}</p>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .head { padding: 18px 24px; display: flex; flex-wrap: wrap; gap: 18px; align-items: center; justify-content: space-between; }
    h1 { font: 600 28px var(--w-num); margin: 4px 0 0; }
    h1 .dim { font: 400 15px var(--w-font); }
    .lede { max-width: 420px; margin: 0; font-size: 13px; }
    .pad { padding: 20px 24px; display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; }
    .pad header { display: flex; align-items: center; gap: 10px; }
    .chip { padding: 1px 9px; border-radius: 99px; font-size: 12px; font-weight: 600;
      background: color-mix(in srgb, var(--w-primary) 18%, transparent); color: var(--w-primary); }
    .note { margin: 0; font-size: 13px; }
    .dim { color: var(--w-dim); }
    .row { display: grid; grid-template-columns: auto minmax(0, 1fr) auto auto; gap: 12px; align-items: center;
      padding: 6px 0; border-top: 1px solid var(--w-glass-edge); }
    .row mat-icon { color: var(--w-primary); }
    .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .strains { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 14px; }
    .strain { padding: 16px; display: grid; gap: 12px; align-content: start; }
    .strain header { display: grid; gap: 2px; }
    .strain a { color: inherit; text-decoration: none; }
    .strain a:hover { color: var(--w-primary); }
    .strain .dim { font-size: 12px; }
    .more { justify-self: start; }
    .done { display: flex; align-items: center; gap: 8px; justify-content: center; margin: 0; font-size: 13px; }
    .done mat-icon { color: var(--w-good); }
    @include bp.phone {
      .pad { padding: 18px 16px; }
      .row { grid-template-columns: auto minmax(0, 1fr) auto; }
      .row .dim { grid-column: 2; grid-row: 2; }
      .row button { grid-row: 1 / 3; grid-column: 3; }
    }
  `,
})
export class Forgot {
  private readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  protected readonly todo = inject(Todo);
  protected readonly rateDays = RATE_DAYS;
  protected readonly strainDays = RATE_STRAIN_DAYS;
  /** Whether every strain waiting is shown, not only the latest three. */
  protected readonly allStrains = signal(false);
  protected readonly dimensions = computed(() => this.wits.effects.value()?.dimensions ?? []);
  /** Any purchase without a price, in storage or not: the walk can show the older ones too. */
  protected readonly unpricedAtAll = computed(() => (this.wits.costs.value()?.unpriced ?? 0) > 0);
  /** What has nothing waiting, said in a line so the page says it looked. */
  protected readonly done = computed(() => [
    ...(this.todo.rate().length ? [] : ['nothing to rate']),
    ...(this.todo.strains().length ? [] : ['every strain rated']),
    ...(this.unpricedAtAll() ? [] : ['every purchase priced']),
    ...(this.todo.stashes().length ? [] : ['no stash left behind']),
    ...(this.todo.avb().length ? [] : ['no AVB waiting']),
    ...(this.todo.terpenes().length ? [] : ['every terpene profile written down']),
  ]);

  /** A device's name, or what a session that named none is called. */
  protected deviceName(slug: string): string {
    return slug ? (this.wits.devices.value().find((d) => d.slug === slug)?.name ?? slug) : 'no device named';
  }

  protected weigh(product: string): void {
    openRecord(this.dialog, { tab: 3, product });
  }

  protected collect(device: string): void {
    openRecord(this.dialog, { tab: 4, device });
  }
}
