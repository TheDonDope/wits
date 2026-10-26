import { Router } from '@angular/router';
import { MatSnackBar } from '@angular/material/snack-bar';

import { Wits } from './wits';

/**
 * Opens the demo — the workbook with its invoices' prices, every device, a
 * year of rated sessions and every jar's terpenes — in a repository of its
 * own, leaving yours as it is; the Repository page goes back to it. The menu
 * and the welcome both open it this way.
 */
export async function openDemo(wits: Wits, router: Router, snack: MatSnackBar): Promise<void> {
  snack.open('Opening the demo…');
  try {
    const place = await wits.demo();
    await router.navigateByUrl('/');
    snack.open(`The demo is open, in ${place.path}: ${String(place.entries)} entries. Repository goes back to yours.`, 'OK',
      { duration: 8000 });
  } catch (e) {
    snack.open((e as Error).message, 'OK', { duration: 8000 });
  }
}
