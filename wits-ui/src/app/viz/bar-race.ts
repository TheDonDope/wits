import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export interface RaceItem {
  key: string;
  label: string;
  value: number;
  note: string;
  color: string;
}

const ROW = 38;

/**
 * Products ranked by what was ground of them. Rows hold their place by key and
 * slide to their new rank, the bars grow rather than redraw — so a replay of
 * the record becomes a race between the strains, overtaking as it went.
 */
@Component({
  selector: 'w-bar-race',
  imports: [DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="race" [style.height.px]="shown().length * row">
      @for (it of shown(); track it.key; let i = $index) {
        <div class="row" [style.transform]="'translateY(' + i * row + 'px)'">
          <span class="rank num">{{ i + 1 }}</span>
          <span class="label" [title]="it.label">{{ it.label }}</span>
          <div class="track">
            <div class="fill" [style.width.%]="(it.value / most()) * 100" [style.background]="it.color"
              [style.box-shadow]="'0 0 16px ' + it.color"></div>
          </div>
          <span class="v num">{{ it.value | number: '1.1-1' }} g</span>
          <span class="note dim">{{ it.note }}</span>
        </div>
      }
    </div>
    @if (hidden() > 0) {
      <p class="dim more">and {{ hidden() }} more</p>
    }
  `,
  styles: `
    :host { display: block; }
    .race { position: relative; transition: height .4s ease; }
    .row { position: absolute; inset: 0 0 auto; height: 32px; display: grid;
      grid-template-columns: 26px minmax(120px, 260px) 1fr 70px minmax(0, 200px); gap: 12px; align-items: center;
      transition: transform .45s cubic-bezier(.2,.8,.2,1); }
    .rank { color: var(--w-dim); text-align: right; font-size: 13px; }
    .label { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-weight: 500; }
    .track { height: 12px; border-radius: 99px; background: rgb(255 255 255 / 5%); overflow: hidden; }
    .fill { height: 100%; border-radius: 99px; transition: width .45s cubic-bezier(.2,.8,.2,1); }
    .v { text-align: right; font-weight: 600; }
    .note { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .more { margin: 6px 0 0 38px; font-size: 12px; }
    @media (width <= 760px) { .row { grid-template-columns: 22px 1fr 1fr 64px; } .note { display: none; } }
  `,
})
export class BarRace {
  readonly items = input.required<RaceItem[]>();
  readonly limit = input(12);

  protected readonly row = ROW;
  protected readonly shown = computed(() => this.items().slice(0, this.limit()));
  protected readonly hidden = computed(() => Math.max(this.items().length - this.limit(), 0));
  protected readonly most = computed(() => Math.max(...this.shown().map((i) => i.value), 0.01));
}
