import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { MatTabsModule } from '@angular/material/tabs';
import { ActivatedRoute, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

/** One page of a tab that holds several, as its route names it. */
export interface Subpage {
  path: string;
  label: string;
  icon: string;
}

/**
 * A tab that holds several pages — Shelf, Insights — with a row of its own
 * to pick among them, and the page picked below it. The pages are the
 * route's `pages`, so the routes say once what each tab holds.
 */
@Component({
  selector: 'w-section',
  imports: [MatIconModule, MatTabsModule, RouterLink, RouterLinkActive, RouterOutlet],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <nav mat-tab-nav-bar [tabPanel]="panel" mat-stretch-tabs="false" class="sub" [attr.aria-label]="label">
      @for (p of pages; track p.path) {
        <a mat-tab-link [routerLink]="p.path" routerLinkActive #rla="routerLinkActive" [active]="rla.isActive">
          <mat-icon>{{ p.icon }}</mat-icon><span>{{ p.label }}</span>
        </a>
      }
    </nav>
    <mat-tab-nav-panel #panel><router-outlet /></mat-tab-nav-panel>
  `,
  styles: `
    @use 'breakpoints' as bp;
    :host { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .sub { background: transparent; --mat-tab-header-divider-height: 0; }
    .sub a { min-width: 0; padding: 0 16px; }
    .sub mat-icon { margin-right: 8px; }
    @include bp.phone {
      .sub a { flex: 1; padding: 0 6px; }
      /* Five pages to a tab want the width their icons took: the labels alone fit 375 px, unpaged. */
      .sub mat-icon { display: none; }
      .sub span { font-size: 13px; }
    }
  `,
})
export class Section {
  private readonly route = inject(ActivatedRoute);
  protected readonly pages = (this.route.snapshot.data['pages'] ?? []) as Subpage[];
  protected readonly label = this.route.snapshot.data['label'] as string;
}
