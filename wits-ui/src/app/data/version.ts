/**
 * The build the page was made from: `git describe` at the time, stamped by
 * `make ui` and `make pages` through `ng build --define`. A build without the
 * stamp (`ng serve`, the tests) says so.
 */
declare const WITS_VERSION: string | undefined;

export const VERSION = typeof WITS_VERSION === 'string' ? WITS_VERSION : 'unstamped';
