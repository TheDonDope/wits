import { ChangeDetectionStrategy, Component, input, model } from '@angular/core';

/**
 * Five dots to tap, 1 to 5: how much a session did for one thing. Tapping the
 * score already chosen clears it — a dimension not rated is left out, not
 * rated low.
 */
@Component({
  selector: 'w-score-dots',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="dots" role="radiogroup" [attr.aria-label]="label()">
      @for (n of scale; track n) {
        <button type="button" role="radio" [attr.aria-checked]="value() === n"
          [attr.aria-label]="label() + ' ' + n" [class.on]="(value() ?? 0) >= n"
          [style.--i]="n" (click)="value.set(value() === n ? null : n)"></button>
      }
    </div>
  `,
  styles: `
    .dots { display: flex; gap: 6px; }
    button {
      width: 18px; height: 18px; border-radius: 50%; padding: 0; cursor: pointer;
      border: 1.5px solid rgb(255 255 255 / 18%); background: transparent;
      transition: transform .15s, background .2s, box-shadow .2s;
    }
    button:hover { transform: scale(1.18); border-color: var(--tone, var(--w-violet)); }
    button.on {
      border-color: transparent;
      background: var(--tone, var(--w-violet));
      box-shadow: 0 0 10px var(--tone, var(--w-violet));
      opacity: calc(.45 + var(--i) * .11);
    }
  `,
})
export class ScoreDots {
  readonly value = model<number | null>(null);
  readonly label = input('score');
  protected readonly scale = [1, 2, 3, 4, 5];
}
