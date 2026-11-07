import { TestBed } from '@angular/core/testing';
import type { StarDetails, Target } from '../../api/api.types';
import { rank } from '../../utils/scoring';
import { SystemPreview } from './system-preview';

const target: Target = {
  catalog: 'KIC',
  catalog_id: '1',
  display_name: 'Kepler-1',
  designation: 'KIC 1',
  ra_deg: 0,
  dec_deg: 0,
};
const star: StarDetails = {
  aliases: [],
  teff_k: 5100,
  radius_solar: 0.8,
  mass_solar: 0.85,
  planets: [],
  sources: [],
};
const rows = rank([
  {
    period_days: 4.2,
    planet_radius_rearth: 1.5,
    potential_probability: 0.9,
    decision_threshold: 0.5,
    prediction: 'POTENTIAL',
  },
]);

describe('SystemPreview', () => {
  const mount = async (details: StarDetails | null, scored = rows) => {
    const fixture = TestBed.createComponent(SystemPreview);
    fixture.componentRef.setInput('target', target);
    fixture.componentRef.setInput('details', details);
    fixture.componentRef.setInput('rows', scored);
    await fixture.whenStable();
    return fixture.nativeElement as HTMLElement;
  };

  it('mounts the Larawag view when the star and candidates are known', async () => {
    const el = await mount(star);
    expect(el.querySelector('larawag-star-system')).not.toBeNull();
    expect(el.querySelector('[data-empty]')).toBeNull();
  });

  it('explains when the star has too little data', async () => {
    const el = await mount(null);
    expect(el.querySelector('larawag-star-system')).toBeNull();
    expect(el.querySelector('[data-empty]')?.textContent).toContain(
      'not enough data',
    );
  });

  it('explains when no candidate is likely enough to show', async () => {
    const el = await mount(star, []);
    expect(el.querySelector('[data-empty]')?.textContent).toContain(
      'No candidates',
    );
  });

  it('labels the reset button Reset view', async () => {
    const el = await mount(star);
    const labels = Array.from(el.querySelectorAll('button')).map((b) =>
      b.textContent?.trim(),
    );
    expect(labels).toContain('Reset view');
    expect(labels).not.toContain('Home');
  });

  it('collapses, expands, maximizes and restores through its buttons', async () => {
    const fixture = TestBed.createComponent(SystemPreview);
    fixture.componentRef.setInput('target', target);
    fixture.componentRef.setInput('details', star);
    fixture.componentRef.setInput('rows', rows);
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const click = async (label: string) => {
      el.querySelector<HTMLButtonElement>(`[aria-label="${label}"]`)?.click();
      await fixture.whenStable();
    };
    expect(fixture.componentInstance.mode()).toBe('expanded');
    await click('Maximize system preview');
    expect(fixture.componentInstance.mode()).toBe('maximized');
    expect(el.querySelector('[data-canvas]')?.classList).toContain('flex-1');
    await click('Restore system preview');
    expect(fixture.componentInstance.mode()).toBe('expanded');
    await click('Collapse system preview');
    expect(fixture.componentInstance.mode()).toBe('collapsed');
    expect(el.querySelector('[data-body]')?.hasAttribute('inert')).toBe(true);
    await click('Expand system preview');
    expect(fixture.componentInstance.mode()).toBe('expanded');
    expect(el.querySelector('[data-body]')?.hasAttribute('inert')).toBe(false);
  });
});
