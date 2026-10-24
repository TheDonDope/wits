import { Routes } from '@angular/router';

import { ENGINE } from './data/mode';

export const routes: Routes = [
  { path: '', loadComponent: () => import('./pages/dashboard').then((m) => m.Dashboard), title: 'wits' },
  { path: 'journal', loadComponent: () => import('./pages/journal').then((m) => m.Journal), title: 'wits · journal' },
  { path: 'analysis', loadComponent: () => import('./pages/analysis').then((m) => m.Analysis), title: 'wits · analysis' },
  { path: 'effects', loadComponent: () => import('./pages/effects').then((m) => m.Effects), title: 'wits · effects' },
  { path: 'jars', loadComponent: () => import('./pages/jars').then((m) => m.Jars), title: 'wits · jars' },
  { path: 'history', loadComponent: () => import('./pages/history').then((m) => m.History), title: 'wits · history' },
  { path: 'costs', loadComponent: () => import('./pages/costs').then((m) => m.Costs), title: 'wits · costs' },
  // The phone keeps its own copy and syncs it by hand; the server has no need.
  ...(ENGINE ? [{ path: 'sync', loadComponent: () => import('./pages/sync').then((m) => m.Sync), title: 'wits · sync' }] : []),
  { path: '**', redirectTo: '' },
];
