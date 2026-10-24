import { DatePipe } from '@angular/common';
import { Component, computed } from '@angular/core';
import { HlmTableImports } from '@spartan-ng/helm/table';
import { RunPicker } from '../../ui/run-picker';
import { useRunData } from './model-page';

const LEARNERS: Record<string, string> = {
  lightgbm: 'LightGBM',
  catboost: 'CatBoost',
  extra_trees: 'ExtraTrees',
  meta: 'Stack (meta-learner)',
};

export function show(value: unknown): string {
  if (value === null || value === undefined) return '—';
  return typeof value === 'object' ? JSON.stringify(value) : String(value);
}

@Component({
  selector: 'an-hyperparameters',
  imports: [DatePipe, HlmTableImports, RunPicker],
  templateUrl: './hyperparameters.html',
  host: { class: 'block p-6 md:p-10' },
})
export class Hyperparameters {
  protected readonly model = useRunData((store, runId) =>
    store.loadHyperparameters(runId),
  );
  protected readonly settings = this.model.hyperparameters;
  protected readonly run = computed(() =>
    this.model.runs().find((r) => r.run_id === this.model.runId()),
  );
  protected readonly learners = computed(() => {
    const settings = this.settings();
    if (!settings) return [];
    const selected =
      settings.selected_model === 'stack' ? 'meta' : settings.selected_model;
    const keys = Object.keys(settings.best_params);
    const known = Object.keys(LEARNERS).filter((k) => keys.includes(k));
    return [...known, ...keys.filter((k) => !known.includes(k))].map((key) => ({
      key,
      name: LEARNERS[key] ?? key,
      selected: key === selected,
      rows: Object.entries(settings.best_params[key]).map(([k, v]) => ({
        key: k,
        value: show(v),
      })),
    }));
  });
}
