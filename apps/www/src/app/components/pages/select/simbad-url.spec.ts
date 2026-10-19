import { describe, expect, it } from 'vitest';
import type { Target } from '../../../api/api.types';
import { simbadUrl } from './simbad-url';

const star = (
  catalog: string,
  id: string,
  name: string,
  ra: number,
  dec: number,
): Target => ({
  catalog,
  catalog_id: id,
  display_name: name,
  designation: `${catalog} ${id}`,
  ra_deg: ra,
  dec_deg: dec,
});

describe('simbadUrl', () => {
  it.each([
    [
      'Kepler-90',
      star('KIC', '11442793', 'Kepler-90', 284.4, 49.3),
      'KIC%2011442793',
    ],
    [
      'Kepler-186',
      star('KIC', '8120608', 'Kepler-186', 297.0, 43.9),
      'KIC%208120608',
    ],
    [
      'Kepler-452',
      star('KIC', '8311864', 'Kepler-452', 289.6, 44.3),
      'KIC%208311864',
    ],
    [
      'K2-18',
      star('EPIC', '201912552', 'K2-18', 172.6, 7.6),
      'EPIC%20201912552',
    ],
    [
      'TOI-700',
      star('TIC', '150428135', 'TOI-700', 97.1, -65.6),
      'TIC%20150428135',
    ],
  ])(
    'links %s by catalogue designation, not common name',
    (_n, target, ident) => {
      expect(simbadUrl(target)).toBe(
        `https://simbad.cds.unistra.fr/simbad/sim-id?Ident=${ident}`,
      );
    },
  );

  it('falls back to catalog and id when the designation is empty', () => {
    expect(
      simbadUrl({
        ...star('KIC', '11442793', 'Kepler-90', 0, 0),
        designation: '',
      }),
    ).toContain('Ident=KIC%2011442793');
  });
});
