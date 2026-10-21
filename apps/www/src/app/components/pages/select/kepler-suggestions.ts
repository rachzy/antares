import { KEPLER_STARS } from './kepler-stars';

export const MAX_SUGGESTIONS = 100;

/** Kepler stars whose name starts with the query, ignoring case and surrounding whitespace. */
export function suggestKeplerStars(query: string): string[] {
  const prefix = query.trim().toLowerCase();
  return KEPLER_STARS.filter((name) =>
    name.toLowerCase().startsWith(prefix),
  ).slice(0, MAX_SUGGESTIONS);
}
