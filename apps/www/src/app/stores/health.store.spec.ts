import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import { HealthStore } from './health.store';

const health = (ready: boolean) => ({
  status: 'ok',
  ready,
  shaula_version: '1',
  fang_version: '2',
});

describe('HealthStore', () => {
  afterEach(() => vi.useRealTimers());

  function setup(health: () => unknown) {
    TestBed.configureTestingModule({
      providers: [{ provide: ApiService, useValue: { health } }],
    });
    return TestBed.inject(HealthStore);
  }

  it('stores health and exposes ready', () => {
    const store = setup(() =>
      of({ status: 'ok', ready: true, shaula_version: '1', fang_version: '2' }),
    );
    store.check();
    expect(store.health()?.ready).toBe(true);
    expect(store.ready()).toBe(true);
    expect(store.loading()).toBe(false);
    expect(store.error()).toBeNull();
  });

  it('is not ready before a check', () => {
    expect(setup(() => of()).ready()).toBe(false);
  });

  it('surfaces errors', () => {
    const store = setup(() =>
      throwError(() => new ApiRequestError(503, 'unavailable', 'down')),
    );
    store.check();
    expect(store.error()?.code).toBe('unavailable');
    expect(store.ready()).toBe(false);
  });

  it('re-checks every 3s while not ready and stops once ready', () => {
    vi.useFakeTimers();
    const calls = vi
      .fn()
      .mockReturnValueOnce(of(health(false)))
      .mockReturnValueOnce(
        throwError(() => new ApiRequestError(0, 'unavailable', 'down')),
      )
      .mockReturnValue(of(health(true)));
    const store = setup(calls);
    store.check();
    expect(calls).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(2999);
    expect(calls).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1);
    expect(calls).toHaveBeenCalledTimes(2);
    expect(store.error()?.code).toBe('unavailable');
    vi.advanceTimersByTime(3000);
    expect(calls).toHaveBeenCalledTimes(3);
    expect(store.ready()).toBe(true);
    expect(store.error()).toBeNull();
    vi.advanceTimersByTime(30_000);
    expect(calls).toHaveBeenCalledTimes(3);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('stops re-checking when the injector is destroyed', () => {
    vi.useFakeTimers();
    const calls = vi.fn(() => of(health(false)));
    setup(calls).check();
    TestBed.resetTestingModule();
    vi.advanceTimersByTime(30_000);
    expect(calls).toHaveBeenCalledTimes(1);
  });
});
