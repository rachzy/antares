/** What the manual form asks for: a star name, or a position in degrees. */
export type ManualLookup = { query: string } | { ra: number; dec: number };
export type ManualResult = ManualLookup | { error: string };

const NUMBER = /^[+-]?\d+(\.\d+)?$/;

/** Degrees from a decimal value or a sexagesimal one (`19h 54m 36.7s`, `+43° 57′ 18″`); RA in hours is scaled to degrees. */
function toDegrees(raw: string, isRa: boolean): number {
  const text = raw
    .toLowerCase()
    .replace(/−/g, '-')
    .replace(/[hdms°′″'":]/g, ' ')
    .trim();
  if (!text) return NaN;
  const parts = text.split(/\s+/);
  if (!parts.every((p) => NUMBER.test(p))) return NaN;
  if (parts.length === 1) return Number(parts[0]);
  if (parts.length > 3) return NaN;
  const [h, m = 0, s = 0] = parts.map(Number);
  const magnitude = Math.abs(h) + m / 60 + s / 3600;
  if (isRa) return magnitude * 15;
  return text.startsWith('-') ? -magnitude : magnitude;
}

/** Reads the manual form, or says what is wrong with it. A name wins over coordinates. */
export function manualLookup(
  name: string,
  ra: string,
  dec: string,
): ManualResult {
  const star = name.trim();
  if (star) return { query: star };
  if (!ra.trim() && !dec.trim())
    return { error: 'Enter a star name or both coordinates.' };
  if (!ra.trim() || !dec.trim()) return { error: 'Enter both RA and Dec.' };
  const raDeg = toDegrees(ra, true);
  const decDeg = toDegrees(dec, false);
  if (Number.isNaN(raDeg) || Number.isNaN(decDeg))
    return {
      error:
        'Use degrees (298.65) or sexagesimal (19h 54m 36.7s) for the coordinates.',
    };
  if (raDeg < 0 || raDeg >= 360 || decDeg < -90 || decDeg > 90)
    return { error: 'RA must be 0–360° and Dec −90° to +90°.' };
  return { ra: raDeg, dec: decDeg };
}
