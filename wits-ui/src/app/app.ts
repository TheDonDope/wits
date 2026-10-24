import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, afterNextRender, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBarModule } from '@angular/material/snack-bar';
import { MatTabsModule } from '@angular/material/tabs';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

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
  templateUrl: './app.html',
  styleUrl: './app.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class App {
  protected readonly wits = inject(Wits);
  private readonly dialog = inject(MatDialog);
  protected readonly now = signal(new Date());
  protected readonly links = [
    { path: '/', label: 'Dashboard', icon: 'space_dashboard' },
    { path: '/journal', label: 'Journal', icon: 'receipt_long' },
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
    // ?record=grind (or session, fill, weigh) opens the form on that tab: a
    // link to bookmark for the thing done most often.
    afterNextRender(() => {
      const tab = new URLSearchParams(location.search).get('record');
      if (tab !== null) {
        this.record(RECORD_TABS.indexOf(tab));
      }
    });
  }

  protected record(tab = 0): void {
    this.dialog.open(RecordDialog, {
      autoFocus: 'first-tabbable',
      panelClass: 'record-panel',
      data: { tab: Math.max(tab, 0) },
    });
  }
}
