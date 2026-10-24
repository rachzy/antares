import { inject } from '@angular/core';
import { patchState, signalStore, withMethods, withState } from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { catchError, EMPTY, pipe, switchMap, tap } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { Run, RunDataset, RunHyperparameters } from '../api/api.types';

interface ModelState {
  runs: Run[];
  runsLoaded: boolean;
  runId: string | null;
  hyperparameters: RunHyperparameters | null;
  dataset: RunDataset | null;
  loading: boolean;
  error: ApiRequestError | null;
}

/** The trained model runs and what the classifier can tell about one of them. */
export const ModelStore = signalStore(
  { providedIn: 'root' },
  withState<ModelState>({
    runs: [],
    runsLoaded: false,
    runId: null,
    hyperparameters: null,
    dataset: null,
    loading: false,
    error: null,
  }),
  withMethods((store, api = inject(ApiService)) => {
    const fail = (error: ApiRequestError) => {
      patchState(store, { error, loading: false });
      return EMPTY;
    };
    const start = () => patchState(store, { loading: true, error: null });
    return {
      loadRuns: rxMethod<void>(
        pipe(
          tap(start),
          switchMap(() =>
            api.listRuns().pipe(
              tap((runs) =>
                patchState(store, {
                  runs,
                  runsLoaded: true,
                  runId: runs.some((r) => r.run_id === store.runId())
                    ? store.runId()
                    : (runs[0]?.run_id ?? null),
                  loading: false,
                }),
              ),
              catchError(fail),
            ),
          ),
        ),
      ),
      selectRun: (runId: string) =>
        patchState(store, { runId, hyperparameters: null, dataset: null }),
      loadHyperparameters: rxMethod<string>(
        pipe(
          tap(start),
          switchMap((runId) =>
            api.runHyperparameters(runId).pipe(
              tap((hyperparameters) =>
                patchState(store, { hyperparameters, loading: false }),
              ),
              catchError(fail),
            ),
          ),
        ),
      ),
      loadDataset: rxMethod<string>(
        pipe(
          tap(start),
          switchMap((runId) =>
            api.runDataset(runId).pipe(
              tap((dataset) => patchState(store, { dataset, loading: false })),
              catchError(fail),
            ),
          ),
        ),
      ),
    };
  }),
);
