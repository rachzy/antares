import type { PredictionRow } from '../api/api.types';
import {
  DASH,
  decisionModel,
  featureNames,
  fixed,
  heat,
  margin,
  modelNames,
  modelScore,
  percent,
  periodLabel,
  rank,
  scoreColumns,
  type ScoredRow,
} from './scoring';

const row = (p: number | null, extra: PredictionRow = {}): PredictionRow => ({
  period_days: 10,
  MES: 9.1,
  prob_lightgbm: 0.4,
  prob_catboost: 0.5,
  prob_extra_trees: 0.3,
  prob_stack: p,
  potential_probability: p,
  decision_threshold: 0.5,
  prediction: p !== null && p >= 0.5 ? 'POTENTIAL' : 'UNLIKELY',
  model_name: 'stack',
  model_run_id: 'run-1',
  star_id: 'KIC 1',
  ...extra,
});

describe('modelNames', () => {
  it('returns the base models in first-seen order, without stack', () => {
    expect(modelNames([row(0.4)])).toEqual([
      'lightgbm',
      'catboost',
      'extra_trees',
    ]);
  });

  it('collects models across rows', () => {
    expect(modelNames([{ prob_a: 1 }, { prob_b: 1, prob_a: 1 }])).toEqual([
      'a',
      'b',
    ]);
  });

  it('returns no models for rows without prob_ keys', () => {
    expect(
      modelNames([{ period_days: 1, potential_probability: 0.3 }]),
    ).toEqual([]);
  });
});

describe('rank', () => {
  it('sorts by potential_probability descending', () => {
    const ranked = rank([row(0.44), row(0.71), row(0.12)]);
    expect(ranked.map((r) => r.probability)).toEqual([0.71, 0.44, 0.12]);
  });

  it('maps the row into a ScoredRow', () => {
    const source = row(0.71, { period_days: 129.9 });
    const [scored] = rank([source]);
    expect(scored).toEqual<ScoredRow>({
      row: source,
      period: 129.9,
      probability: 0.71,
      threshold: 0.5,
      decision: 'POTENTIAL',
      models: {
        lightgbm: 0.4,
        catboost: 0.5,
        extra_trees: 0.3,
        stack: 0.71,
      },
    });
  });

  it('sorts rows with a missing or null probability last', () => {
    const ranked = rank([row(null), { period_days: 3 }, row(0.2), row(0.9)]);
    expect(ranked.map((r) => r.probability)).toEqual([0.9, 0.2, NaN, NaN]);
  });

  it('keeps null model probabilities as null', () => {
    const [scored] = rank([row(0.3, { prob_catboost: null })]);
    expect(scored.models['catboost']).toBeNull();
  });

  it('falls back to the threshold when prediction is missing', () => {
    const [a, b] = rank([
      row(0.6, { prediction: null }),
      row(0.4, { prediction: null }),
    ]);
    expect(a.decision).toBe('POTENTIAL');
    expect(b.decision).toBe('UNLIKELY');
  });
});

describe('margin', () => {
  it('is the probability minus the threshold', () => {
    expect(margin(row(0.44))).toBeCloseTo(-0.06, 10);
    expect(margin(row(0.71))).toBeCloseTo(0.21, 10);
  });

  it('is NaN when a value is missing', () => {
    expect(margin({ potential_probability: 0.4 })).toBeNaN();
  });
});

describe('heat', () => {
  it('maps a probability onto 0.1 to 0.8 opacity', () => {
    expect(heat(0)).toBeCloseTo(0.1);
    expect(heat(1)).toBeCloseTo(0.8);
    expect(heat(0.5)).toBeCloseTo(0.45);
  });

  it('clamps out of range and missing values', () => {
    expect(heat(-1)).toBeCloseTo(0.1);
    expect(heat(2)).toBeCloseTo(0.8);
    expect(heat(NaN)).toBeCloseTo(0.1);
  });
});

describe('featureNames', () => {
  it('returns the input feature columns only', () => {
    expect(featureNames([row(0.4, { snr_global: null })])).toEqual([
      'period_days',
      'MES',
      'snr_global',
    ]);
  });
});

describe('formatting', () => {
  it('labels a period in days', () => {
    expect(periodLabel(129.9)).toBe('129.9 d');
    expect(periodLabel(4.8213)).toBe('4.821 d');
    expect(periodLabel(NaN)).toBe(DASH);
  });

  it('formats fixed values and missing ones as a dash', () => {
    expect(fixed(0.71, 3)).toBe('0.710');
    expect(fixed(null, 2)).toBe(DASH);
    expect(fixed(NaN, 2)).toBe(DASH);
  });

  it('turns a probability into a bar percentage', () => {
    expect(percent(0.57)).toBe(57);
    expect(percent(NaN)).toBe(0);
    expect(percent(1.4)).toBe(100);
  });
});

describe('decision model columns', () => {
  it('names the decision model from model_name, defaulting to stack', () => {
    expect(decisionModel([row(0.4, { model_name: 'lightgbm' })])).toBe(
      'lightgbm',
    );
    expect(decisionModel([{ potential_probability: 0.4 }])).toBe('stack');
  });

  it('lists base models plus stack when present', () => {
    expect(scoreColumns([row(0.4, { model_name: 'lightgbm' })])).toEqual([
      'lightgbm',
      'catboost',
      'extra_trees',
      'stack',
    ]);
  });

  it('drops stack when no row has prob_stack', () => {
    const noStack = row(0.4, { model_name: 'lightgbm' });
    delete noStack['prob_stack'];
    expect(scoreColumns([noStack])).toEqual([
      'lightgbm',
      'catboost',
      'extra_trees',
    ]);
  });

  it('adds the decision model when it has no prob_ column', () => {
    expect(
      scoreColumns([{ prob_lightgbm: 0.3, potential_probability: 0.4 }]),
    ).toEqual(['lightgbm', 'stack']);
  });

  it('falls back to potential_probability for the decision model only', () => {
    const [scored] = rank([
      { prob_lightgbm: null, potential_probability: 0.4 },
    ]);
    expect(modelScore(scored, 'stack', 'stack')).toBe(0.4);
    expect(modelScore(scored, 'lightgbm', 'stack')).toBeNaN();
    expect(modelScore(scored, 'catboost', 'stack')).toBeNaN();
  });
});
