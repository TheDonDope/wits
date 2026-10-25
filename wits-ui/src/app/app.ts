import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, afterNextRender, computed, effect, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatSnackBarModule } from '@angular/material/snack-bar';
import { MatTabsModule } from '@angular/material/tabs';
import { MatTooltipModule } from '@angular/material/tooltip';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { filter, map } from 'rxjs';

import { Binding, Keys } from './data/keys';
import { ENGINE } from './data/mode';
import { THEMES, Theme } from './data/theme';
import { VERSION } from './data/version';
import { Wits } from './data/wits';
import { RecordDialog } from './pages/record';

/** The pages a phone's dock holds; the rest are under More. */
const DOCK = ['/', '/journal', '/jars'];

/** The record dialog's tabs, in order, as a link names them. */
const RECORD_TABS = ['grind', 'session', 'fill', 'weigh', 'avb'];

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
  protected readonly themeList = THEMES;
  protected readonly version = VERSION;
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);
  protected readonly now = signal(new Date());
  protected readonly links = [
    { path: '/', label: 'Dashboard', icon: 'space_dashboard' },
    { path: '/journal', label: 'Journal', icon: 'receipt_long' },
    { path: '/analysis', label: 'Analysis', icon: 'monitoring' },
    { path: '/effects', label: 'Effects', icon: 'psychology' },
    { path: '/jars', label: 'Jars', icon: 'inventory_2' },
    { path: '/history', label: 'History', icon: 'insights' },
    { path: '/costs', label: 'Costs', icon: 'euro' },
    // The phone keeps its own copy and syncs it by hand; a computer's server
    // opens whichever repository is wanted, and imports into it.
    ...(ENGINE
      ? [{ path: '/sync', label: 'Sync', icon: 'sync' }]
      : [{ path: '/repository', label: 'Repository', icon: 'folder_open' }]),
  ];
  /** A phone's dock and its More sheet: the same links, split for a thumb. */
  protected readonly dock = this.links.filter((l) => DOCK.includes(l.path));
  protected readonly more = this.links.filter((l) => !DOCK.includes(l.path));
  protected readonly moreOpen = signal(false);
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
    return this.links.find((l) => l.path !== '/' && path.startsWith(l.path)) ?? this.links[0];
  });
  protected readonly inMore = computed(() => this.more.includes(this.here()));

  constructor() {
    // Any way of arriving at a page — a tap, h/l, a number — puts More away.
    effect(() => {
      this.here();
      this.moreOpen.set(false);
    });
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
    const go = (i: number) => () => {
      void this.router.navigateByUrl(this.links[i].path);
    };
    const here = () => Math.max(this.links.findIndex((l) => l.path !== '/' && this.router.url.startsWith(l.path)), 0);
    const step = (d: number) => () => {
      go((here() + d + this.links.length) % this.links.length)();
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
      ...this.links.map((l, i) => ({ keys: [String(i + 1)], label: String(i + 1), help: l.label.toLowerCase(), run: go(i), quiet: true })),
      { keys: ['l'], label: 'h/l', help: 'pages', run: step(1) },
      { keys: ['h'], help: 'previous page', run: step(-1), quiet: true },
      { keys: ['t'], label: 't/T', help: 'theme', run: () => {
        this.theme.step(1);
      }, quiet: true },
      { keys: ['T'], help: 'previous theme', run: () => {
        this.theme.step(-1);
      }, quiet: true },
      { keys: ['Escape'], help: 'close', run: () => {
        this.moreOpen.set(false);
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

  protected record(tab = 0): void {
    this.moreOpen.set(false);
    this.dialog.open(RecordDialog, {
      autoFocus: 'first-tabbable',
      panelClass: 'record-panel',
      // Five tabs want more than Material's 560 px; a phone still gets the screen less a margin.
      maxWidth: 'min(660px, 94vw)',
      data: { tab: Math.max(tab, 0) },
    });
  }
}
