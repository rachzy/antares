import { DecimalPipe } from '@angular/common';
import { Component, computed, signal } from '@angular/core';
import { RunPicker } from '../../ui/run-picker';
import { useRunData } from './model-page';

const MODELS: Record<string, string> = {
  lightgbm: 'LightGBM',
  catboost: 'CatBoost',
  extra_trees: 'ExtraTrees',
  stack: 'Stack',
};

const SOURCES = {
  held_out: 'Pooled over stars held out of training.',
  cross_fit: 'Cross-fitted predictions on the training stars.',
};

@Component({
  selector: 'an-dataset',
  imports: [DecimalPipe, RunPicker],
  templateUrl: './dataset.html',
  host: { class: 'block p-6 md:p-10' },
})
export class Dataset {
  protected readonly model = useRunData((store, runId) =>
    store.loadDataset(runId),
  );
  protected readonly data = this.model.dataset;
  private readonly chosen = signal<string | null>(null);

  protected readonly models = computed(() => {
    const data = this.data();
    if (!data) return [];
    const keys = Object.keys(data.confusion_matrices);
    const order = Object.keys(MODELS).filter((k) => keys.includes(k));
    return [...order, ...keys.filter((k) => !order.includes(k))].map((key) => ({
      key,
      name: MODELS[key] ?? key,
      selected: key === data.selected_model,
    }));
  });
  protected readonly active = computed(() => {
    const keys = this.models().map((m) => m.key);
    const chosen = this.chosen();
    if (chosen && keys.includes(chosen)) return chosen;
    const selected = this.data()?.selected_model;
    return selected && keys.includes(selected) ? selected : (keys[0] ?? null);
  });
  protected readonly matrix = computed(() => {
    const key = this.active();
    return key ? (this.data()?.confusion_matrices[key] ?? null) : null;
  });
  protected readonly precision = computed(() => {
    const m = this.matrix();
    return m && m.tp + m.fp > 0 ? m.tp / (m.tp + m.fp) : null;
  });
  protected readonly recall = computed(() => {
    const m = this.matrix();
    return m && m.tp + m.fn > 0 ? m.tp / (m.tp + m.fn) : null;
  });
  protected readonly source = computed(() => {
    const m = this.matrix();
    return m ? SOURCES[m.source] : '';
  });
  protected readonly labels = computed(() => {
    const d = this.data();
    if (!d || d.n_rows === 0) return [];
    return [
      { name: 'Planet candidates', count: d.n_positive, tone: 'bg-primary' },
      { name: 'Not planets', count: d.n_negative, tone: 'bg-muted-foreground' },
      { name: 'Unlabelled', count: d.n_unlabelled, tone: 'bg-border' },
    ]
      .filter((l) => l.count > 0)
      .map((l) => ({ ...l, share: (l.count / d.n_rows) * 100 }));
  });

  protected choose(key: string): void {
    this.chosen.set(key);
  }
}
