import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import type { StarDetails } from '../../../api/api.types';
import { StarDetailsView } from './star-details';

const kepler90: StarDetails = {
  aliases: ['Kepler-90', 'KOI-351', 'HD 175692'],
  spectral_type: 'G0V',
  distance_ly: 2717.4,
  teff_k: 6080,
  radius_solar: 1.2,
  mass_solar: 1.1,
  magnitude: 13.9,
  magnitude_band: 'G',
  planets: [
    {
      name: 'Kepler-90 b',
      period_days: 7.008,
      radius_earth: 1.3,
      confirmation: 'confirmed',
    },
    { name: 'K00351.99', confirmation: 'candidate' },
  ],
  sources: ['SIMBAD', 'NASA Exoplanet Archive'],
};

@Component({
  imports: [StarDetailsView],
  template: `<an-star-details
    [details]="details()"
    [loading]="loading()"
    [failed]="failed()"
  />`,
})
class Host {
  readonly details = signal<StarDetails | null>(null);
  readonly loading = signal(false);
  readonly failed = signal(false);
}

describe('StarDetailsView', () => {
  const mount = async (init: (h: Host) => void) => {
    const fixture = TestBed.createComponent(Host);
    init(fixture.componentInstance);
    await fixture.whenStable();
    return fixture.nativeElement as HTMLElement;
  };

  it('shows the star facts, chips, planets and sources', async () => {
    const el = await mount((h) => h.details.set(kepler90));
    const text = el.textContent ?? '';
    expect(text).toContain('G0V');
    expect(text).toContain('KOI-351');
    expect(text).toContain('1 confirmed planet');
    expect(text).toContain('1 candidate');
    expect(text).toContain('2,717 ly');
    expect(text).toContain('6,080 K');
    expect(text).toContain('1.20 R☉');
    expect(text).toContain('Magnitude (Gaia G)');
    expect(text).toContain('Kepler-90 b');
    expect(text).toContain('7.01');
    expect(text).toContain('Sources: SIMBAD, NASA Exoplanet Archive');
    expect(el.querySelectorAll('tbody tr').length).toBe(2);
  });

  it('leaves out facts the catalogues do not have', async () => {
    const el = await mount((h) =>
      h.details.set({
        aliases: [],
        planets: [],
        sources: ['SIMBAD'],
        distance_ly: 100,
      }),
    );
    expect(el.textContent).toContain('100 ly');
    expect(el.textContent).not.toContain('Temperature');
    expect(el.querySelector('table')).toBeNull();
  });

  it('says so when neither service knows the star', async () => {
    const el = await mount((h) =>
      h.details.set({ aliases: [], planets: [], sources: [] }),
    );
    expect(el.textContent).toContain('have nothing on this star');
  });

  it('shows loading and failure states', async () => {
    expect((await mount((h) => h.loading.set(true))).textContent).toContain(
      'Loading star details',
    );
    expect((await mount((h) => h.failed.set(true))).textContent).toContain(
      'unavailable right now',
    );
  });
});
