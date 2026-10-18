import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { of, Subject, throwError } from 'rxjs';
import type { Job, JobResult, SubscriptionEvent } from '../api/api.types';
import { ApiRequestError, ApiService } from '../api/api.service';
import type { SubscriptionPhase } from './subscription.store';
import { SubscriptionStore } from './subscription.store';
import { PipelineStore } from './pipeline.store';

const result = { cached: false, features: [] } as unknown as JobResult;
const queued = (token: string, position = 2): Job => ({
  status: 'queued',
  token,
  subscription_path: `/s/${token}`,
  position,
});

describe('PipelineStore', () => {
  const state = signal<SubscriptionPhase>('idle');
  const events = signal<SubscriptionEvent[]>([]);
  const subResult = signal<JobResult | null>(null);
  const subError = signal<ApiRequestError | null>(null);
  const track = vi.fn();
  const stop = vi.fn();

  function setup(api: object) {
    state.set('idle');
    events.set([]);
    subResult.set(null);
    subError.set(null);
    track.mockReset();
    stop.mockReset();
    TestBed.configureTestingModule({
      providers: [
        { provide: ApiService, useValue: api },
        {
          provide: SubscriptionStore,
          useValue: {
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
    return TestBed.inject(PipelineStore);
  }

  it('a 200 done sets the result and never tracks', () => {
    const store = setup({ extract: () => of({ status: 'done', result }) });
    store.extract({ query: 'x' });
    expect(store.result()).toBe(result);
    expect(store.loading()).toBe(false);
    expect(track).not.toHaveBeenCalled();
  });

  it('a 202 tracks the token and stores the position', () => {
    const store = setup({ analyze: () => of(queued('tok', 3)) });
    store.analyze({ query: 'x' });
    expect(track).toHaveBeenCalledWith('tok');
    expect(store.position()).toBe(3);
    expect(store.loading()).toBe(true);
  });

  it('mirrors the subscription result when it is done', () => {
    const store = setup({ extract: () => of(queued('tok')) });
    store.extract({ query: 'x' });
    state.set('done');
    subResult.set(result);
    TestBed.tick();
    expect(store.result()).toBe(result);
    expect(store.loading()).toBe(false);
    expect(store.status()).toBe('done');
  });

  it('surfaces a failed subscription as an error', () => {
    const store = setup({ extract: () => of(queued('tok')) });
    store.extract({ query: 'x' });
    state.set('failed');
    subError.set(new ApiRequestError(0, 'internal', 'boom'));
    TestBed.tick();
    expect(store.error()?.message).toBe('boom');
    expect(store.loading()).toBe(false);
  });

  it('surfaces a dropped subscription as an error', () => {
    const store = setup({ extract: () => of(queued('tok')) });
    store.extract({ query: 'x' });
    state.set('dropped');
    TestBed.tick();
    expect(store.error()?.code).toBe('internal');
    expect(store.loading()).toBe(false);
  });

  it('exposes the latest progress event', () => {
    const store = setup({});
    expect(store.progress()).toBeNull();
    events.set([
      {
        seq: 1,
        type: 'progress',
        message: 'a',
        fraction: 0.1,
        stage: 's',
        at: '',
      },
      { seq: 2, type: 'state', stage: 'running', at: '' },
      {
        seq: 3,
        type: 'progress',
        message: 'b',
        fraction: 0.5,
        stage: 't',
        at: '',
      },
    ]);
    expect(store.progress()).toEqual({
      message: 'b',
      fraction: 0.5,
      stage: 't',
    });
  });

  it('a 503 busy keeps retryAfter and defaults the seconds', () => {
    const store = setup({
      extract: () =>
        throwError(() => new ApiRequestError(503, 'busy', 'busy', 7)),
      analyze: () => throwError(() => new ApiRequestError(503, 'busy', 'busy')),
    });
    store.extract({ query: 'x' });
    expect(store.error()?.retryAfter).toBe(7);
    expect(store.retryAfterSeconds()).toBe(7);
    store.analyze({ query: 'x' });
    expect(store.retryAfterSeconds()).toBe(30);
  });

  it('a 422 sets the error, ends loading and has no retry seconds', () => {
    const store = setup({
      extract: () =>
        throwError(() => new ApiRequestError(422, 'unprocessable', 'bad')),
    });
    store.extract({ query: 'x' });
    expect(store.error()?.code).toBe('unprocessable');
    expect(store.loading()).toBe(false);
    expect(store.retryAfterSeconds()).toBeNull();
  });

  it('a new call replaces the pending one and clears previous state', () => {
    const first = new Subject<Job>();
    const second = new Subject<Job>();
    const streams = [first, second];
    const store = setup({ extract: () => streams.shift() });
    store.extract({ query: 'a' });
    first.next(queued('one'));
    state.set('done');
    subResult.set(result);
    TestBed.tick();
    expect(store.result()).toBe(result);

    store.extract({ query: 'b' });
    expect(store.result()).toBeNull();
    expect(store.position()).toBeNull();
    expect(store.error()).toBeNull();
    expect(stop).toHaveBeenCalled();
    first.next(queued('late'));
    expect(track).not.toHaveBeenCalledWith('late');
    second.next(queued('two'));
    expect(track).toHaveBeenLastCalledWith('two');
  });

  it('reset stops the subscription and clears state', () => {
    const store = setup({ extract: () => of({ status: 'done', result }) });
    store.extract({ query: 'x' });
    stop.mockClear();
    store.reset();
    expect(stop).toHaveBeenCalledTimes(1);
    expect(store.result()).toBeNull();
  });
});
