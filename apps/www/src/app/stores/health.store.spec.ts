import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { ApiRequestError, ApiService } from '../api/api.service';
import { HealthStore } from './health.store';

describe('HealthStore', () => {
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
});
