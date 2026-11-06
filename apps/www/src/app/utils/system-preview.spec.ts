import { deriveSystem } from '@larawag/core/physics';
import type { StarDetails, Target } from '../api/api.types';
import { rank } from './scoring';
import { hasStarData, toSystemPreview } from './system-preview';

const target: Target = {
  catalog: 'KIC',
  catalog_id: '8120608',
  display_name: 'Kepler-186',
  designation: 'KIC 8120608',
  ra_deg: 298.3,
  dec_deg: 43.95,
};
const details: StarDetails = {
  aliases: [],
  spectral_type: 'M1V',
  teff_k: 3788,
  radius_solar: 0.52,
  mass_solar: 0.54,
  planets: [],
  sources: [],
};
const scored = (extra: Record<string, unknown>, p = 0.9) =>
  rank([
    {
      potential_probability: p,
      decision_threshold: 0.5,
      prediction: p >= 0.5 ? 'POTENTIAL' : 'UNLIKELY',
      ...extra,
    },
  ])[0];

describe('toSystemPreview', () => {
  it('orders by period, letters planets with their probability and passes Shaula values as overrides', () => {
    const rows = [
      scored(
        {
          period_days: 22.4,
          planet_radius_rearth: 1.27,
          t0: 15,
          semi_major_axis_au: 0.109,
        },
        0.98,
      ),
      scored(
        {
          period_days: 3.9,
          planet_radius_rearth: 1.07,
          t0: 2,
          equilibrium_temp_k: 666,
        },
        0.97,
      ),
    ];
    const { system, planets } = toSystemPreview(target, details, rows, false);
    expect(system.planets.map((p) => p.name)).toEqual(['b · 97%', 'c · 98%']);
    expect(system.planets[0].teq).toBe(666);
    expect(system.planets[1].a).toBe(0.109);
    expect(planets[1].scored).toBe(rows[0]);
    expect(deriveSystem(system).planets).toHaveLength(2);
  });

  it('hides unlikely and provisional candidates unless asked', () => {
    const rows = [
      scored({ period_days: 5, planet_radius_rearth: 1 }, 0.9),
      scored({ period_days: 6, planet_radius_rearth: 1 }, 0.2),
      scored(
        {
          period_days: 7,
          planet_radius_rearth: 1,
          detection_status: 'provisional',
        },
        0.9,
      ),
    ];
    expect(toSystemPreview(target, details, rows, false).planets).toHaveLength(
      1,
    );
    expect(toSystemPreview(target, details, rows, true).planets).toHaveLength(
      3,
    );
  });

  it('draws a placeholder radius when Shaula had none and marks it unknown', () => {
    const { system, planets } = toSystemPreview(
      target,
      details,
      [scored({ period_days: 5, planet_radius_rearth: NaN })],
      false,
    );
    expect(system.planets[0].radius).toBe(2);
    expect(planets[0].radiusKnown).toBe(false);
  });

  it('builds a valid star from the spectral type alone', () => {
    const sparse: StarDetails = {
      ...details,
      teff_k: undefined,
      radius_solar: undefined,
      mass_solar: undefined,
    };
    const { system } = toSystemPreview(
      target,
      sparse,
      [scored({ period_days: 5, planet_radius_rearth: 1 })],
      false,
    );
    expect(deriveSystem(system).star.teff).toBeGreaterThan(0);
    expect(hasStarData(sparse)).toBe(true);
    expect(hasStarData({ ...sparse, spectral_type: undefined })).toBe(false);
    expect(hasStarData(null)).toBe(false);
  });
});
