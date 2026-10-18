import type { PredictionRow } from '../api/api.types';

export interface ScoredRow {
  row: PredictionRow;
  period: number;
  probability: number;
  threshold: number;
  decision: 'POTENTIAL' | 'UNLIKELY';
  models: Record<string, number | null>;
}

const PROB = /^prob_(.+)$/;
const OUTPUTS = new Set([
  'potential_probability',
  'decision_threshold',
  'prediction',
  'model_name',
  'model_run_id',
  'star_id',
]);

const num = (value: unknown): number =>
  typeof value === 'number' && Number.isFinite(value) ? value : NaN;

const unique = (rows: PredictionRow[], pick: (key: string) => string | null) =>
  [
    ...new Set(
      rows.flatMap((r) => Object.keys(r).map(pick)).filter((k) => k !== null),
    ),
  ] as string[];

export function modelNames(rows: PredictionRow[]): string[] {
  return unique(rows, (key) => {
    const name = PROB.exec(key)?.[1];
    return name && name !== 'stack' ? name : null;
  });
}

export function featureNames(rows: PredictionRow[]): string[] {
  return unique(rows, (key) =>
    PROB.test(key) || OUTPUTS.has(key) ? null : key,
  );
}

export function margin(row: PredictionRow): number {
  return num(row['potential_probability']) - num(row['decision_threshold']);
}

export function heat(p: number): number {
  const clamped = Number.isFinite(p) ? Math.min(1, Math.max(0, p)) : 0;
  return 0.1 + 0.7 * clamped;
}

function score(row: PredictionRow): ScoredRow {
  const probability = num(row['potential_probability']);
  const threshold = num(row['decision_threshold']);
  const prediction = row['prediction'];
  const models: Record<string, number | null> = {};
  for (const key of Object.keys(row)) {
    const name = PROB.exec(key)?.[1];
    if (name)
      models[name] = Number.isFinite(num(row[key])) ? num(row[key]) : null;
  }
  return {
    row,
    period: num(row['period_days']),
    probability,
    threshold,
    decision:
      prediction === 'POTENTIAL' || prediction === 'UNLIKELY'
        ? prediction
        : probability >= threshold
          ? 'POTENTIAL'
          : 'UNLIKELY',
    models,
  };
}

export function rank(rows: PredictionRow[]): ScoredRow[] {
  const key = (r: ScoredRow) =>
    Number.isNaN(r.probability) ? -1 : r.probability;
  return rows.map(score).sort((a, b) => key(b) - key(a));
}

export const DASH = '—';

export function fixed(value: number | null, digits: number): string {
  return value === null || !Number.isFinite(value)
    ? DASH
    : value.toFixed(digits);
}

export function periodLabel(period: number): string {
  return Number.isFinite(period) ? `${+period.toPrecision(4)} d` : DASH;
}

export function percent(p: number): number {
  return Number.isFinite(p)
    ? Math.round(Math.min(1, Math.max(0, p)) * 1000) / 10
    : 0;
}
