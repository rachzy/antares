import { effect, inject, untracked } from '@angular/core';
import { ModelStore } from '../../../stores/model.store';

/** Loads the runs on first use and `load`s the page's data for the chosen run. */
export function useRunData(
  load: (store: InstanceType<typeof ModelStore>, runId: string) => void,
) {
  const store = inject(ModelStore);
  if (!store.runsLoaded()) store.loadRuns();
  effect(() => {
    const runId = store.runId();
    if (runId) untracked(() => load(store, runId));
  });
  return store;
}
