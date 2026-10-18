import { inject } from '@angular/core';
import { patchState, signalStore, withMethods, withState } from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { catchError, EMPTY, pipe, switchMap, tap } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { PredictRequest, PredictionRow, Run } from '../api/api.types';

interface MlState {
  runs: Run[];
  predictions: PredictionRow[];
  loading: boolean;
  error: ApiRequestError | null;
}

export const MlStore = signalStore(
  { providedIn: 'root' },
  withState<MlState>({
    runs: [],
    predictions: [],
    loading: false,
    error: null,
  }),
  withMethods((store, api = inject(ApiService)) => {
    const fail = (error: ApiRequestError) => {
      patchState(store, { error, loading: false });
      return EMPTY;
    };
    const run = rxMethod<PredictRequest>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((body) =>
          api.predict(body).pipe(
            tap((r) =>
              patchState(store, { predictions: r.predictions, loading: false }),
            ),
            catchError(fail),
          ),
        ),
      ),
    );
    return {
      loadRuns: rxMethod<void>(
        pipe(
          tap(() => patchState(store, { loading: true, error: null })),
          switchMap(() =>
            api.listRuns().pipe(
              tap((runs) => patchState(store, { runs, loading: false })),
              catchError(fail),
            ),
          ),
        ),
      ),
      predict: (
        features: PredictRequest['features'],
        run_id?: string,
        star_id?: string,
      ) => run({ features, run_id, star_id }),
    };
  }),
);
