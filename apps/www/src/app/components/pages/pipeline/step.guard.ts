import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { RunStore, type Step } from '../../../stores/run.store';

export const PIPELINE_STEPS = [
  'select',
  'download',
  'process',
  'score',
] as const;

export function stepGuard(step: Step): CanActivateFn {
  return () => {
    const unlocked = inject(RunStore).unlocked();
    if (PIPELINE_STEPS.indexOf(step) <= PIPELINE_STEPS.indexOf(unlocked)) {
      return true;
    }
    return inject(Router).createUrlTree(['/', unlocked]);
  };
}
