import { TestBed } from '@angular/core/testing';
import type { PredictionRow } from '../../api/api.types';
import { heat, modelNames, rank } from '../../utils/scoring';
import { ScoreMatrix } from './score-matrix';

const rows: PredictionRow[] = [
  {
    period_days: 129.9,
    prob_lightgbm: 0.68,
    prob_catboost: 0.77,
    prob_extra_trees: null,
    prob_stack: 0.71,
    potential_probability: 0.71,
    decision_threshold: 0.5,
    prediction: 'POTENTIAL',
  },
  {
    period_days: 4.82,
    prob_lightgbm: 0.41,
    prob_catboost: 0.47,
    prob_extra_trees: 0.39,
    prob_stack: 0.44,
    potential_probability: 0.44,
    decision_threshold: 0.5,
    prediction: 'UNLIKELY',
  },
];

describe('ScoreMatrix', () => {
  const mount = async (data: PredictionRow[]) => {
    const fixture = TestBed.createComponent(ScoreMatrix);
    fixture.componentRef.setInput('rows', rank(data));
    fixture.componentRef.setInput('models', modelNames(data));
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const heads = () =>
      Array.from(el.querySelectorAll('thead th')).map((h) =>
        h.textContent?.trim(),
      );
    return { el, heads };
  };

  it('has one column per model plus stack and the decision', async () => {
    const { heads } = await mount(rows);
    expect(heads()).toEqual([
      'Candidate',
      'lightgbm',
      'catboost',
      'extra_trees',
      'stack',
      'Decision',
    ]);
  });

  it('shades each cell with the primary color at heat opacity', async () => {
    const { el } = await mount(rows);
    const cells = Array.from(
      el.querySelectorAll<HTMLElement>('tbody tr:first-child [data-cell]'),
    );
    expect(cells.map((c) => c.textContent?.trim())).toEqual([
      '0.68',
      '0.77',
      '—',
      '0.71',
    ]);
    const shade = cells[0].querySelector<HTMLElement>('[data-heat]');
    expect(shade?.classList).toContain('bg-primary');
    expect(Number(shade?.style.opacity)).toBeCloseTo(heat(0.68));
    expect(cells[2].querySelector('[data-heat]')).toBeNull();
    const tags = Array.from(el.querySelectorAll('tbody [data-tag]')).map((t) =>
      t.textContent?.trim(),
    );
    expect(tags).toEqual(['POTENTIAL', 'UNLIKELY']);
  });

  it('has no model columns for rows without prob_ keys', async () => {
    const { heads } = await mount([
      { period_days: 1, potential_probability: 0.2 },
    ]);
    expect(heads()).toEqual(['Candidate', 'stack', 'Decision']);
  });
});
