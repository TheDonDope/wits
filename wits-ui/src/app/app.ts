import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, afterNextRender, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBarModule } from '@angular/material/snack-bar';
import { MatTabsModule } from '@angular/material/tabs';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

import { Binding, Keys } from './data/keys';
import { Wits } from './data/wits';
import { RecordDialog } from './pages/record';

/** The record dialog's tabs, in order, as a link names them. */
const RECORD_TABS = ['grind', 'session', 'fill', 'weigh'];

@Component({
  selector: 'app-root',
  imports: [
    DatePipe,
    MatButtonModule,
    MatIconModule,
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
  ];

  constructor() {
    const tick = setInterval(() => {
      this.now.set(new Date());
    }, 1000);
    inject(DestroyRef).onDestroy(() => {
      clearInterval(tick);
    });
    this.keys.register('Everywhere', () => this.global(), inject(DestroyRef));
    // ?record=grind (or session, fill, weigh) opens the form on that tab: a
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
      ...this.links.map((l, i) => ({ keys: [String(i + 1)], label: String(i + 1), help: l.label.toLowerCase(), run: go(i), quiet: true })),
      { keys: ['l'], label: 'h/l', help: 'pages', run: step(1) },
      { keys: ['h'], help: 'previous page', run: step(-1), quiet: true },
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
    this.dialog.open(RecordDialog, {
      autoFocus: 'first-tabbable',
      panelClass: 'record-panel',
      data: { tab: Math.max(tab, 0) },
    });
  }
}
