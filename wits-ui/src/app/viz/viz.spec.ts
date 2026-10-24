import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { FillRing } from './fill-ring';
import { GlassJar } from './glass-jar';

describe('GlassJar', () => {
  function jar(capacity: number, layers: { grams: number; color: string; label: string }[]) {
    const f = TestBed.createComponent(GlassJar);
    f.componentRef.setInput('capacity', capacity);
    f.componentRef.setInput('layers', layers);
    f.detectChanges();
    return f;
  }

  it('stacks the layers from the bottom, each as tall as its share', () => {
    const f = jar(20, [
      { grams: 10, color: 'blue', label: 'sealed' },
      { grams: 5, color: 'green', label: 'ground' },
    ]);
    const bands = f.nativeElement.querySelectorAll('rect.band');
    expect(bands.length).toBe(2);
    const sealed = Number(bands[0].getAttribute('height'));
    const ground = Number(bands[1].getAttribute('height'));
    expect(sealed - 1).toBeCloseTo(2 * (ground - 1), 5);
    expect(Number(bands[1].getAttribute('y'))).toBeLessThan(Number(bands[0].getAttribute('y')));
  });

  it('never fills past the brim, whatever the layers say', () => {
    const f = jar(10, [{ grams: 40, color: 'blue', label: 'sealed' }]);
    const band = f.nativeElement.querySelector('rect.band');
    expect(Number(band.getAttribute('y'))).toBeGreaterThanOrEqual(34);
  });

  it('draws no surface on an empty jar', () => {
    const f = jar(20, [{ grams: 0, color: 'blue', label: 'sealed' }]);
    expect(f.nativeElement.querySelector('path.wave')).toBeNull();
  });
});

describe('FillRing', () => {
  function ring(sealed: number, stash: number, avb: number, carried = 0) {
    const f = TestBed.createComponent(FillRing);
    f.componentRef.setInput('dispensed', 60);
    f.componentRef.setInput('sealed', sealed);
    f.componentRef.setInput('stash', stash);
    f.componentRef.setInput('avb', avb);
    f.componentRef.setInput('carried', carried);
    f.detectChanges();
    return f;
  }

  it('draws a segment for each part that has grams', () => {
    expect(ring(36.73, 23.27, 0).nativeElement.querySelectorAll('path.seg').length).toBe(2);
    expect(ring(30, 20, 10).nativeElement.querySelectorAll('path.seg').length).toBe(3);
  });

  it('says what is still sealed of what was dispensed', () => {
    const svg = ring(36.73, 23.27, 0).nativeElement.querySelector('svg');
    expect(svg.getAttribute('aria-label')).toBe('36.73 grams of 60 still sealed');
  });

  it('draws the older jars apart, as a halo, only when there are any', () => {
    expect(ring(36.73, 23.27, 0).nativeElement.querySelector('path.halo')).toBeNull();
    expect(ring(36.73, 23.27, 0, 8.07).nativeElement.querySelector('path.halo')).not.toBeNull();
  });
});
