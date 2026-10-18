import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router, UrlTree } from '@angular/router';
import { type Step, RunStore } from '../../../stores/run.store';
import { stepGuard } from './step.guard';

describe('stepGuard', () => {
  const unlocked = signal<Step>('select');

  beforeEach(() => {
    unlocked.set('select');
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: RunStore, useValue: { unlocked } },
      ],
    });
  });

  const run = (step: Step) =>
    TestBed.runInInjectionContext(() =>
      stepGuard(step)({} as never, {} as never),
    );

  const target = (result: unknown) =>
    TestBed.inject(Router).serializeUrl(result as UrlTree);

  it('always allows select', () => {
    expect(run('select')).toBe(true);
  });

  it.each(['download', 'process', 'score'] as const)(
    'redirects %s to /select on a fresh run',
    (step) => {
      expect(target(run(step))).toBe('/select');
    },
  );

  it('redirects a step ahead of the unlocked one to the unlocked one', () => {
    unlocked.set('process');
    expect(target(run('score'))).toBe('/process');
  });

  it('allows steps up to the unlocked one', () => {
    unlocked.set('process');
    expect(run('download')).toBe(true);
    expect(run('process')).toBe(true);
  });
});
