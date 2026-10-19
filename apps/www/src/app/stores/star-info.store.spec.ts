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

  beforeEach(() => {
    starInfo.mockReset();
    TestBed.configureTestingModule({
      providers: [{ provide: ApiService, useValue: { starInfo } }],
    });
  });

  it('asks the API for the star by its designation and keeps the details', () => {
    starInfo.mockReturnValue(of(info));
    const store = TestBed.inject(StarInfoStore);
    store.load(target);
    expect(starInfo).toHaveBeenCalledWith('KIC 11442793');
    expect(store.details()).toEqual(info.star);
    expect(store.loading()).toBe(false);
    expect(store.failed()).toBe(false);
  });

  it('marks a failed lookup without keeping stale details', () => {
    const store = TestBed.inject(StarInfoStore);
    starInfo.mockReturnValueOnce(of(info));
    store.load(target);
    starInfo.mockReturnValueOnce(throwError(() => new Error('down')));
    store.load(target);
    expect(store.details()).toBeNull();
    expect(store.failed()).toBe(true);
    expect(store.loading()).toBe(false);
  });

  it('reset clears everything', () => {
    starInfo.mockReturnValue(of(info));
    const store = TestBed.inject(StarInfoStore);
    store.load(target);
    store.reset();
    expect(store.details()).toBeNull();
  });
});
