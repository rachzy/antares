import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router, UrlTree } from '@angular/router';
import { isObservable, firstValueFrom, Observable } from 'rxjs';
import { type Step, RunStore } from '../../../stores/run.store';
import { stepGuard } from './step.guard';

describe('stepGuard', () => {
  const unlocked = signal<Step>('select');
  const ready = signal(true);

  beforeEach(() => {
    unlocked.set('select');
    ready.set(true);
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: RunStore, useValue: { unlocked, ready } },
      ],
    });
  });

  const run = (step: Step) =>
    TestBed.runInInjectionContext(
      () => stepGuard(step)({} as never, {} as never) as Observable<unknown>,
    );

  const decide = (step: Step) => firstValueFrom(run(step));

  const target = (result: unknown) =>
    TestBed.inject(Router).serializeUrl(result as UrlTree);

  it('returns an observable', () => {
    expect(isObservable(run('select'))).toBe(true);
  });

  it('always allows select', async () => {
    expect(await decide('select')).toBe(true);
  });

  it.each(['download', 'process', 'score'] as const)(
    'redirects %s to /select on a fresh run',
    async (step) => {
      expect(target(await decide(step))).toBe('/select');
    },
  );

  it('redirects a step ahead of the unlocked one to the unlocked one', async () => {
    unlocked.set('process');
    expect(target(await decide('score'))).toBe('/process');
  });

  it('allows steps up to the unlocked one', async () => {
    unlocked.set('process');
    expect(await decide('download')).toBe(true);
    expect(await decide('process')).toBe(true);
  });

  it('waits for the restore, then allows a deep link to a restored step', async () => {
    ready.set(false);
    unlocked.set('download');
    let result: unknown = 'pending';
    run('score').subscribe((r) => (result = r));
    await Promise.resolve();
    TestBed.tick();
    expect(result).toBe('pending');

    unlocked.set('score');
    ready.set(true);
    TestBed.tick();
    expect(result).toBe(true);
  });
});
