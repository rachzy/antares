import { TestBed } from '@angular/core/testing';
import type { JobResult, PredictionRow } from '../../api/api.types';
import { rank } from '../../utils/scoring';
import { ScoreDetails } from './score-details';

const features = (i: number): PredictionRow => ({
  period_days: i === 0 ? 129.9 : 4.82,
  duration_hours: 5.8,
  MES: 12.5,
  snr_global: 14.2,
  cdpp_6h: 38.1,
  secondary_depth_snr: 0.4,
  vshape_metric: i === 0 ? null : 0.62,
  odd_even_ratio: 1.1,
  depth_ppm: 410,
});

const row = (i: number, p: number): PredictionRow => ({
  ...features(i),
  prob_lightgbm: 0.68,
  prob_catboost: 0.77,
  prob_extra_trees: 0.63,
  prob_stack: p,
  potential_probability: p,
  decision_threshold: 0.5,
  prediction: p >= 0.5 ? 'POTENTIAL' : 'UNLIKELY',
  model_name: 'stack',
  model_run_id: '20261001T120000Z-abcd1234',
  star_id: 'KIC 8120608',
});

const predictions = [row(1, 0.44), row(0, 0.71)];

const result = (cached: boolean): JobResult => ({
  target: {
    catalog: 'KIC',
    catalog_id: '8120608',
    display_name: 'Kepler-186',
    designation: 'KIC 8120608',
    ra_deg: 0,
    dec_deg: 0,
  },
  cached,
  shaula_version: '1',
  features: [],
  model_run_id: '20261001T120000Z-abcd1234',
  schema_version: 1,
  predictions,
});

const text = (el: Element | null | undefined): string | undefined =>
  el?.children.length
    ? Array.from(el.children)
        .map(text)
        .filter((t) => t)
        .join(' ')
    : el?.textContent?.replace(/\s+/g, ' ').trim();

describe('ScoreDetails', () => {
  const mount = async (cached = true) => {
    const fixture = TestBed.createComponent(ScoreDetails);
    fixture.componentRef.setInput('rows', rank(predictions));
    fixture.componentRef.setInput('result', result(cached));
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const table = (name: string) =>
      el.querySelector<HTMLTableElement>(`[data-table="${name}"]`);
    return { fixture, el, table };
  };

  it('shows the run chips', async () => {
    const { el } = await mount();
    const chips = Array.from(el.querySelectorAll('[data-chip]')).map(text);
    expect(chips).toEqual([
      'Model stack',
      'Threshold 0.50',
      'Run 20261001T120000Z-abcd1234',
      'Schema v1',
      'Features from cache',
    ]);
  });

  it('says fresh when the features were not cached', async () => {
    const { el } = await mount(false);
    expect(text(el.querySelectorAll('[data-chip]')[4])).toBe('Features fresh');
  });

  it('shows the decision table with a signed margin', async () => {
    const { table } = await mount();
    const rows = Array.from(
      table('decision')?.querySelectorAll('tbody tr') ?? [],
    );
    expect(text(table('decision')?.querySelector('thead'))).toContain(
      'Score (stack)',
    );
    expect(text(rows[0])).toBe(
      '129.9 d KIC 8120608 0.710 0.500 +0.210 POTENTIAL',
    );
    expect(text(rows[1])).toBe(
      '4.82 d KIC 8120608 0.440 0.500 −0.060 UNLIKELY',
    );
  });

  it('only gives the decision model row the primary bar', async () => {
    const { table } = await mount();
    const rows = Array.from(
      table('models')?.querySelectorAll('tbody tr') ?? [],
    );
    expect(rows).toHaveLength(8);
    const bars = rows.map((r) => r.querySelector<HTMLElement>('[data-fill]'));
    const primary = rows.filter((_, i) =>
      bars[i]?.classList.contains('bg-primary'),
    );
    expect(primary.map(text)).toEqual([
      'stack 0.71 Used for decision',
      'stack 0.44 Used for decision',
    ]);
    const muted = bars.filter((b) =>
      b?.classList.contains('bg-muted-foreground'),
    );
    expect(muted).toHaveLength(6);
    expect(rows[0].querySelector<HTMLElement>('[data-tick]')?.style.left).toBe(
      '50%',
    );
  });

  it('shows the first 7 features with a dash for null and a Show all toggle', async () => {
    const { fixture, el, table } = await mount();
    const body = () =>
      Array.from(table('features')?.querySelectorAll('tbody tr') ?? []);
    expect(body()).toHaveLength(7);
    expect(text(body()[6])).toBe('vshape_metric — 0.62');
    const toggle = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Show all'),
    );
    expect(toggle).toBeDefined();
    toggle?.click();
    await fixture.whenStable();
    expect(body()).toHaveLength(9);
    expect(
      Array.from(el.querySelectorAll('button')).some((b) =>
        b.textContent?.includes('Show fewer'),
      ),
    ).toBe(true);
  });
});
