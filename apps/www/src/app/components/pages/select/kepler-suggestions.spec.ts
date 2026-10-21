import { KEPLER_STARS } from './kepler-stars';
import { MAX_SUGGESTIONS, suggestKeplerStars } from './kepler-suggestions';

describe('suggestKeplerStars', () => {
  it('prefix-matches without matching mid-name', () => {
    const names = suggestKeplerStars('Kepler-18');
    expect(names).toContain('Kepler-186');
    expect(names).toContain('Kepler-1866');
    expect(names.every((n) => n.startsWith('Kepler-18'))).toBe(true);
    expect(names).not.toContain('Kepler-118');
  });

  it('ignores case and surrounding whitespace', () => {
    expect(suggestKeplerStars('  kepler-186 ')).toContain('Kepler-186');
  });

  it('caps an empty query and returns nothing for other stars', () => {
    expect(suggestKeplerStars('')).toHaveLength(
      Math.min(MAX_SUGGESTIONS, KEPLER_STARS.length),
    );
    expect(suggestKeplerStars('Vega')).toEqual([]);
  });
});
