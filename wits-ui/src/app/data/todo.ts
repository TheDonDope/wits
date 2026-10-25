import { Injectable, computed, inject } from '@angular/core';

import { Wits } from './wits';

/** How long after a session a rating is still asked for: past it, nobody remembers. */
export const RATE_DAYS = 7;

/**
 * What was forgotten, as the "I forgot to" page lists it and its tab counts
 * it: each a thing the ledger can show was left undone, and none that it
 * cannot. A weighing that agrees with the ledger writes nothing, so a jar
 * not weighed lately is offered there but never counted.
 */
@Injectable({ providedIn: 'root' })
export class Todo {
  private readonly wits = inject(Wits);

  /** Sessions of the last week without a rating, newest first. */
  readonly rate = computed(() => {
    const since = Date.now() - RATE_DAYS * 86_400_000;
    return (this.wits.effects.value()?.unrated ?? []).filter((s) => new Date(s.occurred_at).getTime() >= since);
  });
  /** The pickups still in storage with a purchase not priced. */
  readonly pickups = computed(() => new Set(this.wits.toPrice.value().map((e) => e.fill)).size);
  /** Stashes still holding grams of a jar with nothing sealed left: sessions never logged. */
  readonly stashes = computed(() => this.wits.jars.value().filter((j) => j.storage <= 0 && j.stash > 0));
  /** Devices whose sessions since they were last emptied hold AVB not yet weighed out. */
  readonly avb = computed(() => this.wits.avb.value()?.waiting ?? []);
  /** Jars still sealed, to weigh against the scale now and then. Offered, not counted. */
  readonly sealed = computed(() => this.wits.jars.value().filter((j) => j.storage > 0));

  /**
   * Every jar ever dispensed with no terpene profile written down, those
   * still sealed first, then the most lately used. Offered, not counted: a profile no page names stays
   * missing, and would hold the tab's count up for good.
   */
  readonly terpenes = computed(() => {
    const missing = this.wits.jars.value().filter((j) => j.bought > 0 && !j.terpenes.length);
    const sealed = (j: { storage: number }) => (j.storage > 0 ? 0 : 1);
    return missing.sort((a, b) => sealed(a) - sealed(b) || b.last_seen.localeCompare(a.last_seen));
  });

  /** Everything counted, for the tab. */
  readonly count = computed(() => this.rate().length + this.pickups() + this.stashes().length + this.avb().length);
}
