import { DatePipe, DecimalPipe, NgTemplateOutlet } from '@angular/common';
import { HttpErrorResponse, httpResource } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, computed, effect, inject, signal, untracked, viewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialog, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTabsModule } from '@angular/material/tabs';

import { Device, DeviceMode, Dimension, Entry, ForgottenPlan, Jar, Preview, Written } from '../data/models';
import { CATEGORIES, heats, needsHeat, sessionWith } from '../data/devices';
import { Choice, ChoiceInput } from '../inputs/choice';
import { FlowLine, FlowPreview, flowOf } from '../inputs/flow';
import { GramsInput } from '../inputs/grams';
import { JarOption, JarPick } from '../inputs/jar-pick';
import { ScaleInput } from '../inputs/scale';
import { Boils, DialMark, TempDial } from '../inputs/temp-dial';
import { WhenInput, isoDay } from '../inputs/when';
import { Wits } from '../data/wits';
import { ScoreDots } from '../viz/score-dots';
import { FillForm } from './fill-form';
import { ASKS } from './rate-queue';

/** What the dialog opens on: a tab, and the jar or the device already picked. */
export interface RecordData {
  tab?: number;
  product?: string;
  device?: string;
  /** The account a weighing starts on: storage, stash or avb. */
  account?: string;
}

/** The record dialog's tabs, in order, as a link names them. */
export const RECORD_TABS = ['grind', 'session', 'fill', 'weigh', 'avb'];

/** Opens the record dialog: on the shell's Record, a key, a link, or a thing forgotten. */
export function openRecord(dialog: MatDialog, data: RecordData = {}): void {
  dialog.open(RecordDialog, {
    autoFocus: 'first-tabbable',
    panelClass: 'record-panel',
    // Five tabs want more than Material's 560 px; a phone still gets the screen less a margin.
    maxWidth: 'min(660px, 94vw)',
    data: { ...data, tab: Math.max(data.tab ?? 0, 0) },
  });
}

/**
 * Records an entry: a grind, a session, a fill, a weighing, or AVB collected or used. Every check is
 * the recorder's, on the server — what this form refuses is what the command
 * line would refuse, said in the same words. A fill, its strains, its prices
 * and the invoices and labels it is read off, is a form of its own (FillForm).
 */
