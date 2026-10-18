import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import type {
  JobResult,
  SubscriptionEvent,
  SubscriptionView,
  Target,
} from '../api/api.types';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { SubscriptionPhase } from './subscription.store';
import { SubscriptionStore } from './subscription.store';
import { PipelineStore } from './pipeline.store';
import { RunStore } from './run.store';

const KEY = 'antares.run';

const star: Target = {
  catalog: 'KIC',
  catalog_id: '11446443',
  display_name: 'Kepler-1',
  designation: 'KIC 11446443',
  ra_deg: 286.8,
  dec_deg: 49.3,
};
const body = { ra_deg: 286.8, dec_deg: 49.3, mission: 'Kepler' };

const ev = (
  seq: number,
  stage: string,
  type: SubscriptionEvent['type'] = 'state',
): SubscriptionEvent => ({ seq, type, stage, at: '2026-01-01T00:00:00Z' });

const features = { cached: false, features: [{}, {}, {}] } as JobResult;
const scores = { cached: false, features: [{}] } as JobResult;

const view = (
  state: SubscriptionView['state'],
  extra: Partial<SubscriptionView> = {},
): SubscriptionView => ({
  kind: 'extract',
  state,
  milestones: [
    { seq: 1, stage: 'downloading', message: '', at: '' },
    { seq: 2, stage: 'detrending', message: '', at: '' },
  ],
  last_seq: 2,
  ...extra,
});

