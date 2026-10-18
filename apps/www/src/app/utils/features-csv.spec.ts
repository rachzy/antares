import { featuresToCsv } from './features-csv';

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
