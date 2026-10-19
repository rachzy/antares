import { inject } from '@angular/core';
import { toObservable } from '@angular/core/rxjs-interop';
import { CanActivateFn, Router } from '@angular/router';
import { filter, map, take } from 'rxjs';
import { RunStore, type Step } from '../../../stores/run.store';

export const PIPELINE_STEPS = [
  'select',
  'download',
  'process',
  'score',
] as const;

export function stepGuard(step: Step): CanActivateFn {
  return () => {
    const run = inject(RunStore);
    const router = inject(Router);
    const decide = () => {
      const unlocked = run.unlocked();
      return PIPELINE_STEPS.indexOf(step) <= PIPELINE_STEPS.indexOf(unlocked)
        ? true
        : router.createUrlTree(['/', unlocked]);
    };
    if (run.ready()) return decide();
    return toObservable(run.ready).pipe(filter(Boolean), take(1), map(decide));
  };
}
