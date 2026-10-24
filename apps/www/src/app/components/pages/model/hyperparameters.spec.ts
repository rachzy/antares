import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import type { Run, RunHyperparameters } from '../../../api/api.types';
import { ModelStore } from '../../../stores/model.store';
import { Hyperparameters, show } from './hyperparameters';

const run: Run = {
  run_id: 'r1',
  created_at: '2026-09-01T12:00:00Z',
  schema_version: 2,
  selected_model: 'stack',
  threshold: 0.4,
};

const settings: RunHyperparameters = {
  run_id: 'r1',
  selected_model: 'stack',
  threshold: 0.4,
  best_params: {
    meta: { C: 1 },
    lightgbm: { n_estimators: 300, learning_rate: 0.05 },
    catboost: { depth: 6 },
  },
};

describe('Hyperparameters', () => {
  const runs = signal<Run[]>([run]);
  const runsLoaded = signal(true);
  const runId = signal<string | null>('r1');
  const hyperparameters = signal<RunHyperparameters | null>(settings);
  const loading = signal(false);
  const loadRuns = vi.fn();
  const loadHyperparameters = vi.fn();

  const mount = async () => {
    TestBed.configureTestingModule({
      providers: [
        {
          provide: ModelStore,
          useValue: {
            runs,
            runsLoaded,
            runId,
            hyperparameters,
            loading,
            loadRuns,
            loadHyperparameters,
            selectRun: vi.fn(),
          },
        },
      ],
    });
    const fixture = TestBed.createComponent(Hyperparameters);
    await fixture.whenStable();
    return fixture.nativeElement as HTMLElement;
  };

  beforeEach(() => {
    runs.set([run]);
    runsLoaded.set(true);
    runId.set('r1');
    hyperparameters.set(settings);
    loading.set(false);
    loadRuns.mockClear();
    loadHyperparameters.mockClear();
  });

  it('loads the settings of the chosen run, and the runs only when not loaded', async () => {
    await mount();
    expect(loadHyperparameters).toHaveBeenCalledWith('r1');
    expect(loadRuns).not.toHaveBeenCalled();
  });

  it('loads the runs on first use', async () => {
    runsLoaded.set(false);
    runs.set([]);
    runId.set(null);
    await mount();
    expect(loadRuns).toHaveBeenCalledOnce();
    expect(loadHyperparameters).not.toHaveBeenCalled();
  });

  it('summarises the run', async () => {
    const el = await mount();
    const facts = Array.from(el.querySelectorAll('dd')).map((d) =>
      d.textContent?.trim(),
    );
    expect(facts).toEqual(['stack', '0.4', '2', '2026-09-01 12:00']);
  });

  it('shows one table per learner in a stable order and flags the selected one', async () => {
    const el = await mount();
    const cards = Array.from(el.querySelectorAll('[data-learner]'));
    expect(cards.map((c) => c.querySelector('h2')?.textContent)).toEqual([
      'LightGBM',
      'CatBoost',
      'Stack (meta-learner)',
    ]);
    expect(cards.map((c) => !!c.querySelector('[data-selected-chip]'))).toEqual(
      [false, false, true],
    );
    const rows = Array.from(cards[0].querySelectorAll('tr')).map((r) =>
      Array.from(r.querySelectorAll('td')).map((c) => c.textContent?.trim()),
    );
    expect(rows).toEqual([
      ['n_estimators', '300'],
      ['learning_rate', '0.05'],
    ]);
  });

  it('says so when no model is trained yet', async () => {
    runs.set([]);
    hyperparameters.set(null);
    const el = await mount();
    expect(el.querySelector('[data-empty]')?.textContent).toContain(
      'No model has been trained yet.',
    );
  });

  it('formats values', () => {
    expect(show(null)).toBe('—');
    expect(show(undefined)).toBe('—');
    expect(show(true)).toBe('true');
    expect(show({ a: 1 })).toBe('{"a":1}');
  });
});
