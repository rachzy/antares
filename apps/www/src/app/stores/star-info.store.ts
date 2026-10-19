import { inject } from '@angular/core';
import { patchState, signalStore, withMethods, withState } from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { catchError, EMPTY, pipe, switchMap, tap } from 'rxjs';
import { ApiService } from '../api/api.service';
import type { StarDetails, Target } from '../api/api.types';

interface StarInfoState {
  details: StarDetails | null;
  loading: boolean;
  failed: boolean;
}

export const StarInfoStore = signalStore(
  { providedIn: 'root' },
  withState<StarInfoState>({ details: null, loading: false, failed: false }),
  withMethods((store, api = inject(ApiService)) => {
    const fetch = rxMethod<Target>(
      pipe(
        tap(() =>
          patchState(store, { details: null, loading: true, failed: false }),
        ),
        switchMap((target) =>
          api.starInfo(target.designation).pipe(
            tap((info) =>
              patchState(store, { details: info.star, loading: false }),
            ),
            catchError(() => {
              patchState(store, { loading: false, failed: true });
              return EMPTY;
            }),
          ),
        ),
      ),
    );
    return {
      load: (target: Target) => fetch(target),
      reset: () =>
        patchState(store, { details: null, loading: false, failed: false }),
    };
  }),
);
