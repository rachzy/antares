import { KEPLER_STARS } from './kepler-stars';

export const MAX_SUGGESTIONS = 100;

/** Kepler stars whose name starts with the query, ignoring case and surrounding whitespace. */
export function suggestKeplerStars(query: string): string[] {
  const prefix = query.trim().toLowerCase();
  return KEPLER_STARS.filter((name) =>
    name.toLowerCase().startsWith(prefix),
  ).slice(0, MAX_SUGGESTIONS);
}

const KEPLER_SET = new Set(KEPLER_STARS);

/** Whether the query is exactly a known Kepler star name. */
export function isKeplerStar(query: string): boolean {
  return KEPLER_SET.has(query.trim());
}
