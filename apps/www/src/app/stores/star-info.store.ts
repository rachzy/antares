import { inject } from '@angular/core';
import { patchState, signalStore, withMethods, withState } from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { catchError, EMPTY, pipe, switchMap, tap } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { StarDetails, Target } from '../api/api.types';

interface StarInfoState {
  target: Target | null;
  details: StarDetails | null;
  loading: boolean;
  failed: boolean;
  error: ApiRequestError | null;
}

interface Position {
  ra_deg: number;
  dec_deg: number;
}

const empty: StarInfoState = {
  target: null,
  details: null,
  loading: false,
  failed: false,
  error: null,
};

export const StarInfoStore = signalStore(
  { providedIn: 'root' },
  withState<StarInfoState>(empty),
  withMethods((store, api = inject(ApiService)) => {
    const fetch = rxMethod<Position>(
      pipe(
        tap(() => patchState(store, { ...empty, loading: true })),
        switchMap(({ ra_deg, dec_deg }) =>
          api.starInfo(ra_deg, dec_deg).pipe(
            tap((info) =>
              patchState(store, {
                target: info.target,
                details: info.star,
                loading: false,
              }),
            ),
            catchError((error: ApiRequestError) => {
              patchState(store, { loading: false, failed: true, error });
              return EMPTY;
            }),
          ),
        ),
      ),
    );
    return {
      // SIMBAD picks the star nearest the position, so any star has details, mission target or not.
      locate: (ra_deg: number, dec_deg: number) => fetch({ ra_deg, dec_deg }),
      load: (target: Target) => {
        const held = store.target();
        if (
          held?.ra_deg === target.ra_deg &&
          held.dec_deg === target.dec_deg &&
          store.details()
        ) {
          return;
        }
        fetch(target);
      },
      reset: () => patchState(store, empty),
    };
  }),
);
