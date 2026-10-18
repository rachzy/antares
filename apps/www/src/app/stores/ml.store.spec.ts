import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import type { PredictRequest, Run } from '../api/api.types';
import { ApiRequestError, ApiService } from '../api/api.service';
import { MlStore } from './ml.store';

const run = (run_id: string): Run => ({
  run_id,
  created_at: '2026-01-01T00:00:00Z',
  schema_version: 1,
  selected_model: 'm',
  threshold: 0.5,
});

describe('MlStore', () => {
  function setup(fake: object) {
    TestBed.configureTestingModule({
      providers: [{ provide: ApiService, useValue: fake }],
    });
    return TestBed.inject(MlStore);
  }

  it('loadRuns stores runs in received order', () => {
    const store = setup({ listRuns: () => of([run('b'), run('a')]) });
    store.loadRuns();
    expect(store.runs().map((r) => r.run_id)).toEqual(['b', 'a']);
    expect(store.loading()).toBe(false);
  });

  it('predict stores predictions', () => {
    const calls: PredictRequest[] = [];
    const store = setup({
      predict: (b: PredictRequest) => {
        calls.push(b);
        return of({
          model_run_id: 'r',
          schema_version: 1,
          predictions: [{ p: 1 }],
          cached: false,
        });
      },
    });
    store.predict([{ f: 1 }], 'r', 's');
    expect(calls[0]).toEqual({
      features: [{ f: 1 }],
      run_id: 'r',
      star_id: 's',
    });
    expect(store.predictions()).toEqual([{ p: 1 }]);
  });

  it('predict surfaces 503 unavailable', () => {
    const store = setup({
      predict: () =>
        throwError(() => new ApiRequestError(503, 'unavailable', 'down')),
    });
    store.predict([{ f: 1 }]);
    expect(store.error()?.code).toBe('unavailable');
    expect(store.error()?.status).toBe(503);
    expect(store.loading()).toBe(false);
  });
});
