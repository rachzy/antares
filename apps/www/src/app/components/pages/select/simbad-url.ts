import type { Target } from '../../../api/api.types';

export function simbadUrl(star: Target): string {
  const designation = star.designation || `${star.catalog} ${star.catalog_id}`;
  return `https://simbad.cds.unistra.fr/simbad/sim-id?Ident=${encodeURIComponent(designation)}`;
}
