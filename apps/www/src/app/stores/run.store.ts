import { computed, effect, inject, signal, untracked } from '@angular/core';
import {
  patchState,
  signalStore,
  withComputed,
  withHooks,
  withMethods,
  withProps,
  withState,
} from '@ngrx/signals';
import {
  catchError,
  forkJoin,
  map,
  of,
  Subscription,
  type Observable,
} from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import type {
  ExtractRequest,
  JobResult,
  Mission,
  SubscriptionEvent,
  SubscriptionView,
  Target,
} from '../api/api.types';
import { PipelineStore } from './pipeline.store';
import { SubscriptionStore } from './subscription.store';

export interface Stage {
  status: 'idle' | 'running' | 'done' | 'failed';
  token: string | null;
  events: SubscriptionEvent[];
  result: JobResult | null;
  error: ApiRequestError | null;
  cached: boolean;
}

export type Step = 'select' | 'download' | 'process' | 'score';
type Job = 'extract' | 'score';

interface RunState {
  star: Target | null;
  extract: Stage;
  score: Stage;
  ready: boolean;
  downloadAll: boolean;
}

interface Saved {
  star: Target | null;
  extractToken: string | null;
  scoreToken: string | null;
  extractCached: boolean;
  downloadAll?: boolean;
}

const KEY = 'antares.run';
const MISSIONS: Record<string, Mission> = {
  KIC: 'Kepler',
  EPIC: 'K2',
  TIC: 'TESS',
};

const idle: Stage = {
  status: 'idle',
  token: null,
  events: [],
  result: null,
  error: null,
  cached: false,
};

function storage(op: (s: Storage) => string | null | void): string | null {
  try {
    return op(localStorage) ?? null;
  } catch {
    return null;
  }
}

function merge(
  kept: SubscriptionEvent[],
  live: SubscriptionEvent[],
): SubscriptionEvent[] {
  const seqs = new Set(live.map((e) => e.seq));
  return [...kept.filter((e) => !seqs.has(e.seq)), ...live].sort(
    (a, b) => a.seq - b.seq,
  );
}

function fromView(token: string, view: SubscriptionView): Stage {
  const events = view.milestones.map((m): SubscriptionEvent => ({
    ...m,
    type: 'state',
  }));
  const base = { ...idle, token, events };
  if (view.state === 'done') {
    return { ...base, status: 'done', result: view.result ?? null };
  }
  if (view.state === 'failed' || view.state === 'dropped') {
    const message = view.error ?? 'The job was dropped.';
    return {
      ...base,
      status: 'failed',
      error: new ApiRequestError(0, 'internal', message),
    };
  }
  return { ...base, status: 'running' };
}

const finished = (stage: Stage) =>
  stage.status === 'done' || stage.status === 'failed';

