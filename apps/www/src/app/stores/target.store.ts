import { inject } from '@angular/core';
import { patchState, signalStore, withMethods, withState } from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { catchError, EMPTY, pipe, switchMap, tap } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { ResolveRequest, Target } from '../api/api.types';

interface TargetState {
  target: Target | null;
  loading: boolean;
  error: ApiRequestError | null;
}

export const TargetStore = signalStore(
  { providedIn: 'root' },
  withState<TargetState>({ target: null, loading: false, error: null }),
  withMethods((store, api = inject(ApiService)) => {
    const resolve = rxMethod<ResolveRequest>(
      pipe(
        tap(() =>
          patchState(store, { target: null, loading: true, error: null }),
        ),
        switchMap((body) =>
          api.resolveTarget(body).pipe(
            tap((target) => patchState(store, { target, loading: false })),
            catchError((error: ApiRequestError) => {
              patchState(store, { target: null, error, loading: false });
              return EMPTY;
            }),
          ),
        ),
      ),
    );
    return {
      reset: () =>
        patchState(store, { target: null, loading: false, error: null }),
      resolveByName: (query: string, mission?: string) =>
        resolve({ query, mission }),
      resolveByPosition: (ra_deg: number, dec_deg: number, mission?: string) =>
        resolve({ ra_deg, dec_deg, mission }),
    };
  }),
);
