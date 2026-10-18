import { TestBed } from '@angular/core/testing';
import { of, Subject, throwError } from 'rxjs';
import type {
  JobResult,
  SubscriptionEvent,
  SubscriptionView,
} from '../api/api.types';
import { ApiRequestError, ApiService } from '../api/api.service';
import { SubscriptionStore } from './subscription.store';

const KEY = 'antares.subscription';

const ev = (
  seq: number,
  type: SubscriptionEvent['type'] = 'progress',
  extra: Partial<SubscriptionEvent> = {},
): SubscriptionEvent => ({ seq, type, at: '2026-01-01T00:00:00Z', ...extra });

const result = { cached: false, features: [] } as unknown as JobResult;

describe('SubscriptionStore', () => {
  let streams: Subject<SubscriptionEvent[]>[];
  let watchCalls: [string, number | undefined][];
  let snapshot: ReturnType<typeof vi.fn>;

  function setup() {
    streams = [];
    watchCalls = [];
    snapshot = vi.fn();
    const watch = (token: string, since?: number) => {
      watchCalls.push([token, since]);
      const s = new Subject<SubscriptionEvent[]>();
      streams.push(s);
      return s;
    };
    TestBed.configureTestingModule({
      providers: [{ provide: ApiService, useValue: { watch, snapshot } }],
    });
    return TestBed.inject(SubscriptionStore);
  }

  const last = () => streams[streams.length - 1];

  beforeEach(() => localStorage.clear());

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('appends frame events and advances lastSeq', () => {
    const store = setup();
    store.track('t');
    expect(watchCalls).toEqual([['t', undefined]]);
    expect(store.token()).toBe('t');
    last().next([ev(1, 'state', { stage: 'running' }), ev(2)]);
    last().next([ev(3)]);
    expect(store.events().map((e) => e.seq)).toEqual([1, 2, 3]);
    expect(store.lastSeq()).toBe(3);
    expect(store.state()).toBe('running');
    expect(localStorage.getItem(KEY)).toBe('t');
  });

  it('sets done and result on a result event', () => {
    const store = setup();
    store.track('t');
    last().next([ev(1), ev(2, 'result', { payload: result })]);
    last().complete();
    expect(store.state()).toBe('done');
    expect(store.result()).toEqual(result);
  });

  it('sets failed and error on an error event', () => {
    const store = setup();
    store.track('t');
    last().next([ev(1, 'error', { message: 'boom' })]);
    last().complete();
    expect(store.state()).toBe('failed');
    expect(store.error()?.message).toBe('boom');
  });

  it('reconnects after 1013 with since=lastSeq and no duplicate seq', () => {
    vi.useFakeTimers();
    const store = setup();
    store.track('t');
    last().next([ev(1), ev(2)]);
    last().error({ code: 1013 });
    vi.advanceTimersByTime(1000);
    expect(watchCalls[1]).toEqual(['t', 2]);
    last().next([ev(2), ev(3)]);
    expect(store.events().map((e) => e.seq)).toEqual([1, 2, 3]);
    expect(store.lastSeq()).toBe(3);
  });

  it('reconnects after an abnormal close', () => {
    vi.useFakeTimers();
    const store = setup();
    store.track('t');
    last().next([ev(1)]);
    last().error({ code: 1006 });
    expect(watchCalls).toHaveLength(1);
    vi.advanceTimersByTime(1000);
    expect(watchCalls[1]).toEqual(['t', 1]);
  });

  it('backs off 1s·2^n and stops after 5 attempts', () => {
    vi.useFakeTimers();
    snapshot.mockReturnValue(of(view({ state: 'running' })));
    const store = setup();
    store.track('t');
    for (let n = 0; n < 5; n++) {
      last().error({ code: 1006 });
      vi.advanceTimersByTime(1000 * 2 ** n - 1);
      expect(watchCalls).toHaveLength(n + 1);
      vi.advanceTimersByTime(1);
      expect(watchCalls).toHaveLength(n + 2);
    }
    last().error({ code: 1006 });
    vi.advanceTimersByTime(60_000);
    expect(watchCalls).toHaveLength(6);
    expect(store.token()).toBe('t');
  });

  it('clears the stored token on a snapshot 404', () => {
    vi.useFakeTimers();
    vi.stubGlobal('WebSocket', undefined);
    const store = setup();
    snapshot.mockReturnValue(
      throwError(() => new ApiRequestError(404, 'not_found', 'gone')),
    );
    store.track('t');
    vi.advanceTimersByTime(0);
    expect(localStorage.getItem(KEY)).toBeNull();
    expect(store.token()).toBeNull();
    expect(store.error()?.code).toBe('not_found');
  });

  it('polls snapshots when WebSocket is undefined', () => {
    vi.useFakeTimers();
    vi.stubGlobal('WebSocket', undefined);
    const store = setup();
    snapshot
      .mockReturnValueOnce(
        of(view({ state: 'running', milestones: [m(1, 'running')] })),
      )
      .mockReturnValueOnce(
        of(
          view({
            state: 'done',
            result,
            milestones: [m(1, 'running'), m(3, 'done')],
            last_seq: 4,
          }),
        ),
      );
    store.track('t');
    vi.advanceTimersByTime(0);
    expect(watchCalls).toHaveLength(0);
    expect(store.state()).toBe('running');
    vi.advanceTimersByTime(2000);
    expect(store.state()).toBe('done');
    expect(store.result()).toEqual(result);
    expect(store.events().map((e) => e.seq)).toEqual([1, 3]);
    expect(store.lastSeq()).toBe(4);
    vi.advanceTimersByTime(10_000);
    expect(snapshot).toHaveBeenCalledTimes(2);
  });

  it('resume() snapshots a running job then watches since lastSeq', () => {
    localStorage.setItem(KEY, 'saved');
    const store = setup();
    snapshot.mockReturnValue(
      of(
        view({
          state: 'running',
          milestones: [m(1, 'running')],
          last_seq: 4,
        }),
      ),
    );
    store.resume();
    expect(snapshot).toHaveBeenCalledWith('saved');
    expect(watchCalls).toEqual([['saved', 4]]);
    expect(store.token()).toBe('saved');
    expect(store.state()).toBe('running');
    expect(store.events().map((e) => e.seq)).toEqual([1]);
  });

  it('resume() clears an expired token without opening a socket', () => {
    vi.useFakeTimers();
    localStorage.setItem(KEY, 'expired');
    const store = setup();
    snapshot.mockReturnValue(
      throwError(() => new ApiRequestError(404, 'not_found', 'gone')),
    );
    store.resume();
    expect(watchCalls).toHaveLength(0);
    expect(vi.getTimerCount()).toBe(0);
    expect(localStorage.getItem(KEY)).toBeNull();
    expect(store.token()).toBeNull();
    expect(store.state()).toBe('idle');
    expect(store.error()).toBeNull();
  });

  it('resume() mirrors a finished job without opening a socket', () => {
    localStorage.setItem(KEY, 'saved');
    const store = setup();
    snapshot.mockReturnValue(
      of(view({ state: 'done', result, milestones: [m(2, 'done')] })),
    );
    store.resume();
    expect(watchCalls).toHaveLength(0);
    expect(store.state()).toBe('done');
    expect(store.result()).toEqual(result);
  });

  it('resume() falls back to tracking after a transient snapshot error', () => {
    localStorage.setItem(KEY, 'saved');
    const store = setup();
    snapshot.mockReturnValue(
      throwError(() => new ApiRequestError(503, 'unavailable', 'down')),
    );
    store.resume();
    expect(watchCalls).toEqual([['saved', undefined]]);
    expect(store.token()).toBe('saved');
  });

  it('resume() does nothing without a stored token', () => {
    const store = setup();
    store.resume();
    expect(watchCalls).toHaveLength(0);
    expect(store.state()).toBe('idle');
  });

  it('track works when localStorage throws', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('denied');
    });
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied');
    });
    const store = setup();
    store.track('t');
    last().next([ev(1)]);
    expect(store.lastSeq()).toBe(1);
    store.resume();
  });

  it('track works when localStorage is absent', () => {
    vi.stubGlobal('localStorage', undefined);
    const store = setup();
    store.track('t');
    last().next([ev(1)]);
    expect(store.lastSeq()).toBe(1);
  });

  it('stop() tears down the stream and forgets the token', () => {
    const store = setup();
    store.track('t');
    const stream = last();
    store.stop();
    expect(stream.observed).toBe(false);
    expect(store.state()).toBe('idle');
    expect(store.token()).toBeNull();
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  describe('snapshot polling errors', () => {
    const fail = (status: number, retryAfter?: number) =>
      throwError(
        () => new ApiRequestError(status, 'unavailable', 'x', retryAfter),
      );

    beforeEach(() => {
      vi.useFakeTimers();
      vi.stubGlobal('WebSocket', undefined);
    });

    it('retries transient errors with backoff and honours retryAfter', () => {
      const store = setup();
      snapshot
        .mockReturnValueOnce(fail(503))
        .mockReturnValueOnce(fail(0))
        .mockReturnValueOnce(fail(429, 7))
        .mockReturnValueOnce(fail(503, 600))
        .mockReturnValueOnce(of(view({ state: 'running' })));
      store.track('t');
      vi.advanceTimersByTime(0);
      expect(snapshot).toHaveBeenCalledTimes(1);
      vi.advanceTimersByTime(999);
      expect(snapshot).toHaveBeenCalledTimes(1);
      vi.advanceTimersByTime(1);
      expect(snapshot).toHaveBeenCalledTimes(2);
      vi.advanceTimersByTime(2000);
      expect(snapshot).toHaveBeenCalledTimes(3);
      vi.advanceTimersByTime(6999);
      expect(snapshot).toHaveBeenCalledTimes(3);
      vi.advanceTimersByTime(1);
      expect(snapshot).toHaveBeenCalledTimes(4);
      expect(store.state()).toBe('queued');
      expect(store.error()).toBeNull();
      expect(store.token()).toBe('t');
      vi.advanceTimersByTime(29_999);
      expect(snapshot).toHaveBeenCalledTimes(4);
      vi.advanceTimersByTime(1);
      expect(snapshot).toHaveBeenCalledTimes(5);
      expect(store.state()).toBe('running');
    });

    it('sets error and stops after 5 consecutive failures', () => {
      const store = setup();
      snapshot.mockReturnValue(fail(503));
      store.track('t');
      vi.advanceTimersByTime(120_000);
      expect(snapshot).toHaveBeenCalledTimes(5);
      expect(store.error()?.status).toBe(503);
      expect(store.token()).toBe('t');
    });
  });

  it('only resets reconnect attempts after a fresh event', () => {
    vi.useFakeTimers();
    const store = setup();
    store.track('t');
    last().next([ev(1)]);
    last().error({ code: 1006 });
    vi.advanceTimersByTime(1000);
    expect(watchCalls).toHaveLength(2);
    last().next([ev(1)]);
    last().error({ code: 1006 });
    vi.advanceTimersByTime(1000);
    expect(watchCalls).toHaveLength(2);
    vi.advanceTimersByTime(1000);
    expect(watchCalls).toHaveLength(3);
    last().next([ev(2)]);
    last().error({ code: 1006 });
    vi.advanceTimersByTime(1000);
    expect(watchCalls).toHaveLength(4);
    expect(store.lastSeq()).toBe(2);
  });

  it('track(B) after track(A) never interleaves A', () => {
    vi.useFakeTimers();
    const store = setup();
    store.track('a');
    const a = last();
    a.next([ev(1)]);
    a.error({ code: 1006 });
    store.track('b');
    expect(a.observed).toBe(false);
    last().next([ev(1, 'state', { stage: 'queued' })]);
    vi.advanceTimersByTime(60_000);
    expect(watchCalls).toEqual([
      ['a', undefined],
      ['b', undefined],
    ]);
    expect(store.token()).toBe('b');
    expect(store.events()).toEqual([ev(1, 'state', { stage: 'queued' })]);
  });

  it('stop() during a pending reconnect leaves no timer or socket', () => {
    vi.useFakeTimers();
    const store = setup();
    store.track('t');
    last().error({ code: 1013 });
    store.stop();
    expect(vi.getTimerCount()).toBe(0);
    vi.advanceTimersByTime(60_000);
    expect(watchCalls).toHaveLength(1);
    expect(streams.every((s) => !s.observed)).toBe(true);
  });

  it('keeps non-progress events and only the latest 200 progress events', () => {
    const store = setup();
    store.track('t');
    last().next([ev(1, 'state', { stage: 'running' })]);
    last().next(Array.from({ length: 250 }, (_, i) => ev(i + 2)));
    last().next([ev(252, 'result', { payload: result })]);
    const events = store.events();
    expect(events).toHaveLength(202);
    expect(events[0].seq).toBe(1);
    expect(events[1].seq).toBe(52);
    expect(events.at(-1)?.type).toBe('result');
    expect(events.filter((e) => e.type === 'progress')).toHaveLength(200);
    expect(store.lastSeq()).toBe(252);
  });
});

function m(seq: number, stage: string) {
  return { seq, stage, message: stage, at: '2026-01-01T00:00:00Z' };
}

function view(v: Partial<SubscriptionView>): SubscriptionView {
  return {
    kind: 'extract',
    state: 'queued',
    milestones: [],
    last_seq: 0,
    ...v,
  };
}
