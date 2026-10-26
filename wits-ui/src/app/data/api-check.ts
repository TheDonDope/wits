import type {
  ApiAccounts, ApiDevice, ApiDeviceFields, ApiEntry, ApiForgottenPlan, ApiJar, ApiJournalSince, ApiPreview, ApiPricedPreview,
  ApiWritten,
} from './api.gen';
import type { Since } from './entries';
import type { Accounts, Device, DeviceFields, Entry, ForgottenPlan, Jar, Preview, PricedPreview, Written } from './models';

/**
 * Whether two shapes have the same fields, by name: a field the server sends
 * that a model lacks, or one a model expects that the server never sends,
 * makes it false. The models keep their own, narrower types — a union where
 * the server says string — and their comments; their fields are held here to
 * what pkg/api answers in (api.gen.ts, written by `make types`), so the two
 * cannot drift apart without the build saying so.
 */
type SameFields<A, B> = [Exclude<keyof A, keyof B>, Exclude<keyof B, keyof A>] extends [never, never] ? true : false;

/** Each model, held to the server's shape: a false here does not compile. */
export const CHECKED = {
  entry: true satisfies SameFields<Entry, ApiEntry>,
  jar: true satisfies SameFields<Jar, ApiJar>,
  device: true satisfies SameFields<Device, ApiDevice>,
  deviceFields: true satisfies SameFields<DeviceFields, ApiDeviceFields>,
  written: true satisfies SameFields<Written, ApiWritten>,
  preview: true satisfies SameFields<Preview, ApiPreview>,
  accounts: true satisfies SameFields<Accounts, ApiAccounts>,
  forgottenPlan: true satisfies SameFields<ForgottenPlan, ApiForgottenPlan>,
  pricedPreview: true satisfies SameFields<PricedPreview, ApiPricedPreview>,
  journalSince: true satisfies SameFields<Since, ApiJournalSince>,
};