export const RunStore = signalStore(
  { providedIn: 'root' },
  withState<RunState>({
    star: null,
    extract: idle,
    score: idle,
    ready: false,
    downloadAll: false,
  }),
  withProps(() => ({ _current: signal<Job | null>(null) })),
  withComputed((store) => {
    const downloadDone = computed(
      () =>
        store.extract().status === 'done' ||
        store.extract().events.some((e) => e.stage === 'detrending'),
    );
    const scoreStarted = computed(() => store.score().status !== 'idle');
    return {
      downloadDone,
      processDone: computed(() => store.extract().status === 'done'),
      scoreStarted,
      scoreDone: computed(() => store.score().status === 'done'),
      unlocked: computed<Step>(() => {
        if (!store.star()) return 'select';
        if (scoreStarted()) return 'score';
        // A confirmed star may have no light curve, so Download opens only once an extraction started.
        if (store.extract().status === 'idle') return 'select';
        return downloadDone() ? 'process' : 'download';
      }),
      candidates: computed(() => store.extract().result?.features.length ?? 0),
    };
  }),
  withMethods(
    (
      store,
      api = inject(ApiService),
      pipeline = inject(PipelineStore),
      subscription = inject(SubscriptionStore),
    ) => {
      const persist = () => {
        const saved: Saved = {
          star: store.star(),
          extractToken: store.extract().token,
          scoreToken: store.score().token,
          extractCached: store.extract().cached,
          downloadAll: store.downloadAll(),
        };
        storage((s) => s.setItem(KEY, JSON.stringify(saved)));
      };

      let restoring = new Subscription();
      let resumeScore: string | null = null;
      // A cached extract restored by re-running it keeps the run unready until the re-run settles.
      let settling = false;

      const cancel = () => {
        restoring.unsubscribe();
        resumeScore = null;
        settling = false;
      };

      const clear = () => {
        cancel();
        store._current.set(null);
        pipeline.reset();
        patchState(store, {
          star: null,
          extract: idle,
          score: idle,
          ready: true,
          downloadAll: false,
        });
        storage((s) => s.removeItem(KEY));
      };

      const body = (): ExtractRequest => {
        const star = store.star();
        if (!star) return {};
        const mission = MISSIONS[star.catalog];
        return {
          ra_deg: star.ra_deg,
          dec_deg: star.dec_deg,
          ...(mission ? { mission } : {}),
          ...(store.downloadAll() ? { download_all: true } : {}),
        };
      };

      const run = (job: Job, cached = false) => {
        cancel();
        settling = cached;
        store._current.set(job);
        patchState(store, {
          ...(cached ? {} : { ready: true }),
          [job]: { ...idle, status: 'running', cached },
        });
        persist();
        if (job === 'extract') pipeline.extract(body());
        else pipeline.analyze(body());
      };

      const sync = () => {
        const job = store._current();
        if (!job) return;
        const slice = untracked(store[job]);
        if (finished(slice)) return;
        const token = subscription.token();
        const state = subscription.state();
        const error = subscription.error() ?? pipeline.error();
        const result = pipeline.result();
        let next: Stage = {
          ...slice,
          token: token ?? slice.token,
          cached: token ? false : slice.cached,
          events: token
            ? merge(slice.events, subscription.events())
            : slice.events,
        };
        if (token && state === 'done') {
          next = { ...next, status: 'done', result: subscription.result() };
        } else if (
          error ||
          (token && (state === 'failed' || state === 'dropped'))
        ) {
          next = {
            ...next,
            status: 'failed',
            error:
              error ??
              new ApiRequestError(0, 'internal', 'The job was dropped.'),
          };
        } else if (!token && result) {
          next = { ...next, status: 'done', result, cached: true };
        }
        patchState(store, { [job]: next });
        if (settling && (finished(next) || next.token)) {
          settling = false;
          patchState(store, { ready: true });
        }
        if (next.token !== slice.token || next.cached !== slice.cached) {
          untracked(persist);
        }
        if (job === 'extract' && next.status === 'done' && resumeScore) {
          const token = resumeScore;
          resumeScore = null;
          store._current.set('score');
          untracked(() => subscription.track(token));
        }
      };

      const load = (token: string | null): Observable<Stage | 'gone'> =>
        token
          ? api.snapshot(token).pipe(
              map((view) => fromView(token, view)),
              catchError((error: ApiRequestError) =>
                of<Stage | 'gone'>(
                  error?.status === 404
                    ? 'gone'
                    : { ...idle, token, status: 'failed', error },
                ),
              ),
            )
          : of(idle);

      const restore = () => {
        const raw = storage((s) => s.getItem(KEY));
        if (!raw) return patchState(store, { ready: true });
        let saved: Saved | null = null;
        try {
          saved = JSON.parse(raw);
        } catch {
          saved = null;
        }
        if (!saved?.star) return clear();
        const cached = !!saved.extractCached;
        cancel();
        store._current.set(null);
        patchState(store, {
          star: saved.star,
          extract: idle,
          score: idle,
          downloadAll: !!saved.downloadAll,
        });
        restoring = forkJoin([
          cached ? of(idle) : load(saved.extractToken),
          load(saved.scoreToken),
        ]).subscribe(([extract, score]) => {
          if (extract === 'gone' || score === 'gone') return clear();
          if (cached) {
            patchState(store, { score });
            run('extract', true);
            if (score.status === 'running') resumeScore = score.token;
            return;
          }
          patchState(store, { extract, score, ready: true });
          const job: Job | null =
            score.status === 'running'
              ? 'score'
              : extract.status === 'running'
                ? 'extract'
                : null;
          if (job) {
            store._current.set(job);
            subscription.track(store[job]().token as string);
          }
        });
      };

      return {
        confirm: (star: Target) => {
          clear();
          patchState(store, { star });
          persist();
        },
        startExtract: (downloadAll = false, resolved?: Target) => {
          patchState(store, {
            score: idle,
            downloadAll,
            ...(resolved ? { star: resolved } : {}),
          });
          run('extract');
        },
        startScore: () => {
          if (store.extract().status === 'done') run('score');
        },
        retry: () => {
          if (store.extract().status === 'failed') run('extract');
          else if (store.score().status === 'failed') run('score');
        },
        restore,
        reset: clear,
        _sync: sync,
      };
    },
  ),
  withHooks((store) => ({
    onInit: () => effect(() => store._sync()),
  })),
);
