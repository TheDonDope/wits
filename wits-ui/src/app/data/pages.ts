import { Subpage } from '../pages/section';

/** A tab of the shell, and the pages it holds when it holds several. */
export interface Tab extends Subpage {
  pages?: Subpage[];
}

/**
 * The four tabs and what they hold: what is happening, what was left undone,
 * what is on hand, and what it all comes to. Routes, the tab bar, the dock,
 * the keys and the phone's top bar all read this one list.
 */
export const TABS: readonly Tab[] = [
  { path: '/', label: 'Dashboard', icon: 'space_dashboard' },
  { path: '/forgot', label: 'I forgot to', icon: 'checklist' },
  { path: '/shelf', label: 'Shelf', icon: 'inventory_2', pages: [
    { path: 'jars', label: 'Jars', icon: 'inventory_2' },
    { path: 'journal', label: 'Journal', icon: 'receipt_long' },
  ] },
  { path: '/insights', label: 'Insights', icon: 'insights', pages: [
    { path: 'analysis', label: 'Analysis', icon: 'monitoring' },
    { path: 'effects', label: 'Effects', icon: 'psychology' },
    { path: 'history', label: 'History', icon: 'timeline' },
    { path: 'costs', label: 'Costs', icon: 'euro' },
  ] },
];

/** Every page in order, a tab's own pages in its place: what h and l step through. */
export const PAGES: readonly Subpage[] = TABS.flatMap((t) =>
  t.pages ? t.pages.map((p) => ({ ...p, path: `${t.path}/${p.path}` })) : [t],
);
