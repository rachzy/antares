import { Component, computed, input } from '@angular/core';
import type { Planet, StarDetails } from '../../../api/api.types';

const BANDS = { G: 'Gaia G', V: 'V', Kepler: 'Kepler' } as const;

@Component({
  selector: 'an-star-details',
  templateUrl: './star-details.html',
  host: { class: 'flex flex-col gap-4' },
})
export class StarDetailsView {
  readonly details = input<StarDetails | null>(null);
  readonly loading = input(false);
  readonly failed = input(false);

  protected readonly facts = computed(() => {
    const d = this.details();
    if (!d) return [];
    const rows: [string, string][] = [];
    if (d.distance_ly != null)
      rows.push([
        'Distance',
        `${Math.round(d.distance_ly).toLocaleString('en')} ly`,
      ]);
    if (d.teff_k != null)
      rows.push([
        'Temperature',
        `${Math.round(d.teff_k).toLocaleString('en')} K`,
      ]);
    if (d.radius_solar != null)
      rows.push(['Radius', `${d.radius_solar.toFixed(2)} R☉`]);
    if (d.mass_solar != null)
      rows.push(['Mass', `${d.mass_solar.toFixed(2)} M☉`]);
    if (d.magnitude != null)
      rows.push([
        `Magnitude (${BANDS[d.magnitude_band ?? 'V']})`,
        d.magnitude.toFixed(1),
      ]);
    return rows;
  });

  protected readonly chips = computed(() => {
    const d = this.details();
    if (!d) return [];
    return [d.spectral_type, ...d.aliases.slice(0, 4)].filter(
      (c): c is string => !!c,
    );
  });

  protected readonly confirmed = computed(
    () =>
      this.details()?.planets.filter((p) => p.confirmation === 'confirmed')
        .length ?? 0,
  );
  protected readonly candidates = computed(
    () =>
      this.details()?.planets.filter((p) => p.confirmation === 'candidate')
        .length ?? 0,
  );

  protected readonly empty = computed(() => {
    const d = this.details();
    return !!d && !d.sources.length;
  });

  protected period(p: Planet): string {
    return p.period_days != null ? p.period_days.toFixed(2) : '–';
  }

  protected radius(p: Planet): string {
    return p.radius_earth != null ? p.radius_earth.toFixed(1) : '–';
  }
}
