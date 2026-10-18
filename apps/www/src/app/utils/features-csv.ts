import type { FeatureRow } from '../api/api.types';

const cell = (value: unknown): string => {
  if (value === null || value === undefined) return '';
  const text =
    typeof value === 'object' ? JSON.stringify(value) : String(value);
  return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
};

export function featuresToCsv(rows: FeatureRow[]): string {
  if (!rows.length) return '';
  const keys = [...new Set(rows.flatMap((row) => Object.keys(row)))];
  return [keys, ...rows.map((row) => keys.map((k) => row[k]))]
    .map((line) => line.map(cell).join(',') + '\r\n')
    .join('');
}

export function csvFilename(
  star: { display_name?: string; ra_deg: number; dec_deg: number } | null,
): string {
  const name = star?.display_name?.trim();
  const label = name
    ? name.replace(/[^\p{L}\p{N}._-]+/gu, '-').replace(/^-+|-+$/g, '')
    : star
      ? `${star.ra_deg.toFixed(5)}_${star.dec_deg.toFixed(5)}`
      : '';
  return `${label ? `${label}-` : ''}predictions.csv`;
}
