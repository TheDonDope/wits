import { ComponentFixture, TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { ChoiceInput } from './choice';
import { FlowPreview, flowOf } from './flow';
import { GramsInput } from './grams';
import { ScaleInput } from './scale';
import { TempDial } from './temp-dial';
import { TempRange } from './temp-range';

/** The component's element, typed. */
function host(f: ComponentFixture<unknown>): HTMLElement {
  return f.nativeElement as HTMLElement;
}

/** The one element a selector names, or the test fails saying which. */
function find(f: ComponentFixture<unknown>, selector: string): HTMLElement {
  const el = host(f).querySelector<HTMLElement>(selector);
  if (!el) {
    throw new Error(`nothing matches ${selector}`);
  }
  return el;
}

function click(el: Element | null): void {
  (el as HTMLElement).click();
}

describe('GramsInput', () => {
  it('steps by five centigrams, never past what the jar holds nor below nothing', () => {
    const f = TestBed.createComponent(GramsInput);
    f.componentRef.setInput('max', 0.12);
    f.detectChanges();
    const [less, more] = host(f).querySelectorAll<HTMLButtonElement>('button.step');
    click(more);
    click(more);
    expect(f.componentInstance.value()).toBe(0.1);
    click(more);
    expect(f.componentInstance.value()).toBe(0.12);
    f.detectChanges();
    for (let i = 0; i < 4; i++) {
      click(less);
      f.detectChanges();
    }
    expect(f.componentInstance.value()).toBe(0);
  });

  it('keeps the figure while half a number is typed', () => {
    const f = TestBed.createComponent(GramsInput);
    f.componentInstance.value.set(0.2);
    f.detectChanges();
    const input = find(f, 'input[type=number]');
    Object.defineProperty(input, 'validity', { value: { badInput: true } });
    input.dispatchEvent(new Event('input'));
    expect(f.componentInstance.value()).toBe(0.2);
  });
});

describe('ScaleInput', () => {
  it('lights the difference against the ledger, to the centigram', () => {
    const f = TestBed.createComponent(ScaleInput);
    f.componentRef.setInput('expected', 12.94);
    f.componentInstance.value.set(12.6);
    f.detectChanges();
    const scale = find(f, '.scale');
    expect(scale.classList).toContain('short');
    expect(host(f).querySelector('.delta')?.textContent).toContain('0.34');
    f.componentInstance.value.set(12.94);
    f.detectChanges();
    expect(scale.classList).toContain('level');
  });
});

describe('ChoiceInput', () => {
  const options = ['a', 'b', 'c', 'd', 'e', 'f'].map((v) => ({ value: v, label: v.toUpperCase() }));

  it('folds past the number asked, always showing the one chosen', () => {
    const f = TestBed.createComponent(ChoiceInput);
    f.componentRef.setInput('options', options);
    f.componentRef.setInput('fold', 3);
    f.componentInstance.value.set('f');
    f.detectChanges();
    const labels = [...host(f).querySelectorAll('button[role=radio] .label')].map((e) => e.textContent);
    expect(labels).toEqual(['A', 'B', 'C', 'F']);
    expect(host(f).querySelector('button.more')?.textContent).toContain('2 more');
  });

  it('moves the choice with the arrow keys, round from the ends', () => {
    const f = TestBed.createComponent(ChoiceInput);
    f.componentRef.setInput('options', options.slice(0, 3));
    f.componentInstance.value.set('c');
    f.detectChanges();
    const chosen = find(f, 'button[aria-checked=true]');
    chosen.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    expect(f.componentInstance.value()).toBe('a');
  });
});

describe('TempDial', () => {
  it('steps with the keys inside the device’s range, and lights what boils by then', () => {
    const f = TestBed.createComponent(TempDial);
    f.componentRef.setInput('min', 40);
    f.componentRef.setInput('max', 210);
    f.componentRef.setInput('terpenes', [{ name: 'Myrcene', at: 167 }, { name: 'Linalool', at: 198 }]);
    f.componentInstance.value.set(195);
    f.detectChanges();
    const svg = host(f).querySelector('svg') as SVGElement;
    svg.dispatchEvent(new KeyboardEvent('keydown', { key: 'PageUp' }));
    f.detectChanges();
    expect(f.componentInstance.value()).toBe(200);
    expect(host(f).querySelectorAll('.chip.on').length).toBe(2);
    svg.dispatchEvent(new KeyboardEvent('keydown', { key: 'End' }));
    svg.dispatchEvent(new KeyboardEvent('keydown', { key: 'PageUp' }));
    expect(f.componentInstance.value()).toBe(210);
  });
});

describe('FlowPreview', () => {
  it('draws every account against the fullest figure', () => {
    const f = TestBed.createComponent(FlowPreview);
    f.componentRef.setInput('lines', [
      { label: 'sealed', before: 10, after: 9.5, tone: 'blue' },
      { label: 'ground', before: 1, after: 1.5, tone: 'green' },
    ]);
    f.detectChanges();
    const kept = [...host(f).querySelectorAll<HTMLElement>('.kept')].map((e) => e.style.width);
    expect(kept).toEqual(['95%', '10%']);
  });
});

describe('TempRange', () => {
  it('keeps the thumbs from passing each other', () => {
    const f = TestBed.createComponent(TempRange);
    f.componentInstance.from.set(40);
    f.componentInstance.to.set(210);
    f.detectChanges();
    const [lo, hi] = host(f).querySelectorAll<HTMLInputElement>('input[type=range]');
    lo.value = '230';
    lo.dispatchEvent(new Event('input'));
    expect(f.componentInstance.from()).toBe(210);
    hi.value = '20';
    hi.dispatchEvent(new Event('input'));
    expect(f.componentInstance.to()).toBe(210);
  });
});

describe('flowOf', () => {
  it('draws only the accounts the rehearsal moves grams in, summed over its jars', () => {
    const none = { storage: 0, stash: 0, consumed: 0, avb: 0 };
    const lines = flowOf({
      entries: [],
      message: '',
      jars: [
        // An amendment's undo and its entry again: the stash 0.50 g lighter, the seal 0.50 g fuller.
        { product: 'wcake-221', name: 'Wedding Cake', before: { ...none, storage: 18.75, stash: 1.25 }, after: { ...none, storage: 19.25, stash: 0.75 } },
      ],
    });
    expect(lines.map((l) => [l.label, l.before, l.after])).toEqual([['sealed', 18.75, 19.25], ['ground', 1.25, 0.75]]);
  });
});
