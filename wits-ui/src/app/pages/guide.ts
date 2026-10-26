import {
  ChangeDetectionStrategy, Component, DestroyRef, ElementRef, ViewEncapsulation, afterNextRender, inject, signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import { GUIDE, Shot } from '../data/guide';
import { Keys } from '../data/keys';
import { openRecord } from './record';

const DOCS = 'https://github.com/TheDonDope/wits/blob/main/';

/**
 * The user guide, off the header's menu: every page and what it is for, with
 * pictures, and beside each the same done in a terminal — the commands and
 * the terminal interface's keys — since the browser is one way into wits of
 * three. The content is data (data/guide.ts); this draws it.
 */
@Component({
  selector: 'w-guide',
  imports: [MatButtonModule, MatIconModule, MatTooltipModule, RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  // The prose is HTML from the data, which carries no component attributes,
  // so the styles are global and every rule is scoped under w-guide.
  encapsulation: ViewEncapsulation.None,
  host: { '(click)': 'follow($event)' },
  template: `
    <header class="hero glass">
      <div class="intro">
        <span class="eyebrow">user guide</span>
        <h1>Everything wits does, and where to find it</h1>
        <p class="lead">Every page of this interface, what it is for and how to use it — and beside each, the same thing
          done in a terminal. The browser is one way into wits; the command line and the terminal interface read and
          write the same repository, through the same checks.</p>
        <div class="ways">
          <span class="way"><mat-icon>web</mat-icon><span><b>The browser</b> — this interface, the desktop window and the phone</span></span>
          <span class="way"><mat-icon>terminal</mat-icon><span><b>The terminal interface</b> — <code>wits</code> on its own</span></span>
          <span class="way"><mat-icon>code</mat-icon><span><b>The command line</b> — <code>wits status</code>, <code>wits grind</code> and the rest</span></span>
        </div>
      </div>
      <div class="pair">
        <figure class="frame browser"><button class="zoom" (click)="zoom(hero[0])">
          <span class="chrome" aria-hidden="true"><i></i><i></i><i></i><span>127.0.0.1:7341</span></span>
          <img [src]="hero[0].src" [alt]="hero[0].alt" [width]="hero[0].w" [height]="hero[0].h" />
        </button></figure>
        <figure class="frame terminal"><button class="zoom" (click)="zoom(hero[1])">
          <span class="chrome" aria-hidden="true"><i></i><i></i><i></i><span>~/wits — wits</span></span>
          <img [src]="hero[1].src" [alt]="hero[1].alt" [width]="hero[1].w" [height]="hero[1].h" />
        </button></figure>
      </div>
    </header>

    <div class="body">
      <nav class="toc" aria-label="Sections">
        @for (s of sections; track s.id; let i = $index) {
          @if (i === 0 || sections[i - 1].group !== s.group) { <span class="eyebrow">{{ s.group }}</span> }
          <button [class.on]="active() === s.id" [attr.aria-current]="active() === s.id ? 'true' : null" (click)="go(s.id)">
            <mat-icon>{{ s.icon }}</mat-icon><span>{{ s.title }}</span>
          </button>
        }
      </nav>

      <div class="sections">
        @for (s of sections; track s.id; let i = $index) {
          <section class="glass topic" [id]="s.id">
            <div class="head">
              <span class="badge"><mat-icon>{{ s.icon }}</mat-icon></span>
              <div>
                <span class="eyebrow">{{ s.group }} · {{ i + 1 }}</span>
                <h2>{{ s.title }}</h2>
              </div>
            </div>
            <p class="lead" [innerHTML]="s.lead"></p>

            @if (s.diagram === 'accounts') {
              <div class="flow" role="img" aria-label="Storage, then stash, then consumed, then AVB">
                <span class="acct storage"><b>Storage</b>sealed jars</span><span class="arrow">grind</span>
                <span class="acct stash"><b>Stash</b>ground</span><span class="arrow">session</span>
                <span class="acct consumed"><b>Consumed</b>through a device</span><span class="arrow">collect</span>
                <span class="acct avb"><b>AVB</b>already vaped</span>
              </div>
            }
            @for (p of s.body ?? []; track $index) { <p [innerHTML]="p"></p> }
            @if (s.steps) {
              <ol class="steps">@for (st of s.steps; track $index) { <li [innerHTML]="st"></li> }</ol>
            }
            @for (p of s.then ?? []; track $index) { <p [innerHTML]="p"></p> }

            @if (s.shots?.length) {
              <div class="shots" [class.many]="(s.shots?.length ?? 0) > 1" [class.phones]="s.shots?.[0]?.frame === 'phone'">
                @for (sh of s.shots; track sh.src) {
                  <figure class="frame" [class]="sh.frame">
                    <button class="zoom" (click)="zoom(sh)">
                      @if (sh.frame === 'browser') {
                        <span class="chrome" aria-hidden="true"><i></i><i></i><i></i><span>127.0.0.1:7341{{ sh.url }}</span></span>
                      }
                      <img [src]="sh.src" [alt]="sh.alt" [width]="sh.w" [height]="sh.h" loading="lazy" decoding="async" />
                    </button>
                    <figcaption>{{ sh.caption }}</figcaption>
                  </figure>
                }
              </div>
            }

            @if (s.page || s.record) {
              <div class="open">
                @if (s.page; as p) {
                  <a mat-flat-button [routerLink]="p.path"><mat-icon>arrow_forward</mat-icon>{{ p.label }}</a>
                }
                @if (s.record; as r) {
                  <button mat-flat-button (click)="record(r.tab)"><mat-icon>add</mat-icon>{{ r.label }}</button>
                }
              </div>
            }

            @if (s.cli?.length || s.tui?.length) {
              <aside class="term">
                <header><mat-icon>terminal</mat-icon><span><b>The same, in a terminal</b></span></header>
                <div class="cols" [class.split]="s.tui?.length">
                  @if (s.cli?.length) {
                    <div class="cli">
                      <span class="eyebrow">the command line</span>
                      @for (e of s.cli; track $index) {
                        <div class="ex">
                          <pre class="console"><code><span class="cmd">@for (line of lines(e.cmd); track $index) {<span class="prompt">$ </span>{{ line }}
}</span>@if (e.out) {<span class="out">{{ e.out }}</span>}</code></pre>
                          <button mat-icon-button class="copy" (click)="copy(e.cmd)" matTooltip="Copy the command" aria-label="Copy the command">
                            <mat-icon>content_copy</mat-icon>
                          </button>
                          @if (e.note) { <p class="note" [innerHTML]="e.note"></p> }
                        </div>
                      }
                    </div>
                  }
                  @if (s.tui?.length) {
                    <div class="tui">
                      <span class="eyebrow">the terminal interface</span>
                      <dl>
                        @for (k of s.tui; track $index) {
                          <dt>@for (key of k.keys; track $index) { <kbd>{{ key }}</kbd> }</dt><dd>{{ k.does }}</dd>
                        }
                      </dl>
                      @if (s.tuiShot; as t) {
                        <figure class="frame terminal">
                          <button class="zoom" (click)="zoom(t)">
                            <span class="chrome" aria-hidden="true"><i></i><i></i><i></i><span>~/wits — wits</span></span>
                            <img [src]="t.src" [alt]="t.alt" [width]="t.w" [height]="t.h" loading="lazy" decoding="async" />
                          </button>
                          <figcaption>{{ t.caption }}</figcaption>
                        </figure>
                      }
                    </div>
                  }
                </div>
              </aside>
            }
            @if (s.tip) { <p class="tip"><mat-icon>lightbulb</mat-icon><span [innerHTML]="s.tip"></span></p> }
          </section>
        }

        <section class="glass topic more">
          <span class="eyebrow">and further</span>
          <p>Every command takes <code>--help</code>. The whole of it, written out, is in the repository:</p>
          <div class="open">
            <a mat-stroked-button [href]="docs + 'docs/browser.md'" target="_blank" rel="noopener"><mat-icon>web</mat-icon>Wits in a browser</a>
            <a mat-stroked-button [href]="docs + 'docs/terminal.md'" target="_blank" rel="noopener"><mat-icon>terminal</mat-icon>Wits in the terminal</a>
            <a mat-stroked-button [href]="docs + 'README.md'" target="_blank" rel="noopener"><mat-icon>description</mat-icon>The README</a>
          </div>
        </section>
      </div>
    </div>

    @if (zoomed(); as z) {
      <div class="lightbox" role="dialog" [attr.aria-label]="z.alt">
        <button class="shut" (click)="zoomed.set(null)" aria-label="Close the picture">
          <img [src]="z.src" [alt]="z.alt" />
          <span>{{ z.caption }}</span>
        </button>
      </div>
    }
  `,
  styles: `
    @use 'breakpoints' as bp;
    w-guide {
      display: grid; grid-template-columns: minmax(0, 1fr); gap: 24px;
      p { margin: 0; line-height: 1.6; }
      code, kbd { font: 0.9em var(--w-mono); }
      p code, li code, dd code, .way code { padding: 1px 6px; border-radius: 6px; background: color-mix(in srgb, var(--w-fg) 8%, transparent); }
      kbd { display: inline-block; min-width: 1.6em; padding: 1px 6px; text-align: center; border-radius: 6px;
        border: 1px solid var(--w-glass-edge); border-bottom-width: 2px; background: color-mix(in srgb, var(--w-fg) 6%, transparent); }
      a:not([mat-flat-button], [mat-stroked-button]) { color: var(--w-primary); }
      .lead { font-size: 17px; color: var(--w-fg); }

      .hero { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 6fr); gap: 32px; padding: 32px; align-items: center;
        background: radial-gradient(120% 140% at 0% 0%, color-mix(in srgb, var(--w-primary) 16%, transparent), transparent 60%), var(--w-glass); }
      .intro { display: grid; gap: 16px; }
      h1 { margin: 0; font: 600 clamp(28px, 3.4vw, 40px)/1.12 var(--w-head); letter-spacing: -0.02em;
        background: linear-gradient(90deg, var(--w-fg), var(--w-primary)); -webkit-background-clip: text; background-clip: text; color: transparent; }
      .ways { display: grid; gap: 8px; }
      .way { display: flex; gap: 10px; align-items: flex-start; line-height: 1.5; color: var(--w-dim); font-size: 14px; }
      .way b { color: var(--w-fg); font-weight: 600; }
      .way mat-icon { color: var(--w-accent); flex: none; }
      .pair { position: relative; display: grid; padding-bottom: 18%; }
      .pair .terminal { position: absolute; width: 62%; right: -8px; bottom: 0; box-shadow: 0 24px 60px -20px rgb(0 0 0 / 60%); }
      .pair .browser { width: 88%; }

      .body { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: 24px; align-items: start; }
      .toc { position: sticky; top: 88px; display: grid; gap: 2px; max-height: calc(100vh - 120px); overflow-y: auto; padding-right: 4px; scrollbar-width: thin; }
      .toc .eyebrow { margin: 14px 0 4px 12px; }
      .toc .eyebrow:first-child { margin-top: 0; }
      .toc button { display: flex; align-items: center; gap: 10px; padding: 7px 12px; border: 0; border-radius: 10px; background: none;
        color: var(--w-dim); font: 13px/1.3 var(--w-font); text-align: left; cursor: pointer; }
      .toc button mat-icon { font-size: 18px; width: 18px; height: 18px; flex: none; }
      .toc button:hover { color: var(--w-fg); background: color-mix(in srgb, var(--w-fg) 5%, transparent); }
      .toc button.on { color: var(--w-fg); background: color-mix(in srgb, var(--w-primary) 16%, transparent); }
      .toc button.on mat-icon { color: var(--w-primary); }

      .sections { display: grid; gap: 24px; min-width: 0; }
      .topic { display: grid; gap: 16px; padding: 28px 32px; scroll-margin-top: 88px; }
      .head { display: flex; gap: 14px; align-items: center; }
      .head h2 { margin: 2px 0 0; font: 600 24px/1.2 var(--w-head); letter-spacing: -0.01em; }
      .badge { display: grid; place-items: center; width: 44px; height: 44px; flex: none; border-radius: 14px; color: var(--w-on-accent);
        background: linear-gradient(135deg, var(--w-primary), var(--w-accent)); }
      .steps { margin: 0; padding: 0; list-style: none; counter-reset: step; display: grid; gap: 10px; }
      .steps li { counter-increment: step; position: relative; padding-left: 40px; line-height: 1.6; }
      .steps li::before { content: counter(step); position: absolute; left: 0; top: 1px; width: 26px; height: 26px; border-radius: 50%;
        display: grid; place-items: center; font: 600 12px var(--w-num); color: var(--w-primary);
        background: color-mix(in srgb, var(--w-primary) 14%, transparent); }

      .flow { display: flex; align-items: stretch; flex-wrap: wrap; gap: 8px; padding: 4px 0; }
      .acct { flex: 1 1 110px; display: grid; gap: 2px; padding: 12px 14px; border-radius: 12px; font-size: 12px; color: var(--w-dim);
        border: 1px solid color-mix(in srgb, var(--c) 45%, transparent); background: color-mix(in srgb, var(--c) 12%, transparent); }
      .acct b { color: var(--c); font: 600 15px var(--w-head); }
      .storage { --c: var(--w-storage); } .stash { --c: var(--w-stash); } .consumed { --c: var(--w-sesh); } .avb { --c: var(--w-avb); }
      .arrow { align-self: center; font: 11px var(--w-mono); color: var(--w-dim); }
      .arrow::after { content: ' →'; }

      .shots { display: grid; gap: 16px; }
      .shots.many { grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); }
      .shots.phones { grid-template-columns: repeat(2, minmax(0, 260px)); justify-content: center; }
      .zoom { display: grid; padding: 0; border: 0; background: none; color: inherit; cursor: zoom-in; text-align: left; }
      .zoom:focus-visible { outline: 2px solid var(--w-primary); outline-offset: -2px; }
      .frame { margin: 0; display: grid; border-radius: 12px; overflow: hidden; border: 1px solid var(--w-glass-edge);
        background: var(--w-ink); transition: transform 0.2s, box-shadow 0.2s; }
      .frame:hover { transform: translateY(-2px); box-shadow: 0 18px 40px -18px rgb(0 0 0 / 55%); }
      .frame img { display: block; width: 100%; height: auto; }
      .frame figcaption { padding: 10px 14px; font-size: 13px; color: var(--w-dim); background: var(--w-glass); border-top: 1px solid var(--w-glass-edge); }
      .frame.phone { border-radius: 28px; border-width: 6px; border-color: color-mix(in srgb, var(--w-fg) 16%, transparent); }
      .frame.dialog { background: transparent; }
      .chrome { display: flex; align-items: center; gap: 6px; padding: 8px 12px; font: 11px var(--w-mono); color: var(--w-dim);
        background: color-mix(in srgb, var(--w-fg) 6%, var(--w-ink)); border-bottom: 1px solid var(--w-glass-edge); white-space: nowrap; overflow: hidden; }
      .chrome i { width: 10px; height: 10px; border-radius: 50%; flex: none; background: color-mix(in srgb, var(--w-fg) 20%, transparent); }
      .chrome span { margin-left: 8px; overflow: hidden; text-overflow: ellipsis; }
      .terminal .chrome span { margin: 0 auto; }

      .open { display: flex; gap: 8px; flex-wrap: wrap; }

      .term { display: grid; gap: 14px; padding: 18px 20px; border-radius: calc(var(--w-radius) * 0.75);
        border: 1px solid color-mix(in srgb, var(--w-accent) 30%, var(--w-glass-edge));
        background: color-mix(in srgb, var(--w-accent) 5%, transparent); }
      .term > header { display: flex; align-items: center; gap: 10px; color: var(--w-accent); }
      .term > header b { color: var(--w-fg); }
      .cols { display: grid; gap: 20px; }
      .cols.split { grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); }
      .cli, .tui { display: grid; gap: 10px; align-content: start; min-width: 0; }
      .ex { position: relative; display: grid; gap: 6px; }
      .console { margin: 0; padding: 12px 44px 12px 14px; overflow-x: auto; border-radius: 10px; background: var(--w-ink);
        color: color-mix(in srgb, var(--w-fg) 92%, transparent); font: 12.5px/1.55 var(--w-mono); border: 1px solid var(--w-glass-edge); }
      .console .prompt { color: var(--w-accent); user-select: none; }
      .console .cmd { color: var(--w-fg); font-weight: 600; }
      .console .out { display: block; margin-top: 4px; color: var(--w-dim); white-space: pre; }
      .copy { position: absolute; top: 2px; right: 2px; transform: scale(0.8); color: var(--w-dim); }
      .note { font-size: 13px; color: var(--w-dim); }
      dl { margin: 0; display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 8px 12px; align-items: baseline; font-size: 14px; }
      dt { display: flex; gap: 4px; justify-content: flex-end; white-space: nowrap; }
      dd { margin: 0; color: var(--w-dim); }
      .tip { display: flex; gap: 10px; align-items: flex-start; font-size: 14px; color: var(--w-dim); }
      .tip mat-icon { color: var(--w-avb); flex: none; }
      .more { gap: 12px; }

      .lightbox { position: fixed; inset: 0; z-index: 30; background: color-mix(in srgb, var(--w-ink) 82%, transparent); backdrop-filter: blur(10px); }
      .shut { width: 100%; height: 100%; display: grid; place-items: center; align-content: center; gap: 12px; padding: 24px;
        border: 0; background: none; color: var(--w-fg); font: 14px var(--w-font); cursor: zoom-out; }
      .shut img { max-width: min(1500px, 96vw); max-height: 84vh; border-radius: 10px; box-shadow: 0 30px 80px rgb(0 0 0 / 60%); }

      @include bp.phone {
        gap: 16px;
        .hero { grid-template-columns: minmax(0, 1fr); padding: 20px; gap: 20px; }
        .body { grid-template-columns: minmax(0, 1fr); gap: 16px; }
        .toc { position: static; display: flex; gap: 6px; overflow-x: auto; max-height: none; padding-bottom: 4px; }
        .toc .eyebrow { display: none; }
        .toc button { flex: none; border: 1px solid var(--w-glass-edge); border-radius: 999px; }
        .topic { padding: 20px 16px; }
        .head h2 { font-size: 20px; }
        .lead { font-size: 15px; }
        .cols.split { grid-template-columns: minmax(0, 1fr); }
        .shots.phones { grid-template-columns: repeat(2, minmax(0, 1fr)); }
        .frame.phone { border-radius: 18px; border-width: 4px; }
        .acct { flex-basis: 100%; display: flex; justify-content: space-between; align-items: baseline; padding: 8px 12px; }
        .flow { gap: 2px; }
        .arrow { flex-basis: 100%; text-align: center; }
        .arrow::after { content: ' ↓'; }
        .term { padding: 14px; }
      }
    }
  `,
})
export class Guide {
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  protected readonly sections = GUIDE;
  protected readonly docs = DOCS;
  /** The dashboard twice, a browser's and a terminal's, for the top of the page. */
  protected readonly hero: readonly Shot[] = [
    { src: 'guide/dashboard.webp', alt: 'The dashboard in a browser', caption: 'The dashboard, in a browser.', frame: 'browser', w: 1440, h: 900 },
    { src: 'guide/tui-dashboard.webp', alt: 'The dashboard in the terminal interface', caption: 'The same dashboard, in the terminal interface wits opens.',
      frame: 'terminal', w: 1500, h: 900 },
  ];
  /** The section being read, which the contents light up. */
  protected readonly active = signal(GUIDE[0].id);
  /** A picture opened full size. */
  protected readonly zoomed = signal<Shot | null>(null);

  constructor() {
    const destroy = inject(DestroyRef);
    inject(Keys).register('Guide', () => [
      { keys: ['j'], label: 'j/k', help: 'next section', run: () => { this.step(1); } },
      { keys: ['k'], help: 'previous section', run: () => { this.step(-1); }, quiet: true },
      { keys: ['Escape'], help: 'close the picture', run: () => { this.zoomed.set(null); }, quiet: true },
    ], destroy);
    afterNextRender(() => {
      const watch = new IntersectionObserver((seen) => {
        const top = seen.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top).at(0);
        if (top) {
          this.active.set(top.target.id);
        }
      }, { rootMargin: '-90px 0px -55% 0px' });
      this.host.nativeElement.querySelectorAll('section.topic[id]').forEach((s) => { watch.observe(s); });
      destroy.onDestroy(() => { watch.disconnect(); });
      const at = this.route.snapshot.fragment;
      if (at) {
        this.go(at, false);
      }
    });
  }

  protected lines(cmd: string): string[] {
    return cmd.split('\n');
  }

  /** Scrolls to a section and names it in the address, for a link to it. */
  protected go(id: string, smooth = true): void {
    const el = this.host.nativeElement.querySelector<HTMLElement>(`#${CSS.escape(id)}`);
    if (!el) {
      return;
    }
    const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
    el.scrollIntoView({ behavior: smooth && !still ? 'smooth' : 'auto', block: 'start' });
    this.active.set(id);
    void this.router.navigate([], { fragment: id, replaceUrl: true });
  }

  /**
   * A link inside the prose to another section. Followed as written, "#weigh"
   * would resolve against the base href, which on Pages is /wits/ — the
   * dashboard, not this page.
   */
  protected follow(ev: MouseEvent): void {
    const a = (ev.target as HTMLElement).closest('a');
    const href = a?.getAttribute('href');
    if (href?.startsWith('#')) {
      ev.preventDefault();
      this.go(href.slice(1));
    }
  }

  private step(d: number): void {
    const i = GUIDE.findIndex((s) => s.id === this.active());
    const next = GUIDE[Math.min(Math.max(i + d, 0), GUIDE.length - 1)];
    this.go(next.id);
  }

  protected zoom(shot: Shot): void {
    this.zoomed.set(shot);
  }

  protected record(tab: number): void {
    openRecord(this.dialog, { tab });
  }

  protected copy(cmd: string): void {
    void navigator.clipboard.writeText(cmd).then(
      () => this.snack.open('Copied.', undefined, { duration: 1500 }),
      () => this.snack.open('The browser would not copy it.', 'OK', { duration: 3000 }),
    );
  }
}
