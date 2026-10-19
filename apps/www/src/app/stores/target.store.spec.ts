import { TestBed } from '@angular/core/testing';
import { of, Subject, throwError } from 'rxjs';
import type { ResolveRequest, Target } from '../api/api.types';
import { ApiRequestError, ApiService } from '../api/api.service';
import { TargetStore } from './target.store';

const target: Target = {
  catalog: 'KIC',
  catalog_id: '1',
  display_name: 'Kepler-1',
  designation: 'KIC 1',
  ra_deg: 1,
  dec_deg: 2,
};

describe('TargetStore', () => {
  function setup(resolveTarget: (b: ResolveRequest) => unknown) {
    TestBed.configureTestingModule({
      providers: [{ provide: ApiService, useValue: { resolveTarget } }],
    });
    return TestBed.inject(TargetStore);
  }

  it('resolveByName sets target and clears error', () => {
    const calls: ResolveRequest[] = [];
    let fail = true;
    const store = setup((b) => {
      calls.push(b);
      return fail
        ? throwError(() => new ApiRequestError(404, 'not_found', 'nope'))
        : of(target);
    });
    store.resolveByName('missing');
    expect(store.error()).not.toBeNull();
    fail = false;
    store.resolveByName('Kepler-1', 'Kepler');
    expect(store.target()).toEqual(target);
    expect(store.error()).toBeNull();
    expect(calls[1]).toEqual({ query: 'Kepler-1', mission: 'Kepler' });
  });

  it('not_found sets error.code and leaves target null', () => {
    const store = setup(() =>
      throwError(() => new ApiRequestError(404, 'not_found', 'nope')),
    );
    store.resolveByName('x');
    expect(store.error()?.code).toBe('not_found');
    expect(store.target()).toBeNull();
    expect(store.loading()).toBe(false);
  });

  it('resolveByPosition sends coordinates', () => {
    const calls: ResolveRequest[] = [];
    const store = setup((b) => {
      calls.push(b);
      return of(target);
    });
    store.resolveByPosition(10, 20, 'TESS');
    expect(calls[0]).toEqual({ ra_deg: 10, dec_deg: 20, mission: 'TESS' });
    expect(store.target()).toEqual(target);
  });

  it('clears the previous target while resolving and when resolving fails', () => {
    let pending = new Subject<Target>();
    const store = setup(() => pending);
    store.resolveByPosition(1, 2);
    pending.next(target);
    pending.complete();
    expect(store.target()).toEqual(target);

    pending = new Subject<Target>();
    store.resolveByPosition(3, 4);
    expect(store.target()).toBeNull();
    expect(store.loading()).toBe(true);

    pending.error(new ApiRequestError(404, 'not_found', 'nope'));
    expect(store.target()).toBeNull();
    expect(store.loading()).toBe(false);
  });

  it('reset forgets the target and any error', () => {
    const store = setup(() => of(target));
    store.resolveByName('Kepler-1');
    store.reset();
    expect(store.target()).toBeNull();
    expect(store.error()).toBeNull();
    expect(store.loading()).toBe(false);
  });
});