describe('RunStore', () => {
  const token = signal<string | null>(null);
  const state = signal<SubscriptionPhase>('idle');
  const events = signal<SubscriptionEvent[]>([]);
  const subResult = signal<JobResult | null>(null);
  const subError = signal<ApiRequestError | null>(null);
  const pResult = signal<JobResult | null>(null);
  const pError = signal<ApiRequestError | null>(null);
  const pLoading = signal(false);
  const track = vi.fn();
  const stop = vi.fn();
  const extract = vi.fn();
  const analyze = vi.fn();
  const reset = vi.fn();
  const snapshot = vi.fn();

  const clearSub = () => {
    token.set(null);
    state.set('idle');
    events.set([]);
    subResult.set(null);
    subError.set(null);
  };
  const start = () => {
    clearSub();
    pResult.set(null);
    pError.set(null);
    pLoading.set(true);
  };

  function setup() {
    clearSub();
    pResult.set(null);
    pError.set(null);
    pLoading.set(false);
    for (const fn of [track, stop, extract, analyze, reset, snapshot]) {
      fn.mockReset();
    }
    stop.mockImplementation(clearSub);
    track.mockImplementation((t: string) => {
      clearSub();
      token.set(t);
      state.set('queued');
    });
    extract.mockImplementation(start);
    analyze.mockImplementation(start);
    TestBed.configureTestingModule({
      providers: [
        { provide: ApiService, useValue: { snapshot } },
        {
          provide: PipelineStore,
          useValue: {
            result: pResult,
            error: pError,
            loading: pLoading,
            extract,
            analyze,
            reset,
          },
        },
        {
          provide: SubscriptionStore,
          useValue: {
            token,
            state,
            events,
            result: subResult,
            error: subError,
            track,
            stop,
          },
        },
      ],
    });
    const store = TestBed.inject(RunStore);
    TestBed.tick();
    return store;
  }

  const runJob = (t: string, result: JobResult) => {
    token.set(t);
    state.set('running');
    events.set([ev(1, 'downloading'), ev(2, 'detrending')]);
    TestBed.tick();
    state.set('done');
    subResult.set(result);
    pResult.set(result);
    pLoading.set(false);
    TestBed.tick();
  };

  beforeEach(() => localStorage.clear());
  afterEach(() => vi.restoreAllMocks());

  it('starts at select with no star', () => {
    const store = setup();
    expect(store.star()).toBeNull();
    expect(store.unlocked()).toBe('select');
  });

  it('confirm stores the star and clears both slices', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    runJob('e', features);
    store.confirm({ ...star, catalog_id: '2' });
    TestBed.tick();
    expect(store.star()?.catalog_id).toBe('2');
    expect(store.extract().status).toBe('idle');
    expect(store.extract().events).toEqual([]);
    expect(store.score().status).toBe('idle');
    expect(store.unlocked()).toBe('download');
    expect(reset).toHaveBeenCalled();
  });

  it('startExtract sends the star position with the mapped mission', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    expect(extract).toHaveBeenCalledWith(body);
    expect(store.extract().status).toBe('running');
  });

  it('maps EPIC to K2, TIC to TESS and drops unknown catalogs', () => {
    const store = setup();
    store.confirm({ ...star, catalog: 'EPIC' });
    store.startExtract();
    store.confirm({ ...star, catalog: 'TIC' });
    store.startExtract();
    store.confirm({ ...star, catalog: 'Gaia' });
    store.startExtract();
    expect(extract.mock.calls.map((c) => c[0].mission)).toEqual([
      'K2',
      'TESS',
      undefined,
    ]);
    expect('mission' in extract.mock.calls[2][0]).toBe(false);
  });

  it('marks the download done once detrending is reached', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    token.set('e');
    state.set('running');
    events.set([ev(1, 'downloading', 'progress')]);
    TestBed.tick();
    expect(store.downloadDone()).toBe(false);
    events.set([ev(1, 'downloading', 'progress'), ev(2, 'detrending')]);
    TestBed.tick();
    expect(store.downloadDone()).toBe(true);
    expect(store.processDone()).toBe(false);
    expect(store.extract().token).toBe('e');
  });

  it('a done state copies events and result into extract', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    runJob('e', features);
    expect(store.extract().status).toBe('done');
    expect(store.extract().result).toBe(features);
    expect(store.extract().events.map((e) => e.seq)).toEqual([1, 2]);
    expect(store.extract().cached).toBe(false);
    expect(store.processDone()).toBe(true);
    expect(store.candidates()).toBe(3);
    expect(store.unlocked()).toBe('process');
  });

  it('a 200 done with no token marks extract cached and done', () => {
    const store = setup();
    store.confirm(star);
    extract.mockImplementation(() => {
      start();
      pResult.set(features);
      pLoading.set(false);
    });
    store.startExtract();
    TestBed.tick();
    expect(store.extract().status).toBe('done');
    expect(store.extract().cached).toBe(true);
    expect(store.extract().token).toBeNull();
    expect(store.downloadDone()).toBe(true);
    expect(store.unlocked()).toBe('process');
  });

  it('startScore sends the same body and unlocks score', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    runJob('e', features);
    store.startScore();
    TestBed.tick();
    expect(analyze).toHaveBeenCalledWith(body);
    expect(store.scoreStarted()).toBe(true);
    expect(store.unlocked()).toBe('score');
    runJob('s', scores);
    expect(store.scoreDone()).toBe(true);
    expect(store.score().result).toBe(scores);
  });

  it('a second job never overwrites the first slice', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    runJob('e', features);
    store.startScore();
    token.set('s');
    state.set('running');
    events.set([ev(1, 'predicting')]);
    TestBed.tick();
    expect(store.extract().token).toBe('e');
    expect(store.extract().result).toBe(features);
    expect(store.extract().events.map((e) => e.stage)).toEqual([
      'downloading',
      'detrending',
    ]);
    expect(store.score().events.map((e) => e.stage)).toEqual(['predicting']);
  });

  it('a failure sets the slice error and retry re-runs only that job', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    runJob('e', features);
    store.startScore();
    const error = new ApiRequestError(503, 'busy', 'busy');
    pError.set(error);
    pLoading.set(false);
    TestBed.tick();
    expect(store.score().status).toBe('failed');
    expect(store.score().error).toBe(error);
    expect(store.extract().status).toBe('done');
    store.retry();
    TestBed.tick();
    expect(analyze).toHaveBeenCalledTimes(2);
    expect(extract).toHaveBeenCalledTimes(1);
    expect(store.score().status).toBe('running');
    expect(store.score().error).toBeNull();
  });

  it('a failed subscription fails the slice', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    token.set('e');
    state.set('failed');
    subError.set(new ApiRequestError(0, 'internal', 'boom'));
    TestBed.tick();
    expect(store.extract().status).toBe('failed');
    expect(store.extract().error?.message).toBe('boom');
  });

  it('persists the star and both tokens', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    runJob('e', features);
    store.startScore();
    token.set('s');
    TestBed.tick();
    expect(JSON.parse(localStorage.getItem(KEY) ?? '')).toEqual({
      star,
      extractToken: 'e',
      scoreToken: 's',
    });
  });

  it('restore fills both slices from snapshots', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ star, extractToken: 'e', scoreToken: 's' }),
    );
    const store = setup();
    snapshot.mockImplementation((t: string) =>
      of(
        t === 'e'
          ? view('done', { result: features })
          : view('running', {
              kind: 'analyze',
              milestones: [
                { seq: 1, stage: 'predicting', message: '', at: '' },
              ],
            }),
      ),
    );
    store.restore();
    TestBed.tick();
    expect(store.star()).toEqual(star);
    expect(store.extract().status).toBe('done');
    expect(store.extract().result).toBe(features);
    expect(store.extract().events.map((e) => [e.type, e.stage])).toEqual([
      ['state', 'downloading'],
      ['state', 'detrending'],
    ]);
    expect(store.score().status).toBe('running');
    expect(store.score().token).toBe('s');
    expect(track).toHaveBeenCalledWith('s');
    expect(track).toHaveBeenCalledTimes(1);
    expect(store.unlocked()).toBe('score');
    expect(store.score().events.map((e) => e.stage)).toEqual(['predicting']);
  });

  it('restore re-tracks an unfinished extract', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ star, extractToken: 'e', scoreToken: null }),
    );
    const store = setup();
    snapshot.mockReturnValue(of(view('running')));
    store.restore();
    TestBed.tick();
    expect(track).toHaveBeenCalledWith('e');
    expect(store.extract().status).toBe('running');
    expect(store.downloadDone()).toBe(true);
    state.set('done');
    subResult.set(features);
    TestBed.tick();
    expect(store.extract().status).toBe('done');
    expect(store.extract().events.map((e) => e.stage)).toEqual([
      'downloading',
      'detrending',
    ]);
  });

  it('a 404 snapshot clears storage and leaves the star null', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ star, extractToken: 'e', scoreToken: null }),
    );
    const store = setup();
    snapshot.mockReturnValue(
      throwError(() => new ApiRequestError(404, 'not_found', 'gone')),
    );
    store.restore();
    TestBed.tick();
    expect(store.star()).toBeNull();
    expect(store.extract().status).toBe('idle');
    expect(store.unlocked()).toBe('select');
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it('restore with nothing stored does nothing', () => {
    const store = setup();
    store.restore();
    expect(snapshot).not.toHaveBeenCalled();
    expect(store.star()).toBeNull();
  });

  it('reset clears the run and storage', () => {
    const store = setup();
    store.confirm(star);
    store.startExtract();
    runJob('e', features);
    store.reset();
    expect(store.star()).toBeNull();
    expect(store.extract().status).toBe('idle');
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it('still works when localStorage throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('denied');
    });
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('denied');
    });
    const store = setup();
    store.restore();
    store.confirm(star);
    store.startExtract();
    runJob('e', features);
    expect(store.extract().status).toBe('done');
    store.reset();
    expect(store.star()).toBeNull();
  });
});
