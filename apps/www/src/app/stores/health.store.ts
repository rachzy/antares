import { computed, inject } from '@angular/core';
import {
  patchState,
  signalStore,
  withComputed,
  withMethods,
  withState,
} from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { catchError, EMPTY, pipe, switchMap, tap } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { Health } from '../api/api.types';

interface HealthState {
  health: Health | null;
  loading: boolean;
  error: ApiRequestError | null;
}

export const HealthStore = signalStore(
  { providedIn: 'root' },
  withState<HealthState>({ health: null, loading: false, error: null }),
  withComputed(({ health }) => ({
    ready: computed(() => health()?.ready ?? false),
  })),
  withMethods((store, api = inject(ApiService)) => ({
    check: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          api.health().pipe(
            tap((health) => patchState(store, { health, loading: false })),
            catchError((error: ApiRequestError) => {
              patchState(store, { error, loading: false });
              return EMPTY;
            }),
          ),
        ),
      ),
    ),
  })),
);
