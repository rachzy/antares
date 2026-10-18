import { computed, effect, inject } from '@angular/core';
import {
  patchState,
  signalStore,
  withComputed,
  withHooks,
  withMethods,
  withState,
} from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { catchError, EMPTY, Observable, pipe, switchMap, tap } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { ExtractRequest, Job, JobResult } from '../api/api.types';
import { SubscriptionStore } from './subscription.store';

interface PipelineState {
  result: JobResult | null;
  position: number | null;
  loading: boolean;
  error: ApiRequestError | null;
}

const BUSY_RETRY_SECONDS = 30;

export const PipelineStore = signalStore(
  { providedIn: 'root' },
  withState<PipelineState>({
    result: null,
    position: null,
    loading: false,
    error: null,
  }),
  withComputed((store, subscription = inject(SubscriptionStore)) => ({
    status: computed(() => subscription.state()),
    progress: computed(() => {
      const last = subscription
        .events()
        .slice()
        .reverse()
        .find((e) => e.type === 'progress');
      return last
        ? { message: last.message, fraction: last.fraction, stage: last.stage }
        : null;
    }),
    retryAfterSeconds: computed(() => {
      const error = store.error();
      return error?.code === 'busy'
        ? (error.retryAfter ?? BUSY_RETRY_SECONDS)
        : null;
    }),
  })),
  withMethods(
    (
      store,
      api = inject(ApiService),
      subscription = inject(SubscriptionStore),
    ) => {
      const start = (call: (body: ExtractRequest) => Observable<Job>) =>
        rxMethod<ExtractRequest>(
          pipe(
            tap(() => {
              subscription.stop();
              patchState(store, {
                result: null,
                position: null,
                error: null,
                loading: true,
              });
            }),
            switchMap((body) =>
              call(body).pipe(
                tap((job) => {
                  if (job.status === 'done') {
                    patchState(store, { result: job.result, loading: false });
                  } else {
                    patchState(store, {
                      position: job.status === 'queued' ? job.position : null,
                    });
                    subscription.track(job.token);
                  }
                }),
                catchError((error: ApiRequestError) => {
                  patchState(store, { error, loading: false });
                  return EMPTY;
                }),
              ),
            ),
          ),
        );
      return {
        extract: start((b) => api.extract(b)),
        analyze: start((b) => api.analyze(b)),
        reset: () => {
          subscription.stop();
          patchState(store, {
            result: null,
            position: null,
            loading: false,
            error: null,
          });
        },
      };
    },
  ),
  withHooks((store, subscription = inject(SubscriptionStore)) => ({
    onInit: () => {
      effect(() => {
        const state = subscription.state();
        const error = subscription.error();
        if (state === 'done') {
          patchState(store, { result: subscription.result(), loading: false });
        } else if (state === 'failed' || state === 'dropped' || error) {
          patchState(store, {
            error:
              error ??
              new ApiRequestError(0, 'internal', 'The job was dropped.'),
            loading: false,
          });
        } else if (state !== 'idle') {
          patchState(store, { loading: true });
        }
      });
    },
  })),
);
