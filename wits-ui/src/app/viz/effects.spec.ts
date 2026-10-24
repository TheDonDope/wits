import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { EffectGroup } from '../data/models';
import { DayWheel } from './day-wheel';
import { EffectMatrix, scoreColor } from './effect-matrix';
import { ScoreDots } from './score-dots';

const groups: EffectGroup[] = [
  { key: 'a', label: 'evening', count: 3, means: { overall: 4.5, sleep: 5 }, counts: { overall: 3, sleep: 2 } },
  { key: 'b', label: 'morning', count: 1, means: { overall: 2 }, counts: { overall: 1 } },
];

describe('scoreColor', () => {
  it('runs from rose at 1 to mint at 5, amber between', () => {
    expect(scoreColor(1)).toBe('rgb(255 122 138)');
    expect(scoreColor(3)).toBe('rgb(255 180 84)');
    expect(scoreColor(5)).toBe('rgb(124 245 196)');
    expect(scoreColor(9)).toBe(scoreColor(5));
  });
});

describe('EffectMatrix', () => {
  it('hatches a mean that stands on one rating, and leaves unrated cells empty', () => {
    const f = TestBed.createComponent(EffectMatrix);
    f.componentRef.setInput('groups', groups);
    f.componentRef.setInput('dimensions', ['overall', 'sleep']);
    f.detectChanges();
    const el = f.nativeElement as HTMLElement;
    expect(el.querySelectorAll('.cell:not(.none)')).toHaveLength(3);
    expect(el.querySelectorAll('.cell.thin')).toHaveLength(1);
    expect(el.querySelectorAll('.cell.none')).toHaveLength(1);
    expect(el.querySelector('.row')?.textContent).toContain('evening');
  });
});

describe('DayWheel', () => {
  it('draws a wedge only for the parts of the day that were rated', () => {
    const f = TestBed.createComponent(DayWheel);
    f.componentRef.setInput('groups', groups);
    f.componentRef.setInput('dimension', 'sleep');
    f.detectChanges();
    const el = f.nativeElement as HTMLElement;
    expect(el.querySelectorAll('.wedge')).toHaveLength(1);
    expect(el.querySelector('.mean')?.textContent).toBe('5.0');
  });
});

describe('ScoreDots', () => {
  it('sets a score, and tapping it again clears it', () => {
    const f = TestBed.createComponent(ScoreDots);
    f.detectChanges();
    const dots = (f.nativeElement as HTMLElement).querySelectorAll('button');
    dots[3].click();
    f.detectChanges();
    expect(f.componentInstance.value()).toBe(4);
    expect((f.nativeElement as HTMLElement).querySelectorAll('button.on')).toHaveLength(4);
    dots[3].click();
    expect(f.componentInstance.value()).toBeNull();
  });
});
