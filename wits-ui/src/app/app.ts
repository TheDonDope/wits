import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, afterNextRender, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { MatTabsModule } from '@angular/material/tabs';
import { MatTooltipModule } from '@angular/material/tooltip';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { filter, map } from 'rxjs';

import { openDemo } from './data/demo';
import { Binding, Keys } from './data/keys';
import { ENGINE } from './data/mode';
import { PAGES, TABS } from './data/pages';
import { THEMES, Theme } from './data/theme';
import { Todo } from './data/todo';
import { VERSION } from './data/version';
import { Wits } from './data/wits';
import { RECORD_TABS, openRecord } from './pages/record';

/** The pages off the header's menu: rarely wanted, so no tab of their own. */
const ASIDE = [
  // Every page explained, and the commands that do the same in a terminal.
  { path: '/guide', label: 'User guide', icon: 'menu_book' },
  // Both open whichever repository is wanted, and import into it; the
  // phone keeps its own copies, and syncs them by hand.
  { path: '/repository', label: 'Repository', icon: 'folder_open' },
  ...(ENGINE ? [{ path: '/sync', label: 'Sync', icon: 'sync' }] : []),
];

@Component({
  selector: 'app-root',
  imports: [
    DatePipe,
    MatButtonModule,
    MatIconModule,
    MatMenuModule,
    MatSnackBarModule,
    MatTabsModule,
    MatTooltipModule,
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
  ],
  host: { '(window:keydown)': 'keys.handle($event)' },
  templateUrl: './app.html',
  styleUrl: './app.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class App {
  protected readonly wits = inject(Wits);
  protected readonly keys = inject(Keys);
  protected readonly theme = inject(Theme);
  protected readonly todo = inject(Todo);
  protected readonly themeList = THEMES;
  protected readonly version = VERSION;
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);
  private readonly snack = inject(MatSnackBar);
  protected readonly now = signal(new Date());
  /** Whether the demo is being opened: restoring it takes a moment, on a phone a few. */
  protected readonly seeding = signal(false);
  /** The four tabs; a phone's dock holds the same four, Record raised between them. */
  protected readonly links = TABS;
  protected readonly aside = ASIDE;
  private readonly url = toSignal(
    this.router.events.pipe(
      filter((e) => e instanceof NavigationEnd),
      map((e) => e.urlAfterRedirects),
    ),
    { initialValue: this.router.url },
  );
  /** The page showing, which a phone names in its top bar for want of tabs. */
  protected readonly here = computed(() => {
    const path = this.url().split('?')[0].split('#')[0];
    return [...PAGES, ...ASIDE].find((l) => l.path !== '/' && path.startsWith(l.path)) ?? PAGES[0];
  });

  /**
   * Opens the demo — the workbook with its invoices' prices, every device, a
   * year of rated sessions and every jar's terpenes — in a repository of its
   * own, leaving yours as it is; the Repository page goes back to it.
   */
  protected async demo(): Promise<void> {
    this.seeding.set(true);
    try {
      await openDemo(this.wits, this.router, this.snack);
    } finally {
      this.seeding.set(false);
    }
  }

  constructor() {
    const tick = setInterval(() => {
      this.now.set(new Date());
    }, 1000);
    inject(DestroyRef).onDestroy(() => {
      clearInterval(tick);
    });
    this.keys.register('Everywhere', () => this.global(), inject(DestroyRef));
    // ?record=grind (or session, fill, weigh, avb) opens the form on that tab: a
    // link to bookmark for the thing done most often.
    afterNextRender(() => {
      const tab = new URLSearchParams(location.search).get('record');
      if (tab !== null) {
        this.record(RECORD_TABS.indexOf(tab));
      }
    });
  }

  /** The keys every page answers — the terminal's, where a browser allows. */
  private global(): Binding[] {
    const go = (path: string) => () => {
      void this.router.navigateByUrl(path);
    };
    // h and l step through every page, a tab's own pages in turn.
    const here = () => Math.max(PAGES.findIndex((l) => l.path !== '/' && this.router.url.startsWith(l.path)), 0);
    const step = (d: number) => () => {
      go(PAGES[(here() + d + PAGES.length) % PAGES.length].path)();
    };
    const open = (tab: number) => () => {
      this.record(tab);
    };
    return [
      { keys: ['g'], help: 'grind', run: open(0) },
      { keys: ['s'], help: 'session', run: open(1) },
      { keys: ['b'], help: 'fill', run: open(2) },
      { keys: ['r'], help: 'weigh', run: open(3) },
      { keys: ['v'], help: 'AVB', run: open(4) },
      ...this.links.map((l, i) => ({ keys: [String(i + 1)], label: String(i + 1), help: l.label.toLowerCase(), run: go(l.path), quiet: true })),
      { keys: ['l'], label: 'h/l', help: 'pages', run: step(1) },
      { keys: ['h'], help: 'previous page', run: step(-1), quiet: true },
      { keys: ['t'], label: 't/T', help: 'theme', run: () => {
        this.theme.step(1);
      }, quiet: true },
      { keys: ['T'], help: 'previous theme', run: () => {
        this.theme.step(-1);
      }, quiet: true },
      { keys: ['?'], help: 'all keys', run: () => {
        this.keys.showAll.update((v) => !v);
      } },
    ];
  }

  /** How a key is written in the help. */
  protected show(key: string): string {
    return ({ ' ': 'space', ArrowLeft: '←', ArrowRight: '→', ArrowUp: '↑', ArrowDown: '↓', Escape: 'esc' } as Record<string, string>)[key] ?? key;
  }

  /**
   * Opens Record: on the tab asked for, or on Fill while there is nothing
   * sealed to grind — the first thing an empty shelf wants — and on Grind
   * once there is.
   */
  protected record(tab?: number): void {
    const empty = this.wits.overview.value()?.fill === null;
    openRecord(this.dialog, { tab: tab ?? (empty ? 2 : 0) });
  }
}
