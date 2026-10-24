import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', loadComponent: () => import('./pages/dashboard').then((m) => m.Dashboard), title: 'wits' },
  { path: 'journal', loadComponent: () => import('./pages/journal').then((m) => m.Journal), title: 'wits · journal' },
  { path: 'jars', loadComponent: () => import('./pages/jars').then((m) => m.Jars), title: 'wits · jars' },
  { path: 'history', loadComponent: () => import('./pages/history').then((m) => m.History), title: 'wits · history' },
  { path: '**', redirectTo: '' },
];
