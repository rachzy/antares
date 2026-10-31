import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { ApiService } from '../api/api.service';
import type { StarInfo, Target } from '../api/api.types';
import { StarInfoStore } from './star-info.store';

const target: Target = {
  catalog: 'KIC',
  catalog_id: '11442793',
  display_name: 'Kepler-90',
  designation: 'KIC 11442793',
  ra_deg: 284.43,
  dec_deg: 49.3,
};
const info: StarInfo = {
  target,
  star: { aliases: ['KOI-351'], planets: [], sources: ['SIMBAD'] },
};

describe('StarInfoStore', () => {
  const starInfo = vi.fn();
  const starInfoByQuery = vi.fn();

  beforeEach(() => {
    starInfo.mockReset();
    starInfoByQuery.mockReset();
    TestBed.configureTestingModule({
      providers: [
        { provide: ApiService, useValue: { starInfo, starInfoByQuery } },
      ],
    });
  });

  it('asks the API for the star by its position and keeps the details', () => {
    starInfo.mockReturnValue(of(info));
    const store = TestBed.inject(StarInfoStore);
    store.load(target);
    expect(starInfo).toHaveBeenCalledWith(284.43, 49.3);
    expect(store.details()).toEqual(info.star);
    expect(store.loading()).toBe(false);
    expect(store.failed()).toBe(false);
  });

  it('locates the star at a position and keeps the target the API found', () => {
    starInfo.mockReturnValue(of(info));
    const store = TestBed.inject(StarInfoStore);
    store.locate(284.4301, 49.3002);
    expect(starInfo).toHaveBeenCalledWith(284.4301, 49.3002);
    expect(store.target()).toEqual(target);
    expect(store.details()).toEqual(info.star);
  });

  it('looks a star up by name and keeps the target the API found', () => {
    starInfoByQuery.mockReturnValue(of(info));
    const store = TestBed.inject(StarInfoStore);
    store.lookup('Kepler-90');
    expect(starInfoByQuery).toHaveBeenCalledWith('Kepler-90');
    expect(starInfo).not.toHaveBeenCalled();
    expect(store.target()).toEqual(target);
    expect(store.details()).toEqual(info.star);
  });

  it('does not ask again for the star it just located', () => {
    starInfo.mockReturnValue(of(info));
    const store = TestBed.inject(StarInfoStore);
    store.locate(284.4301, 49.3002);
    store.load(target);
    expect(starInfo).toHaveBeenCalledTimes(1);
  });

  it('loads a star other than the one it holds', () => {
    starInfo.mockReturnValue(of(info));
    const store = TestBed.inject(StarInfoStore);
    store.locate(284.4301, 49.3002);
    store.load({ ...target, ra_deg: 10, dec_deg: 20 });
    expect(starInfo).toHaveBeenCalledTimes(2);
  });

  it('marks a failed lookup without keeping stale details', () => {
    const store = TestBed.inject(StarInfoStore);
    starInfo.mockReturnValueOnce(of(info));
    store.load(target);
    const error = new Error('down');
    starInfo.mockReturnValueOnce(throwError(() => error));
    store.load({ ...target, ra_deg: 10 });
    expect(store.details()).toBeNull();
    expect(store.target()).toBeNull();
    expect(store.failed()).toBe(true);
    expect(store.error()).toBe(error);
    expect(store.loading()).toBe(false);
  });

  it('reset clears everything', () => {
    starInfo.mockReturnValue(of(info));
    const store = TestBed.inject(StarInfoStore);
    store.load(target);
    store.reset();
    expect(store.details()).toBeNull();
    expect(store.target()).toBeNull();
  });
});
