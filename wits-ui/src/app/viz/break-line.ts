import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

const MONTHS = ['J', 'F', 'M', 'A', 'M', 'J', 'J', 'A', 'S', 'O', 'N', 'D'];

/**
 * The year as one line, January to December, the days with grinding or a
 * session marked on it and the longest break opened out between them.
 */
@Component({
  selector: 'w-break-line',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="line">
      @for (d of marks(); track d) { <i class="day" [style.left.%]="d"></i> }
      <span class="gap" [style.left.%]="gap()[0]" [style.width.%]="gap()[1] - gap()[0]"></span>
      @for (m of months; track $index) {
        <span class="m" [style.left.%]="($index / 12) * 100 + 100 / 24">{{ m }}</span>
      }
    </div>
  `,
  styles: `
    :host { display: block; }
    .line { position: relative; height: 54px; }
    .line::before { content: ''; position: absolute; left: 0; right: 0; top: 20px; height: 2px; border-radius: 1px;
      background: color-mix(in srgb, var(--w-fg) 12%, transparent); }
    .day { position: absolute; top: 12px; width: 2px; height: 18px; margin-left: -1px; border-radius: 1px;
      background: color-mix(in srgb, var(--w-stash) 45%, transparent); }
    .gap { position: absolute; top: 6px; height: 30px; border-radius: 8px; animation: open 1s cubic-bezier(.2,.8,.2,1) both;
      background: repeating-linear-gradient(135deg, color-mix(in srgb, var(--w-avb) 22%, transparent) 0 6px, transparent 6px 12px);
      border: 1px solid color-mix(in srgb, var(--w-avb) 60%, transparent); transform-origin: left; }
    .m { position: absolute; bottom: 0; transform: translateX(-50%); font: 10px var(--w-num); color: var(--w-dim); }
    @keyframes open { from { transform: scaleX(0); } }
  `,
})
export class BreakLine {
  readonly year = input.required<number>();
  /** YYYY-MM-DD: the days with anything, and the break's first and last. */
  readonly days = input.required<string[]>();
  readonly from = input.required<string>();
  readonly to = input.required<string>();
  protected readonly months = MONTHS;

  private readonly place = computed(() => {
    const start = Date.UTC(this.year(), 0, 1);
    const span = Date.UTC(this.year() + 1, 0, 1) - start;
    return (day: string, end = false) => {
      const [y, m, d] = day.split('-').map(Number);
      return ((Date.UTC(y, m - 1, d + (end ? 1 : 0)) - start) / span) * 100;
    };
  });

  protected readonly marks = computed(() => this.days().map((d) => this.place()(d) + 100 / 732));
  protected readonly gap = computed(() => [this.place()(this.from()), this.place()(this.to(), true)]);
}
