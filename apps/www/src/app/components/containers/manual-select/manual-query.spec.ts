import { manualQuery } from './manual-query';

describe('manualQuery', () => {
  it('uses the trimmed name when one is given', () => {
    expect(manualQuery('  Kepler-186 ', '', '')).toEqual({
      query: 'Kepler-186',
    });
  });

  it('prefers the name over coordinates', () => {
    expect(manualQuery('Kepler-90', '10', '20')).toEqual({
      query: 'Kepler-90',
    });
  });

  it('accepts decimal degrees', () => {
    expect(manualQuery('', '298.65', '43.95')).toEqual({
      query: '298.650000 +43.950000',
    });
  });

  it('converts sexagesimal RA from hours and keeps the Dec sign', () => {
    expect(manualQuery('', '19h 54m 36.7s', '-43° 57′ 18″')).toEqual({
      query: '298.652917 -43.955000',
    });
  });

  it('keeps the sign of a Dec between -1 and 0 degrees', () => {
    expect(manualQuery('', '10', '-0 30 00')).toEqual({
      query: '10.000000 -0.500000',
    });
  });

  it('asks for something when the form is empty', () => {
    expect(manualQuery(' ', '', '')).toEqual({
      error: 'Enter a star name or both coordinates.',
    });
  });

  it('asks for the missing coordinate', () => {
    expect(manualQuery('', '10', '')).toEqual({
      error: 'Enter both RA and Dec.',
    });
    expect(manualQuery('', '', '10')).toEqual({
      error: 'Enter both RA and Dec.',
    });
  });

  it('rejects text that is not a coordinate', () => {
    expect(manualQuery('', 'abc', '10')).toEqual({
      error:
        'Use degrees (298.65) or sexagesimal (19h 54m 36.7s) for the coordinates.',
    });
  });

  it('rejects out-of-range coordinates', () => {
    const error = { error: 'RA must be 0–360° and Dec −90° to +90°.' };
    expect(manualQuery('', '360', '10')).toEqual(error);
    expect(manualQuery('', '-1', '10')).toEqual(error);
    expect(manualQuery('', '10', '91')).toEqual(error);
  });
});
