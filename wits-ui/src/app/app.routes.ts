import { Routes } from '@angular/router';

import { ENGINE } from './data/mode';
import { TABS } from './data/pages';

const shelf = TABS[2];
const insights = TABS[3];

export const routes: Routes = [
  { path: '', loadComponent: () => import('./pages/dashboard').then((m) => m.Dashboard), title: 'wits' },
  { path: 'forgot', loadComponent: () => import('./pages/forgot').then((m) => m.Forgot), title: 'wits · i forgot to' },
  {
    path: 'shelf',
    loadComponent: () => import('./pages/section').then((m) => m.Section),
    data: { pages: shelf.pages, label: shelf.label },
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'jars' },
      { path: 'jars', loadComponent: () => import('./pages/jars').then((m) => m.Jars), title: 'wits · jars' },
      { path: 'journal', loadComponent: () => import('./pages/journal').then((m) => m.Journal), title: 'wits · journal' },
    ],
  },
  {
    path: 'insights',
    loadComponent: () => import('./pages/section').then((m) => m.Section),
    data: { pages: insights.pages, label: insights.label },
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'analysis' },
      { path: 'analysis', loadComponent: () => import('./pages/analysis').then((m) => m.Analysis), title: 'wits · analysis' },
      { path: 'effects', loadComponent: () => import('./pages/effects').then((m) => m.Effects), title: 'wits · effects' },
      { path: 'history', loadComponent: () => import('./pages/history').then((m) => m.History), title: 'wits · history' },
      { path: 'costs', loadComponent: () => import('./pages/costs').then((m) => m.Costs), title: 'wits · costs' },
    ],
  },
  // Which repository, and importing a workbook: the computer's folders, or the phone's own. Off the header's menu.
  { path: 'repository', loadComponent: () => import('./pages/repository').then((m) => m.Repository), title: 'wits · repository' },
  // The phone keeps its own copy and syncs it by hand; the server has no need.
  ...(ENGINE ? [{ path: 'sync', loadComponent: () => import('./pages/sync').then((m) => m.Sync), title: 'wits · sync' }] : []),
  // Where the pages were before the tabs were bundled, for a bookmark.
  { path: 'jars', redirectTo: 'shelf/jars' },
  { path: 'journal', redirectTo: 'shelf/journal' },
  { path: 'analysis', redirectTo: 'insights/analysis' },
  { path: 'effects', redirectTo: 'insights/effects' },
  { path: 'history', redirectTo: 'insights/history' },
  { path: 'costs', redirectTo: 'insights/costs' },
  { path: '**', redirectTo: '' },
];
