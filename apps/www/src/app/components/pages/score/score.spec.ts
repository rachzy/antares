import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import type {
  JobResult,
  PredictionRow,
  SubscriptionEvent,
} from '../../../api/api.types';
import { RunStore, type Stage } from '../../../stores/run.store';
import { clock } from '../../ui/job-log/job-log';
import { Score } from './score';

const at = '2026-10-18T10:04:02Z';
const ev: SubscriptionEvent = {
  seq: 1,
  type: 'state',
  stage: 'predicting',
  at,
};

const idle: Stage = {
  status: 'idle',
  token: null,
  events: [],
  result: null,
  error: null,
  cached: false,
};

const row = (p: number): PredictionRow => ({
  period_days: 10,
  prob_lightgbm: p,
  prob_stack: p,
  potential_probability: p,
  decision_threshold: 0.5,
  prediction: p >= 0.5 ? 'POTENTIAL' : 'UNLIKELY',
  model_name: 'stack',
});

const result = (predictions: PredictionRow[]): JobResult => ({
  target: {
    catalog: 'KIC',
    catalog_id: '1',
    display_name: 'Kepler-1',
    designation: 'KIC 1',
    ra_deg: 0,
    dec_deg: 0,
  },
  cached: true,
  shaula_version: '1',
  features: [],
  model_run_id: 'run-1',
  schema_version: 1,
  predictions,
});

describe('Score', () => {
  const extract = signal<Stage>(idle);
  const score = signal<Stage>(idle);
  const startScore = vi.fn();
  const retry = vi.fn();

  beforeEach(async () => {
    extract.set({ ...idle, status: 'done' });
    score.set(idle);
    startScore.mockReset();
    retry.mockReset();
    await TestBed.configureTestingModule({
      imports: [Score],
      providers: [
        {
          provide: RunStore,
          useValue: { extract, score, startScore, retry },
        },
      ],
    }).compileComponents();
  });

  const mount = async () => {
    const fixture = TestBed.createComponent(Score);
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const button = (label: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        b.textContent?.includes(label),
      );
    const tabs = () =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
    const panels = () =>
      Array.from(el.querySelectorAll<HTMLElement>('[role="tabpanel"]'));
    return { fixture, el, button, tabs, panels };
  };

  const done = (predictions: PredictionRow[]) =>
    score.set({
      ...idle,
      status: 'done',
      events: [ev],
      result: result(predictions),
    });

  it('shows the log on top and three tabs with Ranking selected', async () => {
    done([row(0.4), row(0.7)]);
    const { el, tabs, panels } = await mount();
    const log = el.querySelector('[data-log]');
    const list = el.querySelector('[role="tablist"]');
    expect(log && list).not.toBeNull();
    if (!log || !list) return;
    expect(
      log.compareDocumentPosition(list) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(tabs().map((t) => t.textContent?.trim())).toEqual([
      'Ranking',
      'Model matrix',
      'Scoring details',
    ]);
    expect(tabs()[0].getAttribute('aria-selected')).toBe('true');
    expect(panels().map((p) => p.hidden)).toEqual([false, true, true]);
    expect(panels()[0].querySelector('an-score-ranking')).not.toBeNull();
    expect(
      Array.from(el.querySelectorAll('[data-line]'))
        .at(-1)
        ?.textContent?.trim(),
    ).toBe(`${clock(at)} done: 2 candidates scored`);
  });

  it('switches to the model matrix and the details', async () => {
    done([row(0.4)]);
    const { fixture, tabs, panels } = await mount();
    tabs()[1].click();
    await fixture.whenStable();
    expect(panels().map((p) => p.hidden)).toEqual([true, false, true]);
    expect(panels()[1].querySelector('an-score-matrix')).not.toBeNull();
    tabs()[2].click();
    await fixture.whenStable();
    expect(panels()[2].querySelector('an-score-details')).not.toBeNull();
  });

  it('offers Score candidates instead of tabs when scoring has not run', async () => {
    const { el, button } = await mount();
    expect(el.querySelector('[role="tablist"]')).toBeNull();
    button('Score candidates')?.click();
    expect(startScore).toHaveBeenCalledOnce();
  });

  it('offers Retry when scoring failed', async () => {
    score.set({ ...idle, status: 'failed', events: [ev] });
    const { el, button } = await mount();
    expect(el.querySelector('[role="tablist"]')).toBeNull();
    button('Retry')?.click();
    expect(retry).toHaveBeenCalledOnce();
  });

  it('shows the log only while scoring runs', async () => {
    score.set({ ...idle, status: 'running', events: [ev] });
    const { el, button } = await mount();
    expect(el.querySelector('[data-log]')).not.toBeNull();
    expect(el.querySelector('[role="tablist"]')).toBeNull();
    expect(button('Score candidates')).toBeUndefined();
  });

  it('says No candidates scored with zero predictions', async () => {
    done([]);
    const { el } = await mount();
    expect(el.textContent).toContain('No candidates scored');
    expect(el.querySelector('[role="tablist"]')).toBeNull();
  });

  it('shows loaded from cache for a cached score', async () => {
    score.set({
      ...idle,
      status: 'done',
      cached: true,
      result: result([row(0.6)]),
    });
    const { el } = await mount();
    expect(
      Array.from(el.querySelectorAll('[data-line]')).map((l) =>
        l.textContent?.trim(),
      ),
    ).toEqual(['loaded from cache']);
  });
});
