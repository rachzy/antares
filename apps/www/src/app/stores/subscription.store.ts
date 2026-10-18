import { inject } from '@angular/core';
import { patchState, signalStore, withMethods, withState } from '@ngrx/signals';
import { exhaustMap, Subscription, takeWhile, timer } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type {
  JobResult,
  SubscriptionEvent,
  SubscriptionView,
} from '../api/api.types';

export type SubscriptionPhase =
  'idle' | 'queued' | 'running' | 'done' | 'failed' | 'dropped';

interface SubscriptionState {
  token: string | null;
  state: SubscriptionPhase;
  events: SubscriptionEvent[];
  lastSeq: number;
  result: JobResult | null;
  error: ApiRequestError | null;
}

const KEY = 'antares.subscription';
const MAX_ATTEMPTS = 5;
const POLL_MS = 2000;
const RETRYABLE = new Set([1001, 1006, 1011, 1012, 1013, 1014]);
const TERMINAL = new Set<SubscriptionPhase>(['done', 'failed', 'dropped']);

const initial: SubscriptionState = {
  token: null,
  state: 'idle',
  events: [],
  lastSeq: 0,
  result: null,
  error: null,
};

function storage(op: (s: Storage) => string | null | void): string | null {
  try {
    return op(localStorage) ?? null;
  } catch {
    return null;
  }
}

function phaseOf(e: SubscriptionEvent): SubscriptionPhase {
  if (e.stage === 'queued' || e.stage === 'dropped') return e.stage;
  return 'running';
}

export const SubscriptionStore = signalStore(
  { providedIn: 'root' },
  withState<SubscriptionState>(initial),
  withMethods((store, api = inject(ApiService)) => {
    let session = new Subscription();
    let attempts = 0;

    const finished = () => TERMINAL.has(store.state());

    const apply = (events: SubscriptionEvent[]) => {
      attempts = 0;
      let lastSeq = store.lastSeq();
      const fresh: SubscriptionEvent[] = [];
      const patch: Partial<SubscriptionState> = {};
      for (const e of events) {
        if (e.seq <= lastSeq) continue;
        lastSeq = e.seq;
        fresh.push(e);
        if (e.type === 'result') {
          patch.state = 'done';
          patch.result = e.payload ?? null;
        } else if (e.type === 'error') {
          patch.state = 'failed';
          patch.error = new ApiRequestError(0, 'internal', e.message ?? '');
        } else if (!patch.state || !TERMINAL.has(patch.state)) {
          patch.state = e.type === 'state' ? phaseOf(e) : 'running';
        }
      }
      if (!fresh.length) return;
      patchState(store, {
        ...patch,
        events: [...store.events(), ...fresh],
        lastSeq,
      });
    };

    const applyView = (view: SubscriptionView) => {
      const lastSeq = store.lastSeq();
      const fresh = view.milestones
        .filter((m) => m.seq > lastSeq)
        .map((m): SubscriptionEvent => ({ ...m, type: 'state' }));
      patchState(store, {
        state: view.state,
        events: [...store.events(), ...fresh],
        lastSeq: Math.max(lastSeq, view.last_seq, ...fresh.map((e) => e.seq)),
        result: view.result ?? null,
        error: view.error
          ? new ApiRequestError(0, 'internal', view.error)
          : null,
      });
    };

    const forget = () => storage((s) => s.removeItem(KEY));

    const poll = (s: Subscription, token: string) =>
      s.add(
        timer(0, POLL_MS)
          .pipe(
            exhaustMap(() => api.snapshot(token)),
            takeWhile((v) => !TERMINAL.has(v.state), true),
          )
          .subscribe({
            next: applyView,
            error: (error: ApiRequestError) => {
              if (error.status === 404) {
                forget();
                patchState(store, { ...initial, error });
              } else {
                patchState(store, { error });
              }
            },
          }),
      );

    const connect = (s: Subscription, token: string) =>
      s.add(
        api.watch(token, store.lastSeq() || undefined).subscribe({
          next: apply,
          complete: () => {
            if (!finished()) poll(s, token);
          },
          error: (e: { code?: number }) => {
            if (finished()) return;
            if (RETRYABLE.has(e?.code ?? 0) && attempts < MAX_ATTEMPTS) {
              const delay = 1000 * 2 ** attempts++;
              s.add(timer(delay).subscribe(() => connect(s, token)));
            } else {
              poll(s, token);
            }
          },
        }),
      );

    const stop = () => {
      session.unsubscribe();
      session = new Subscription();
      attempts = 0;
      forget();
      patchState(store, initial);
    };

    const track = (token: string) => {
      stop();
      storage((s) => s.setItem(KEY, token));
      patchState(store, { token, state: 'queued' });
      if (typeof WebSocket === 'undefined') poll(session, token);
      else connect(session, token);
    };

    return {
      track,
      stop,
      resume: () => {
        const token = storage((s) => s.getItem(KEY));
        if (token) track(token);
      },
    };
  }),
);
