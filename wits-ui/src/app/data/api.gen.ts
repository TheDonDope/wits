// What the API answers in, written by `make types` from pkg/api (typescript.go).
// Do not edit: data/api-check.ts holds the models to these at compile time.

export interface ApiAccounts {
  storage: number;
  stash: number;
  consumed: number;
  avb: number;
}

export interface ApiDevice {
  slug: string;
  name: string;
  kind?: string;
  min_temp?: number;
  max_temp?: number;
  default_temp?: number;
  category: string;
  maker?: string;
  model?: string;
  country?: string;
  power?: string;
  signature?: string;
  use?: string;
  water?: boolean;
  modes: ApiMode[];
  auto_off?: number;
  load?: number;
  cents?: number;
  shop?: string;
  bought_on?: string;
  link?: string;
  notes?: string;
  retired?: boolean;
  sessions: number;
  grams: number;
  heated: number;
  last_used?: string;
}

export interface ApiDeviceFields {
  name: string;
  kind?: string;
  min_temp?: number;
  max_temp?: number;
  default_temp?: number;
  category: string;
  maker?: string;
  model?: string;
  country?: string;
  power?: string;
  signature?: string;
  use?: string;
  water?: boolean;
  modes: ApiMode[];
  auto_off?: number;
  load?: number;
  cents?: number;
  shop?: string;
  bought_on?: string;
  link?: string;
  notes?: string;
  retired?: boolean;
}

export interface ApiEntry {
  hash: string;
  type: string;
  occurred_at: string;
  recorded_at: string;
  product?: string;
  product_name?: string;
  grams: number;
  from: string;
  to: string;
  device?: string;
  temperature?: number;
  mode?: string;
  heater?: string;
  setting?: string;
  timer?: number;
  note?: string;
  reverts?: string;
  corrected?: boolean;
}

export interface ApiForgottenPlan {
  expected: number;
  short: number;
  entries: ApiEntry[];
  why?: string;
  sized_by?: string;
}

export interface ApiJar {
  slug: string;
  name: string;
  manufacturer?: string;
  cultivar?: string;
  thc?: number;
  cbd?: number;
  terpenes: string[];
  rating?: ApiStrainRating;
  storage: number;
  stash: number;
  avb: number;
  bought: number;
  ground: number;
  fills: number;
  last_seen: string;
  last_used?: string;
}

export interface ApiJournalSince {
  entries: ApiEntry[];
  head: string;
  reset: boolean;
  total: number;
  names: Record<string, string>;
}

export interface ApiPreview {
  entries: ApiEntry[];
  message: string;
  jars: ApiJarChange[];
  lines?: ApiPreviewLine[];
}

export interface ApiPricedPreview {
  lines: ApiPricedLine[];
  message: string;
}

export interface ApiWritten {
  entry: ApiEntry;
  message: string;
}

export interface ApiMode {
  name: string;
  signature?: string;
  temp?: number;
  timer?: number;
}

export interface ApiStrainRating {
  overall?: number;
  taste?: number;
  aroma?: number;
  again?: string;
  note?: string;
  at?: string;
}

export interface ApiJarChange {
  product: string;
  name: string;
  before: ApiAccounts;
  after: ApiAccounts;
}

export interface ApiPreviewLine {
  product: string;
  added: boolean;
  cents: number;
  vat_cents: number;
}

export interface ApiPricedLine {
  purchase: string;
  cents: number;
  vat_cents: number;
}
