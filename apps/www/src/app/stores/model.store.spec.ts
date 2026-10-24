import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { Run, RunDataset, RunHyperparameters } from '../api/api.types';
import { ModelStore } from './model.store';

const run = (run_id: string): Run => ({
  run_id,
  created_at: '2026-01-01T00:00:00Z',
  schema_version: 2,
  selected_model: 'lightgbm',
  threshold: 0.5,
});

const settings: RunHyperparameters = {
  run_id: 'b',
  selected_model: 'lightgbm',
  threshold: 0.5,
  best_params: { lightgbm: { n_estimators: 300 } },
};

const dataset: RunDataset = {
  run_id: 'b',
  selected_model: 'lightgbm',
  n_rows: 10,
  n_stars: 4,
  n_files: 4,
  n_features: 36,
  n_positive: 3,
  n_negative: 7,
  n_unlabelled: 0,
  status_counts: { accepted: 10 },
  confusion_matrices: {
    lightgbm: { tp: 3, fp: 1, fn: 0, tn: 6, source: 'held_out' },
  },
};

describe('ModelStore', () => {
  function setup(fake: object) {
    TestBed.configureTestingModule({
      providers: [{ provide: ApiService, useValue: fake }],
    });
    return TestBed.inject(ModelStore);
  }

  it('selects the newest run once the runs load', () => {
    const store = setup({ listRuns: () => of([run('b'), run('a')]) });
    store.loadRuns();
    expect(store.runs().map((r) => r.run_id)).toEqual(['b', 'a']);
    expect(store.runId()).toBe('b');
  });

  it('keeps the chosen run when the runs reload', () => {
    const store = setup({ listRuns: () => of([run('b'), run('a')]) });
    store.loadRuns();
    store.selectRun('a');
    store.loadRuns();
    expect(store.runId()).toBe('a');
  });

  it('has no run when none is trained yet', () => {
    const store = setup({ listRuns: () => of([]) });
    store.loadRuns();
    expect(store.runId()).toBeNull();
  });

  it('drops what it loaded for the previous run when another is chosen', () => {
    const store = setup({
      runHyperparameters: () => of(settings),
      runDataset: () => of(dataset),
    });
    store.loadHyperparameters('b');
    store.loadDataset('b');
    expect(store.hyperparameters()).toEqual(settings);
    expect(store.dataset()).toEqual(dataset);
    store.selectRun('a');
    expect(store.hyperparameters()).toBeNull();
    expect(store.dataset()).toBeNull();
  });

  it('keeps the error of a failed request', () => {
    const error = new ApiRequestError(404, 'not_found', 'unknown run');
    const store = setup({ runDataset: () => throwError(() => error) });
    store.loadDataset('x');
    expect(store.error()).toBe(error);
    expect(store.loading()).toBe(false);
  });
});
