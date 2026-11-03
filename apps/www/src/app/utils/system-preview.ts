import type { PlanetInput, SystemInput } from '@larawag/core';
import type { StarDetails, Target } from '../api/api.types';
import type { ScoredRow } from './scoring';

export interface PreviewPlanet {
  scored: ScoredRow;
  radiusKnown: boolean;
}

export interface SystemPreview {
  system: SystemInput;
  planets: PreviewPlanet[];
}

const positive = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) && value > 0
    ? value
    : undefined;

// Placeholder drawn when Shaula had no stellar radius to turn the depth into a planet radius.
const UNKNOWN_RADIUS_EARTH = 2;

function phase(t0: unknown, period: number): number {
  if (typeof t0 !== 'number' || !Number.isFinite(t0)) return 0;
  const turn = 2 * Math.PI;
  return ((((-turn * t0) / period) % turn) + turn) % turn;
}

/**
 * Builds the Larawag input for a scored target. Candidates are ordered by period and lettered
 * b, c, d… with their probability; values Shaula already derived are passed as overrides.
 */
export function toSystemPreview(
  target: Target,
  details: StarDetails | null,
  rows: ScoredRow[],
  includeUnlikely: boolean,
): SystemPreview {
  const shown = rows
    .filter((r) => Number.isFinite(r.period) && r.period > 0)
    .filter(
      (r) =>
        includeUnlikely ||
        (r.decision === 'POTENTIAL' &&
          r.row['detection_status'] !== 'provisional'),
    )
    .sort((a, b) => a.period - b.period);

  const planets: PreviewPlanet[] = [];
  const inputs: PlanetInput[] = shown.map((scored, i) => {
    const { row } = scored;
    const radius = positive(row['planet_radius_rearth']);
    planets.push({ scored, radiusKnown: radius !== undefined });
    const planet: PlanetInput = {
      name: letter(i, scored.probability),
      period: scored.period,
      radius: radius ?? UNKNOWN_RADIUS_EARTH,
      phase: phase(row['t0'], scored.period),
    };
    const a = positive(row['semi_major_axis_au']);
    const teq = positive(row['equilibrium_temp_k']);
    const insolation = positive(row['insolation_earth']);
    const velocity = positive(row['orbital_speed_kms']);
    if (a !== undefined) planet.a = a;
    if (teq !== undefined) planet.teq = teq;
    if (insolation !== undefined) planet.insolation = insolation;
    if (velocity !== undefined) planet.velocity = velocity;
    return planet;
  });

  return {
    system: {
      star: {
        name: target.display_name || target.designation,
        teff: positive(details?.teff_k),
        radius: positive(details?.radius_solar),
        mass: positive(details?.mass_solar),
        spectralType: details?.spectral_type,
      },
      planets: inputs,
    },
    planets,
  };
}

function letter(index: number, probability: number): string {
  const name =
    index < 25 ? String.fromCharCode(98 + index) : `planet ${index + 1}`;
  return Number.isFinite(probability)
    ? `${name} · ${Math.round(probability * 100)}%`
    : name;
}

/** True when Larawag can build a star from these details (explicit values or a spectral type). */
export function hasStarData(details: StarDetails | null): boolean {
  if (!details) return false;
  const explicit =
    positive(details.teff_k) &&
    positive(details.radius_solar) &&
    positive(details.mass_solar);
  return Boolean(explicit || details.spectral_type);
}
