import { computed, inject } from '@angular/core';
import {
  patchState,
  signalStore,
  withComputed,
  withMethods,
  withState,
} from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import {
  catchError,
  defer,
  EMPTY,
  pipe,
  repeat,
  switchMap,
  tap,
  timer,
} from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { Health } from '../api/api.types';

interface HealthState {
  health: Health | null;
  loading: boolean;
  error: ApiRequestError | null;
}

const RECHECK_MS = 3000;

export const HealthStore = signalStore(
  { providedIn: 'root' },
  withState<HealthState>({ health: null, loading: false, error: null }),
  withComputed(({ health }) => ({
    ready: computed(() => health()?.ready ?? false),
  })),
  withMethods((store, api = inject(ApiService)) => ({
    check: rxMethod<void>(
      pipe(
        switchMap(() =>
          defer(() => {
            patchState(store, { loading: true });
            return api.health().pipe(
              tap((health) =>
                patchState(store, { health, error: null, loading: false }),
              ),
              catchError((error: ApiRequestError) => {
                patchState(store, { error, loading: false });
                return EMPTY;
              }),
            );
          }).pipe(
            repeat({
              delay: () => (store.ready() ? EMPTY : timer(RECHECK_MS)),
            }),
          ),
        ),
      ),
    ),
  })),
);
