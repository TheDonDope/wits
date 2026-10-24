import { DatePipe, DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatIconModule } from '@angular/material/icon';

import { Jar } from '../data/models';
import { Wits } from '../data/wits';
import { GlassJar } from '../viz/glass-jar';

/** Every product, one card each, in the jar's scope: its whole life. */
@Component({
  selector: 'w-jars',
  imports: [DatePipe, DecimalPipe, GlassJar, MatButtonToggleModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="glass head">
      <div>
        <span class="eyebrow">the jars <span class="scope">each across its life</span></span>
        <h1 class="num">{{ list().length }} <span class="dim">{{ which() === 'shelf' ? 'on the shelf' : 'finished' }}</span></h1>
      </div>
      <mat-button-toggle-group [value]="which()" (change)="which.set($event.value)" aria-label="Which jars">
        <mat-button-toggle value="shelf">On the shelf</mat-button-toggle>
        <mat-button-toggle value="finished">Finished</mat-button-toggle>
      </mat-button-toggle-group>
    </section>

    <section class="grid">
      @for (j of list(); track j.slug; let i = $index) {
        <article class="card glass" [style.animation-delay.ms]="i * 40">
          <w-jar [capacity]="j.bought"
            [layers]="[
              { grams: j.storage, color: 'var(--w-storage)', label: 'sealed' },
              { grams: j.stash, color: 'var(--w-stash)', label: 'in the stash' },
            ]" />
          <div class="body">
            <h3>{{ j.name }}</h3>
            <div class="chips">
              @if (j.thc) { <span class="chip thc">THC {{ j.thc }}%</span> }
              @if (j.cbd) { <span class="chip cbd">CBD {{ j.cbd }}%</span> }
              <span class="chip">{{ j.fills }} {{ j.fills === 1 ? 'fill' : 'fills' }}</span>
            </div>
            <dl>
              <div><dt>sealed</dt><dd class="num" style="color: var(--w-storage)">{{ j.storage | number: '1.2-2' }}</dd></div>
              <div><dt>stash</dt><dd class="num" style="color: var(--w-stash)">{{ j.stash | number: '1.2-2' }}</dd></div>
              <div><dt>ground</dt><dd class="num">{{ j.ground | number: '1.0-0' }}</dd></div>
              <div><dt>dispensed</dt><dd class="num">{{ j.bought | number: '1.0-0' }}</dd></div>
            </dl>
            <span class="dim seen"><mat-icon>schedule</mat-icon>last {{ j.last_seen | date: 'd MMM y' }}</span>
          </div>
        </article>
      } @empty {
        <p class="dim">Nothing here.</p>
      }
    </section>
  `,
  styles: `
    :host { display: grid; gap: 20px; }
    .head { padding: 18px 24px; display: flex; flex-wrap: wrap; gap: 18px; align-items: center; justify-content: space-between; }
    h1 { font: 600 28px var(--w-num); margin: 4px 0 0; }
    h1 .dim { font: 400 15px 'Inter Variable'; }
    .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 18px; }
    .card { display: grid; grid-template-columns: 96px 1fr; gap: 16px; padding: 18px; opacity: 0;
      animation: rise .6s cubic-bezier(.2,.8,.2,1) forwards; transition: transform .25s, border-color .25s; }
    .card:hover { transform: translateY(-3px); border-color: rgba(139,123,255,.35); }
    w-jar { width: 96px; height: 134px; }
    .body { display: grid; gap: 8px; align-content: start; min-width: 0; }
    h3 { margin: 0; font: 600 15px/1.3 'Inter Variable'; }
    .chips { display: flex; gap: 6px; flex-wrap: wrap; }
    .chip { font: 600 10px/1.8 'Inter Variable'; padding: 0 8px; border-radius: 99px; background: rgba(255,255,255,.06); color: var(--w-dim); }
    .chip.thc { background: rgba(124,245,196,.12); color: var(--w-mint); }
    .chip.cbd { background: rgba(110,168,255,.14); color: var(--w-storage); }
    dl { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 14px; margin: 4px 0 0; }
    dl div { display: grid; gap: 1px; min-width: 0; }
    dt { font: 600 9px 'Inter Variable'; letter-spacing: .1em; text-transform: uppercase; color: var(--w-dim); }
    dd { margin: 0; font-size: 15px; font-weight: 600; }
    .seen { display: flex; gap: 4px; align-items: center; font-size: 12px; }
    .seen mat-icon { font-size: 14px; width: 14px; height: 14px; }
    @keyframes rise { from { opacity: 0; transform: translateY(14px); } to { opacity: 1; transform: none; } }
  `,
})
export class Jars {
  private readonly wits = inject(Wits);
  protected readonly which = signal<'shelf' | 'finished'>('shelf');

  protected readonly list = computed<Jar[]>(() => {
    const held = (j: Jar) => j.storage + j.stash > 0;
    return this.wits.jars.value().filter((j) => (this.which() === 'shelf' ? held(j) : !held(j)));
  });
}