@Component({
  selector: 'w-record',
  imports: [
    ChoiceInput,
    DatePipe,
    DecimalPipe,
    FillForm,
    FlowPreview,
    FormsModule,
    GramsInput,
    JarPick,
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatTabsModule,
    NgTemplateOutlet,
    ScaleInput,
    ScoreDots,
    TempDial,
    WhenInput,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (rating(); as r) {
      <h2 mat-dialog-title>How was it?</h2>
      <mat-dialog-content>
        <p class="dim said">{{ r.message }}</p>
        <div class="dims">
          @for (d of asked(); track d) {
            <span class="ask" [class.lead]="d === 'overall'">{{ asks[d] }}</span>
            <w-score-dots [label]="asks[d]" [value]="scores()[d] ?? null" (valueChange)="score(d, $event)"
              [style.--tone]="d === 'overall' ? 'var(--w-accent)' : 'var(--w-primary)'" />
          }
        </div>
        @if (!allDims()) {
          <button mat-button class="more" (click)="allDims.set(true)"><mat-icon>expand_more</mat-icon>Pain, sleep, calm and more</button>
        }
        <p class="dim">Or later, under I forgot to — a session waits there a week.</p>
      </mat-dialog-content>
      <mat-dialog-actions align="end">
        <button mat-button (click)="ref.close(r)">Later</button>
        <button mat-flat-button (click)="rate(r)" [disabled]="busy() || !rated()">Save</button>
      </mat-dialog-actions>
    } @else {
    <h2 mat-dialog-title>Record</h2>
    <mat-dialog-content>
      <mat-tab-group [selectedIndex]="tab()" (selectedIndexChange)="tab.set($event); error.set('')" animationDuration="200ms">
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>grain</mat-icon><span>Grind</span></span></ng-template>
          <div class="form">
            <w-jar-pick label="from the jar" empty="Nothing sealed on the shelf." [jars]="grindJars()" [value]="product()"
              (valueChange)="product.set($event)" />
            <div class="duo">
              <w-grams label="ground" [max]="held()" [lately]="recent()" [offerAll]="true" [value]="grams()" (valueChange)="grams.set($event)" />
              <w-when [value]="date()" (valueChange)="date.set($event)" />
            </div>
            <ng-container *ngTemplateOutlet="rehearsed" />
          </div>
        </mat-tab>
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>air</mat-icon><span>Session</span></span></ng-template>
          <div class="form">
            <w-jar-pick label="from what is ground" empty="Nothing ground yet: grind first." [jars]="seshJars()" [value]="product()"
              (valueChange)="product.set($event)" />
            <w-choice label="had with" [tiles]="true" [fold]="4" [options]="deviceChoices()" [value]="device()" (valueChange)="pickDevice($event)" />
            @if (picked()?.modes?.length) {
              <w-choice label="mode" [options]="modeChoices()" [value]="mode()" (valueChange)="pickMode($event)" />
            }
            @if (picked() && heatedBy(picked()!)) {
              <w-choice label="heated with" [options]="heaterChoices()" [value]="heater()" (valueChange)="pickHeater($event)" />
              @if (heaterPicked(); as h) {
                @if (h.modes.length) {
                  <w-choice label="setting" [options]="settingChoices()" [value]="setting()" (valueChange)="setting.set($event)" />
                } @else {
                  <mat-form-field appearance="outline" subscriptSizing="dynamic">
                    <mat-label>Setting</mat-label>
                    <input matInput [ngModel]="setting()" (ngModelChange)="setting.set($event)" placeholder="45 W, 40 s" />
                  </mat-form-field>
                }
              }
            }
            <div class="duo" [class.solo]="!range()">
              @if (range(); as r) {
                <w-temp-dial [min]="r.min" [max]="r.max" [marks]="dialMarks()" [terpenes]="boils()" [value]="temp()" (valueChange)="temp.set($event)" />
              }
              <div class="stack">
                <w-grams label="had" [max]="held()" [lately]="recent()" [value]="grams()" (valueChange)="grams.set($event)" />
                @if (timed()) {
                  <mat-form-field appearance="outline" subscriptSizing="dynamic" class="timer">
                    <mat-label>Session timer</mat-label>
                    <input matInput type="number" min="0" [ngModel]="timer()" (ngModelChange)="timer.set($event)" />
                    <span matTextSuffix>s</span>
                  </mat-form-field>
                }
                <w-when [value]="date()" (valueChange)="date.set($event)" />
              </div>
            </div>
            <ng-container *ngTemplateOutlet="rehearsed" />
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Note</mat-label>
              <textarea matInput rows="2" [ngModel]="note()" (ngModelChange)="note.set($event)"
                placeholder="How it tasted, what it was for"></textarea>
            </mat-form-field>
          </div>
        </mat-tab>
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>local_pharmacy</mat-icon><span>Prescription</span></span></ng-template>
          <w-fill-form class="form" [(date)]="date" [(error)]="error" />
        </mat-tab>
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>scale</mat-icon><span>Weigh</span></span></ng-template>
          <div class="form">
            <w-choice label="on the scale" [options]="accounts" [value]="account()" (valueChange)="account.set($event)" />
            <w-jar-pick label="the jar" empty="Nothing on the shelf to weigh." [jars]="weighJars()" [value]="product()"
              (valueChange)="product.set($event)" />
            @if (product()) {
              <w-scale [expected]="expected()" [value]="grams()" (valueChange)="grams.set($event)" />
            }
            @if (weighed()) {
              <p class="said weighed"><mat-icon>check_circle</mat-icon>{{ weighed() }}</p>
            }
            @if (why()) {
              <p class="dim why"><mat-icon>info</mat-icon>{{ why() }}</p>
            }
            @if (plan(); as p) {
              <div class="forgot" role="radiogroup" [attr.aria-label]="(p.short | number: '1.2-2') + ' g less than the ledger says'">
                <span class="eyebrow">{{ p.short | number: '1.2-2' }} g less than the ledger says</span>
                <button type="button" role="radio" class="pick" [class.on]="!lost()" [attr.aria-checked]="!lost()" (click)="lost.set(false)">
                  <mat-icon>{{ p.entries[0].type === 'grind' ? 'grain' : 'air' }}</mat-icon>
                  @if (p.entries[0].type === 'grind') {
                    <strong>I forgot to record a grind</strong>
                    <span class="dim">A grind of {{ p.short | number: '1.2-2' }} g on {{ p.entries[0].occurred_at | date: 'EEE d MMM' }},
                      just after the jar's last entry. Weigh what is ground after.</span>
                  } @else {
                    <strong>I forgot to record {{ p.entries.length === 1 ? 'a session' : 'sessions' }}</strong>
                    <span class="dim">{{ sessionsSaid(p) }}</span>
                  }
                </button>
                <button type="button" role="radio" class="pick" [class.on]="lost()" [attr.aria-checked]="lost()" (click)="lost.set(true)">
                  <mat-icon>delete_outline</mat-icon>
                  <strong>It went missing</strong>
                  <span class="dim">Spilled, misread, gone: the {{ p.short | number: '1.2-2' }} g are recorded as lost.</span>
                </button>
              </div>
            }
          </div>
        </mat-tab>
        <mat-tab><ng-template mat-tab-label><span class="tab"><mat-icon>recycling</mat-icon><span>AVB</span></span></ng-template>
          <div class="form">
            <w-choice label="what happened" [tiles]="true" [options]="avbModes" [value]="avbMode()" (valueChange)="pickAvbMode($event)" />
            @if (avbMode() === 'collect') {
              @if (avbDevices().length) {
                <w-choice label="the device emptied" [tiles]="true" [options]="avbDevices()" [value]="emptied()" (valueChange)="device.set($event)" />
              } @else {
                <p class="dim">No session has been through a device since it was last emptied.</p>
              }
            }
            <div class="duo">
              <w-grams [label]="avbMode() === 'collect' ? 'weighed out' : 'used'" [max]="avbMode() === 'use' ? avbHeld() : null"
                [offerAll]="avbMode() === 'use'" [value]="grams()" (valueChange)="grams.set($event)" />
              <w-when [value]="date()" (valueChange)="date.set($event)" />
            </div>
            <ng-container *ngTemplateOutlet="rehearsed" />
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>Note</mat-label>
              <input matInput [ngModel]="note()" (ngModelChange)="note.set($event)" placeholder="Butter, a tincture" />
            </mat-form-field>
          </div>
        </mat-tab>
      </mat-tab-group>

      <ng-template #rehearsed>
        @if (refusal()) {
          <p class="refused"><mat-icon>report</mat-icon>Would be refused: {{ refusal() }}</p>
        } @else if (flow().length) {
          <w-flow [lines]="flow()" />
        }
      </ng-template>

      @if (error()) {
        <div class="error"><mat-icon>block</mat-icon>{{ error() }}</div>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Cancel</button>
      <button mat-flat-button (click)="save()" [disabled]="busy()">Record</button>
    </mat-dialog-actions>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    /* The dialog is at most 94vw: less its padding and its border, so the form never pushes past it. */
    .form { display: grid; gap: 18px; padding: 18px 0 4px; width: min(580px, calc(94vw - 52px)); }
    .duo { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 18px 24px; align-items: start; }
    .duo.solo { grid-template-columns: minmax(0, 1fr); }
    .stack { display: grid; gap: 16px; align-content: start; }
    .timer { width: 12ch; }
    .error { display: flex; gap: 8px; align-items: center; color: var(--w-warn); background: color-mix(in srgb, var(--w-warn) 8%, transparent);
      border: 1px solid color-mix(in srgb, var(--w-warn) 25%, transparent); border-radius: 12px; padding: 10px 12px; margin-top: 8px; }
    mat-icon { vertical-align: middle; }
    .tab { display: inline-flex; align-items: center; gap: 6px; }
    .dims { display: grid; grid-template-columns: auto 1fr; gap: 10px 16px; align-items: center; margin: 8px 0; }
    .ask { font-size: 13px; color: var(--w-dim); }
    .ask.lead { color: var(--mat-sys-on-surface); font-weight: 600; }
    .said { margin-top: 0; }
    .weighed { display: flex; gap: 8px; align-items: center; margin: 0; color: var(--w-good); font-size: 13px; }
    .refused { display: flex; gap: 8px; align-items: center; margin: 0; font-size: 13px; color: var(--w-warn); }
    .why { display: flex; gap: 8px; align-items: flex-start; font-size: 12px; margin: 0; }
    .why mat-icon { font-size: 18px; width: 18px; height: 18px; flex: none; }
    .forgot { display: grid; gap: 8px; }
    .pick { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 2px 12px; align-items: center; text-align: left;
      padding: 12px 14px; border-radius: 12px; border: 1px solid var(--w-glass-edge); background: none; color: inherit;
      font: inherit; cursor: pointer; }
    .pick mat-icon { grid-row: 1 / 3; color: var(--w-dim); }
    .pick .dim { font-size: 12px; line-height: 1.45; }
    .pick:hover { border-color: var(--w-primary); }
    .pick.on { border-color: var(--w-primary); background: color-mix(in srgb, var(--w-primary) 12%, transparent); }
    .pick.on mat-icon { color: var(--w-primary); }
    .more { margin-left: -8px; }
    /* A phone: the five tabs side by side, each its icon over its name, none paged out of sight. */
    @include bp.phone {
      .tab { flex-direction: column; gap: 2px; font-size: 11px; line-height: 1.1; }
      .duo { grid-template-columns: minmax(0, 1fr); }
    }
  `,
})
export class RecordDialog {
  protected readonly wits = inject(Wits);
  protected readonly ref = inject(MatDialogRef<RecordDialog>);
  private readonly snack = inject(MatSnackBar);

  private readonly data = inject<RecordData | null>(MAT_DIALOG_DATA, { optional: true });
  protected readonly tab = signal(this.data?.tab ?? 0);
  protected readonly product = signal(this.data?.product ?? '');
  protected readonly grams = signal<number | null>(null);
  protected readonly date = signal<Date | null>(null);
  protected readonly device = signal(this.data?.device ?? '');
  protected readonly temp = signal(185);
  protected readonly note = signal('');
  protected readonly account = signal(this.data?.account ?? 'storage');
  /** Whether a weighing short of the ledger is recorded as grams lost, not something forgotten. */
  protected readonly lost = signal(false);
  /** What the last weighing in this dialog recorded, when it stays open for the next. */
  protected readonly weighed = signal('');
  protected readonly avbMode = signal<'collect' | 'use'>('collect');
  protected readonly error = signal('');
  protected readonly busy = signal(false);
  protected readonly asks = ASKS;
  /** The session just recorded, asked how it went before the dialog closes. */
  protected readonly rating = signal<Written | null>(null);
  protected readonly scores = signal<Partial<Record<Dimension, number>>>({});
  protected readonly rated = computed(() => Object.keys(this.scores()).length > 0);
  /** Overall, unless asked for every dimension. */
  protected readonly allDims = signal(false);
  protected readonly asked = computed<Dimension[]>(() =>
    this.allDims() ? (Object.keys(ASKS) as Dimension[]) : ['overall']);
  /** The sessions and grinds that stand, newest first: what the form starts from. */
  private readonly standing = computed(() => this.wits.journal.value().entries.filter((e) => !e.corrected));
  /** The amounts ground or had lately, the most used first, four at most, smallest first. */
  protected readonly recent = computed(() => {
    const type = this.tab() === 0 ? 'grind' : this.tab() === 1 ? 'sesh' : '';
    return type ? lately(this.standing().filter((e) => e.type === type).slice(0, 30)) : [];
  });
  /** The Fill tab's form, which says what a fill records. */
  private readonly fill = viewChild(FillForm);

  /** What the ledger says the jar picked holds in the account on the scale. */
  protected readonly expected = computed(() => {
    const j = this.wits.jars.value().find((x) => x.slug === this.product());
    const a = this.account();
    return !j ? 0 : a === 'storage' ? j.storage : a === 'stash' ? j.stash : j.avb;
  });
  /** What a reading short of the ledger would be put down to, asked of the server as it is typed. */
  private readonly forgotten = httpResource<ForgottenPlan>(() => {
    const g = this.grams();
    if (this.tab() !== 3 || !this.product() || this.account() === 'avb' || g === null || g < 0 || g >= this.expected()) {
      return undefined;
    }
    const q = new URLSearchParams({ product: this.product(), account: this.account(), weighed: String(g) });
    return `/api/weigh/forgotten?${q.toString()}&v=${this.wits.version()}`;
  });
  protected readonly plan = computed(() => {
    const p = this.forgotten.hasValue() ? this.forgotten.value() : undefined;
    const g = this.grams();
    // A plan for a reading since changed back to no shortfall is not shown.
    return p?.entries.length && g !== null && g < this.expected() ? p : null;
  });
  /** Why a reading short of the ledger is recorded as lost, when it cannot be anything forgotten. */
  protected readonly why = computed(() => {
    const p = this.forgotten.hasValue() ? this.forgotten.value() : undefined;
    return !this.plan() && p?.why ? p.why : '';
  });

  protected readonly sealed = computed(() => this.wits.jars.value().filter((j) => j.storage > 0));
  protected readonly stashed = computed(() => this.wits.jars.value().filter((j) => j.stash > 0));
  /** The jar picked, as the shelf has it. */
  private readonly jar = computed(() => this.wits.jars.value().find((j) => j.slug === this.product()));
  protected readonly grindJars = computed(() => this.sealed().map((j) => jarOption(j, j.storage, 'sealed')));
  protected readonly seshJars = computed(() => this.stashed().map((j) => jarOption(j, j.stash, 'ground')));
  /** Every jar holding anything, the one picked among them, by what the account on the scale holds. */
  protected readonly weighJars = computed(() => {
    const a = this.account();
    const held = (j: Jar) => (a === 'storage' ? j.storage : a === 'stash' ? j.stash : j.avb);
    // What the account holds most of first; a jar with none of it last.
    return this.wits.jars.value()
      .filter((j) => j.storage + j.stash + j.avb > 0 || j.slug === this.product())
      .sort((x, y) => held(y) - held(x) || x.name.localeCompare(y.name))
      .map((j) => jarOption(j, held(j), ACCOUNT_WORDS[a] ?? a));
  });
  /** The most a grind or a session can take of the jar picked. */
  protected readonly held = computed(() => {
    const j = this.jar();
    return !j ? null : this.tab() === 0 ? j.storage : j.stash;
  });
  protected readonly avbHeld = computed(() => this.wits.avb.value()?.held ?? 0);
  protected readonly accounts: Choice[] = [
    { value: 'storage', label: 'Sealed', icon: 'inventory_2' },
    { value: 'stash', label: 'Ground', icon: 'grain' },
    { value: 'avb', label: 'Already vaped', icon: 'recycling' },
  ];
  protected readonly avbModes: Choice[] = [
    { value: 'collect', label: 'Emptied a device', icon: 'move_to_inbox', hint: 'what came out, weighed' },
    { value: 'use', label: 'Used some', icon: 'soup_kitchen', hint: 'butter, a tincture' },
  ];
  protected readonly deviceChoices = computed<Choice[]>(() => [
    { value: '', label: 'Not said', icon: 'remove', hint: 'no device' },
    // The most used first, so the fold keeps what is reached for.
    ...[...this.sessionDevices()].sort((a, b) => b.sessions - a.sessions || a.name.localeCompare(b.name))
      .map((d) => ({ value: d.slug, label: d.model?.trim() ? d.model.trim() : d.name, icon: iconOf(d),
        hint: [d.model ? d.maker : '', d.sessions ? `${String(d.sessions)} sessions` : 'not used yet'].filter((x) => x).join(' · ') })),
  ]);
  protected readonly modeChoices = computed<Choice[]>(() => [
    { value: '', label: 'Not said' },
    ...(this.picked()?.modes ?? []).map((m) => ({ value: m.name, label: m.name, hint: this.modeHint(m).replace(/^ · /, '') })),
  ]);
  protected readonly heaterChoices = computed<Choice[]>(() => [
    { value: '', label: 'Not said' },
    ...this.heaters().map((h) => ({ value: h.slug, label: h.name, icon: iconOf(h) })),
  ]);
  protected readonly settingChoices = computed<Choice[]>(() => [
    { value: '', label: 'Not said' },
    ...(this.heaterPicked()?.modes ?? []).map((m) => ({ value: m.name, label: m.name })),
  ]);
  /** The devices with sessions since they were last emptied, each with what they hold. */
  protected readonly avbDevices = computed<Choice[]>(() => (this.wits.avb.value()?.waiting ?? []).map((w) => {
    const d = this.wits.devices.value().find((x) => x.slug === w.device);
    return { value: w.device, label: d?.model?.trim() ? d.model.trim() : this.deviceName(w.device), icon: d ? iconOf(d) : 'help_outline',
      hint: `${w.grams.toFixed(2)} g in ${String(w.sessions)} ${w.sessions === 1 ? 'session' : 'sessions'}` };
  }));
  /** The device's modes where they sit on the dial. */
  protected readonly dialMarks = computed<DialMark[]>(() =>
    (this.picked()?.modes ?? []).filter((m) => m.temp).map((m) => ({ temp: m.temp ?? 0, label: m.name })));
  /** The jar's own terpenes, dominant first, each where it boils, as the shelf knows them. */
  protected readonly boils = computed<Boils[]>(() => {
    const known = new Map(this.wits.shelfTerpenes.value().map((t) => [t.name.toLowerCase(), t.boiling_point]));
    return (this.jar()?.terpenes ?? []).flatMap((name) => {
      const at = known.get(name.toLowerCase());
      return at ? [{ name, at }] : [];
    });
  });
  /** What the entry does to the jar, account by account, before it is recorded. */
  /**
   * The entry the form would record, rehearsed by the server as it is typed:
   * what it would do to each jar, or why it would be refused — worked out
   * by the recorder, never here.
   */
  private readonly rehearsal = httpResource<Preview>(() => {
    const e = this.entry();
    this.wits.version();
    return e && (this.grams() ?? 0) > 0 ? { url: `/api/${e.what}?dry=1`, method: 'POST', body: e.body } : undefined;
  });
  /** Why the entry would be refused, said before Record is pressed. */
  protected readonly refusal = computed(() => {
    const err = this.rehearsal.error() as HttpErrorResponse | undefined;
    return (err?.error as { error?: string } | null)?.error ?? '';
  });
  protected readonly flow = computed<FlowLine[]>(() => {
    const p = this.rehearsal.hasValue() ? this.rehearsal.value() : undefined;
    return p && !this.refusal() ? flowOf(p) : [];
  });
  /** The device emptied: the one picked, or the only one with sessions waiting. */
  protected readonly emptied = computed(() => {
    const waiting = this.wits.avb.value()?.waiting ?? [];
    return this.device() || (waiting.length === 1 ? waiting[0].device : '');
  });
  protected readonly mode = signal('');
  protected readonly heater = signal('');
  protected readonly setting = signal('');
  protected readonly timer = signal<number | null>(null);
  protected readonly heatedBy = needsHeat;
  /** What a session is had with: the vaporizers and glass still in use. */
  protected readonly sessionDevices = computed(() => this.wits.devices.value().filter((d) => sessionWith(d) && !d.retired));
  protected readonly heaters = computed(() => this.wits.devices.value().filter((d) => heats(d) && !d.retired));
  protected readonly picked = computed(() => this.wits.devices.value().find((x) => x.slug === this.device()));
  protected readonly heaterPicked = computed(() => this.heaters().find((x) => x.slug === this.heater()));
  /** Whether the mode picked runs a session timer. */
  protected readonly timed = computed(() => !!this.picked()?.modes.find((m) => m.name === this.mode())?.timer);
  /** The temperatures a device can be set to: an electric one's range, none for one heated by hand. */
  protected readonly range = computed(() => {
    const d = this.picked();
    if (!d || needsHeat(d) || d.power === 'manual') {
      return null;
    }
    // The API leaves a range out when the device has none.
    return { min: d.min_temp ?? 120, max: d.max_temp ?? 230 };
  });

  constructor() {
    // A session starts from the last one: its strain while the stash holds it,
    // its device, mode, heater and temperature. Only once, and only what is
    // still unpicked.
    let started = false;
    effect(() => {
      const last = this.standing().find((e) => e.type === 'sesh');
      if (started || this.tab() !== 1 || !last || !this.wits.devices.value().length) {
        return;
      }
      started = true;
      untracked(() => {
        this.startFrom(last);
      });
    });
  }

  private startFrom(last: Entry): void {
    if (!this.product() && this.stashed().some((j) => j.slug === last.product)) {
      this.product.set(last.product ?? '');
    }
    if (this.device() || !last.device) {
      return;
    }
    this.pickDevice(last.device);
    if (last.mode) {
      this.pickMode(last.mode);
    }
    if (last.heater) {
      this.pickHeater(last.heater);
      this.setting.set(last.setting ?? '');
    }
    if (last.temperature) {
      this.temp.set(last.temperature);
    }
  }

  protected score(d: Dimension, v: number | null): void {
    this.scores.update((all) => {
      const kept = Object.entries(all).filter(([k]) => k !== d);
      return Object.fromEntries(v === null ? kept : [...kept, [d, v]]);
    });
  }

  protected async rate(w: Written): Promise<void> {
    this.busy.set(true);
    try {
      const out = await this.wits.rate(w.entry.hash, this.scores(), '');
      this.snack.open(out.message, undefined, { duration: 3000 });
      this.ref.close(w);
    } catch (err) {
      this.snack.open((err as Error).message, 'OK', { duration: 6000 });
    } finally {
      this.busy.set(false);
    }
  }

  /** A device's name, or what a session that named none is called. */
  protected deviceName(slug: string): string {
    return slug ? (this.wits.devices.value().find((d) => d.slug === slug)?.name ?? slug) : 'no device named';
  }

  protected pickDevice(slug: string): void {
    this.device.set(slug);
    this.mode.set('');
    this.timer.set(null);
    const d = this.wits.devices.value().find((x) => x.slug === slug);
    if (d?.default_temp) {
      this.temp.set(d.default_temp);
    }
    if (!d || !needsHeat(d)) {
      this.heater.set('');
      this.setting.set('');
    }
  }

  /** A mode starts the session at its usual temperature, with its timer. */
  protected pickMode(name: string): void {
    this.mode.set(name);
    const m = this.picked()?.modes.find((x) => x.name === name);
    if (m?.temp) {
      this.temp.set(m.temp);
    }
    this.timer.set(m?.timer ?? null);
  }

  protected pickAvbMode(mode: string): void {
    this.avbMode.set(mode === 'use' ? 'use' : 'collect');
  }

  protected pickHeater(slug: string): void {
    this.heater.set(slug);
    this.setting.set('');
  }

  protected modeHint(m: DeviceMode): string {
    const parts = [m.temp ? `${m.temp} °C` : '', m.timer ? `${m.timer} s` : '', m.signature ?? ''].filter((x) => x);
    return parts.length ? ` · ${parts.join(', ')}` : '';
  }

  /** Forgotten sessions in a line: how many, of about what, over which days, with what. */
  protected sessionsSaid(p: ForgottenPlan): string {
    const entries = p.entries;
    const total = entries.reduce((s, e) => s + Math.round(e.grams * 100), 0) / 100;
    const day = (e: Entry) => new Date(e.occurred_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
    const first = entries[0];
    const last = entries[entries.length - 1];
    const what = entries.length === 1 ? `A session of ${total.toFixed(2)} g` :
      `${String(entries.length)} sessions of about ${(total / entries.length).toFixed(2)} g`;
    const when = day(first) === day(last) ? day(first) : `${day(first)} – ${day(last)}`;
    const how = first.device ? `, with the ${this.deviceName(first.device)}` : '';
    const sized = { sessions: 'the size your sessions usually are', load: 'a load of your gear', day: 'one a day, nothing saying how big a session is' };
    return `${what}, ${when}${how} — ${sized[p.sized_by ?? 'day']}.`;
  }

  /**
   * Records a weighing short of the ledger as what was forgotten. After a
   * forgotten grind the dialog stays, on what is ground, to be weighed next.
   */
  private async weighForgotten(): Promise<boolean> {
    const out = await this.wits.weighForgotten({ product: this.product(), account: this.account(), weighed: this.grams() ?? 0 });
    if (this.account() !== 'storage') {
      this.snack.open(out.message, undefined, { duration: 3500 });
      return true;
    }
    this.weighed.set(`Recorded ${out.message}. Now weigh what is ground of it.`);
    this.account.set('stash');
    this.grams.set(null);
    return false;
  }

  protected async save(): Promise<void> {
    this.error.set('');
    this.busy.set(true);
    try {
      if (this.tab() === 3 && this.plan() && !this.lost()) {
        if (await this.weighForgotten()) {
          this.ref.close();
        }
        return;
      }
      const w = await this.send();
      if (this.tab() === 1 && 'entry' in w) {
        // A session is asked about at once, while it is fresh; Later closes.
        this.rating.set(w);
        return;
      }
      this.snack.open(w.message, undefined, { duration: 3500 });
      this.ref.close(w);
    } catch (err) {
      this.error.set((err as Error).message);
    } finally {
      this.busy.set(false);
    }
  }

  /** The entry a grind, a session or AVB would record, as the API takes it: rehearsed as typed, and sent on Record. */
  private readonly entry = computed<{ what: string; body: object } | null>(() => {
    const day = this.date();
    const date = day ? isoDay(day) : undefined;
    switch (this.tab()) {
      case 0:
        return this.product() ? { what: 'grind', body: { product: this.product(), grams: this.grams() ?? 0, date } } : null;
      case 1:
        return this.product() ? { what: 'sesh', body: this.session(date) } : null;
      case 4:
        return this.avbEntry(date);
      default:
        return null;
    }
  });

  /** A session as the API takes it: the jar, the grams, and what it was had with. */
  private session(date: string | undefined): object {
    return {
      product: this.product(), grams: this.grams() ?? 0, device: this.device() || undefined,
      temperature: this.range() ? this.temp() : undefined, mode: this.mode() || undefined,
      heater: this.heater() || undefined, setting: this.setting().trim() || undefined,
      timer: this.timed() ? (this.timer() ?? undefined) : undefined, note: this.note() || undefined, date,
    };
  }

  /** AVB weighed out of a device, or used up, as the API takes it. */
  private avbEntry(date: string | undefined): { what: string; body: object } {
    const grams = this.grams() ?? 0;
    const note = this.note() || undefined;
    return this.avbMode() === 'collect'
      ? { what: 'avb/collect', body: { grams, device: this.emptied(), note, date } }
      : { what: 'avb/use', body: { grams, note, date } };
  }

  private send(): Promise<Written | { message: string }> {
    const day = this.date();
    const date = day ? isoDay(day) : undefined;
    switch (this.tab()) {
      case 2: {
        const fill = this.fill();
        return fill ? this.wits.fill({ ...fill.request(), date }) : Promise.reject(new Error('the fill form is not open'));
      }
      case 3:
        return this.wits.weigh({ product: this.product(), account: this.account(), weighed: this.grams() ?? 0 });
      default: {
        const e = this.entry();
        return e ? this.wits.submit<Written | { message: string }>(e.what, e.body) : Promise.reject(new Error('pick the jar first'));
      }
    }
  }
}

/** What an account's grams are called on a jar's tile. */
const ACCOUNT_WORDS: Record<string, string> = { storage: 'sealed', stash: 'ground', avb: 'vaped' };

/** A jar as the picker draws it: its glass filled as it stands, the grams that matter here named. */
function jarOption(j: Jar, grams: number, what: string): JarOption {
  return {
    slug: j.slug, name: j.name, grams, what,
    capacity: Math.max(j.storage + j.stash + j.avb, 0.01),
    layers: [
      { grams: j.storage, color: 'var(--w-storage)', label: 'sealed' },
      { grams: j.stash, color: 'var(--w-stash)', label: 'ground' },
      { grams: j.avb, color: 'var(--w-avb)', label: 'already vaped' },
    ],
  };
}

/** A device's icon, by what it is. */
function iconOf(d: Device): string {
  return CATEGORIES.find((c) => c.id === d.category)?.icon ?? 'devices';
}

/** The amounts in the entries the most used first, four of them, then smallest first. */
function lately(entries: Entry[]): number[] {
  const count = new Map<number, number>();
  for (const e of entries) {
    count.set(e.grams, (count.get(e.grams) ?? 0) + 1);
  }
  return [...count.entries()].sort((a, b) => b[1] - a[1] || a[0] - b[0]).slice(0, 4).map(([g]) => g).sort((a, b) => a - b);
}

