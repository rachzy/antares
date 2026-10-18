import { csvFilename, featuresToCsv } from './features-csv';

describe('featuresToCsv', () => {
  it('uses the union of keys in first-seen order as the header', () => {
    expect(
      featuresToCsv([
        { a: 1, b: 2 },
        { b: 3, c: 4 },
      ]),
    ).toBe('a,b,c\r\n1,2,\r\n,3,4\r\n');
  });

  it('renders null and undefined as empty cells', () => {
    expect(featuresToCsv([{ a: null, b: undefined, c: 0 }])).toBe(
      'a,b,c\r\n,,0\r\n',
    );
  });

  it('escapes commas, quotes and newlines', () => {
    expect(featuresToCsv([{ x: 'a,b', y: 'say "hi"', z: 'one\ntwo' }])).toBe(
      'x,y,z\r\n"a,b","say ""hi""","one\ntwo"\r\n',
    );
  });

  it('escapes the header too', () => {
    expect(featuresToCsv([{ 'a,b': 1 }])).toBe('"a,b"\r\n1\r\n');
  });

  it('returns an empty string for no rows', () => {
    expect(featuresToCsv([])).toBe('');
  });
});

describe('csvFilename', () => {
  const at = { ra_deg: 298.65272, dec_deg: -3.9 };

  it('uses the star name', () => {
    expect(csvFilename({ display_name: 'Kepler-186', ...at })).toBe(
      'Kepler-186-predictions.csv',
    );
  });

  it('makes names safe for a file', () => {
    expect(csvFilename({ display_name: 'KIC 8120608 / b', ...at })).toBe(
      'KIC-8120608-b-predictions.csv',
    );
  });

  it('falls back to the coordinates when the name is empty', () => {
    expect(csvFilename({ display_name: ' ', ...at })).toBe(
      '298.65272_-3.90000-predictions.csv',
    );
  });

  it('falls back to a bare name without a star', () => {
    expect(csvFilename(null)).toBe('predictions.csv');
  });
});
