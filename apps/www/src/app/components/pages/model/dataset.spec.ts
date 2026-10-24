import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import type { Run, RunDataset } from '../../../api/api.types';
import { ModelStore } from '../../../stores/model.store';
import { Dataset } from './dataset';

const run: Run = {
  run_id: 'r1',
  created_at: '2026-09-01T12:00:00Z',
  schema_version: 2,
  selected_model: 'lightgbm',
  threshold: 0.5,
};

const dataset: RunDataset = {
  run_id: 'r1',
  selected_model: 'lightgbm',
  n_rows: 5200,
  n_stars: 1800,
  n_files: 1810,
  n_features: 36,
  n_positive: 640,
  n_negative: 4560,
  n_unlabelled: 0,
  status_counts: { accepted: 4900, provisional: 300 },
  confusion_matrices: {
    lightgbm: { tp: 610, fp: 380, fn: 30, tn: 4180, source: 'held_out' },
    stack: { tp: 612, fp: 400, fn: 28, tn: 4160, source: 'cross_fit' },
  },
};

describe('Dataset', () => {
  const runs = signal<Run[]>([run]);
  const runsLoaded = signal(true);
  const runId = signal<string | null>('r1');
  const data = signal<RunDataset | null>(dataset);
  const loadDataset = vi.fn();

  const mount = async () => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: ModelStore,
          useValue: {
            runs,
            runsLoaded,
            runId,
            dataset: data,
            loading: signal(false),
            loadRuns: vi.fn(),
            loadDataset,
            selectRun: vi.fn(),
          },
        },
      ],
    });
    const fixture = TestBed.createComponent(Dataset);
    await fixture.whenStable();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  };

  const cells = (el: HTMLElement) =>
    Object.fromEntries(
      Array.from(el.querySelectorAll('[data-cell]')).map((c) => [
        (c as HTMLElement).dataset['cell'],
        c.querySelector('.font-mono')?.textContent?.trim(),
      ]),
    );

  beforeEach(() => {
    runs.set([run]);
    runsLoaded.set(true);
    runId.set('r1');
    data.set(dataset);
    loadDataset.mockClear();
  });

  it('loads the dataset of the chosen run', async () => {
    await mount();
    expect(loadDataset).toHaveBeenCalledWith('r1');
  });

  it('counts candidates, stars and features', async () => {
    const { el } = await mount();
    expect(
      Array.from(el.querySelectorAll('dd')).map((d) => d.textContent?.trim()),
    ).toEqual(['5,200', '1,800', '36']);
  });

  it('splits the candidates by label, leaving out empty labels', async () => {
    const { el } = await mount();
    const labels = Array.from(el.querySelectorAll('[data-label]')).map((l) =>
      l.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(labels).toEqual([
      'Planet candidates 640(12.3%)',
      'Not planets 4,560(87.7%)',
    ]);
  });

  it('shows the selected model first and its matrix, precision and recall', async () => {
    const { el } = await mount();
    const models = Array.from(el.querySelectorAll('[data-model]'));
    expect(
      models.map((m) => m.textContent?.replace(/\s+/g, ' ').trim()),
    ).toEqual(['LightGBM (selected)', 'Stack']);
    expect(models[0].getAttribute('aria-pressed')).toBe('true');
    expect(cells(el)).toEqual({ tp: '610', fn: '30', fp: '380', tn: '4,180' });
    expect(el.querySelector('[data-precision]')?.textContent?.trim()).toBe(
      '0.62',
    );
    expect(el.querySelector('[data-recall]')?.textContent?.trim()).toBe('0.95');
    expect(el.querySelector('[data-source]')?.textContent).toContain(
      'held out',
    );
  });

  it('switches the matrix to the chosen model', async () => {
    const { fixture, el } = await mount();
    (el.querySelectorAll('[data-model]')[1] as HTMLButtonElement).click();
    await fixture.whenStable();
    expect(cells(el)).toEqual({ tp: '612', fn: '28', fp: '400', tn: '4,160' });
    expect(el.querySelector('[data-source]')?.textContent).toContain(
      'Cross-fitted',
    );
  });

  it('shows no precision or recall when nothing was predicted or labelled positive', async () => {
    data.set({
      ...dataset,
      confusion_matrices: {
        lightgbm: { tp: 0, fp: 0, fn: 0, tn: 5, source: 'held_out' },
      },
    });
    const { el } = await mount();
    expect(el.querySelector('[data-precision]')?.textContent?.trim()).toBe('—');
    expect(el.querySelector('[data-recall]')?.textContent?.trim()).toBe('—');
  });

  it('says so when no model is trained yet', async () => {
    runs.set([]);
    data.set(null);
    const { el } = await mount();
    expect(el.querySelector('[data-empty]')).not.toBeNull();
  });
});
