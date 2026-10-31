import { manualLookup } from './manual-query';

describe('manualLookup', () => {
  it('uses the trimmed name when one is given', () => {
    expect(manualLookup('  Kepler-186 ', '', '')).toEqual({
      query: 'Kepler-186',
    });
  });

  it('prefers the name over coordinates', () => {
    expect(manualLookup('Kepler-90', '10', '20')).toEqual({
      query: 'Kepler-90',
    });
  });

  it('accepts decimal degrees', () => {
    expect(manualLookup('', '298.65', '43.95')).toEqual({
      ra: 298.65,
      dec: 43.95,
    });
  });

  it('converts sexagesimal RA from hours and keeps the Dec sign', () => {
    const result = manualLookup('', '19h 54m 36.7s', '-43° 57′ 18″');
    expect(result).toMatchObject({ ra: expect.any(Number) });
    const { ra, dec } = result as { ra: number; dec: number };
    expect(ra).toBeCloseTo(298.652917, 5);
    expect(dec).toBeCloseTo(-43.955, 5);
  });

  it('keeps the sign of a Dec between -1 and 0 degrees', () => {
    expect(manualLookup('', '10', '-0 30 00')).toEqual({ ra: 10, dec: -0.5 });
  });

  it('asks for something when the form is empty', () => {
    expect(manualLookup(' ', '', '')).toEqual({
      error: 'Enter a star name or both coordinates.',
    });
  });

  it('asks for the missing coordinate', () => {
    expect(manualLookup('', '10', '')).toEqual({
      error: 'Enter both RA and Dec.',
    });
    expect(manualLookup('', '', '10')).toEqual({
      error: 'Enter both RA and Dec.',
    });
  });

  it('rejects text that is not a coordinate', () => {
    expect(manualLookup('', 'abc', '10')).toEqual({
      error:
        'Use degrees (298.65) or sexagesimal (19h 54m 36.7s) for the coordinates.',
    });
  });

  it('rejects out-of-range coordinates', () => {
    const error = { error: 'RA must be 0–360° and Dec −90° to +90°.' };
    expect(manualLookup('', '360', '10')).toEqual(error);
    expect(manualLookup('', '-1', '10')).toEqual(error);
    expect(manualLookup('', '10', '91')).toEqual(error);
  });
});
