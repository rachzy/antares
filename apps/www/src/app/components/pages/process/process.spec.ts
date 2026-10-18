import { computed, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideRouter, Router } from '@angular/router';
import { BrnTooltip } from '@spartan-ng/brain/tooltip';
import type { JobResult, SubscriptionEvent } from '../../../api/api.types';
import { RunStore, type Stage } from '../../../stores/run.store';
import { clock } from '../../ui/job-log/job-log';
import { Process, SCORE_TOOLTIP } from './process';

const at = '2026-10-18T10:02:11Z';
const ev = (seq: number, stage: string): SubscriptionEvent => ({
  seq,
  type: 'state',
  stage,
  at,
});

const idle: Stage = {
  status: 'idle',
  token: null,
  events: [],
  result: null,
  error: null,
  cached: false,
};

const result = (features: JobResult['features']): JobResult => ({
  target: {
    catalog: 'KIC',
    catalog_id: '1',
    display_name: 'Kepler-1',
    designation: 'KIC 1',
    ra_deg: 0,
    dec_deg: 0,
  },
  cached: false,
  shaula_version: '1',
  features,
});

const read = (blob: Blob) =>
  new Promise<string>((resolve) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.readAsText(blob);
  });

describe('Process', () => {
  afterEach(() => vi.useRealTimers());
  const extract = signal<Stage>(idle);
  const startScore = vi.fn();
  const retry = vi.fn();

  beforeEach(async () => {
    extract.set({ ...idle, status: 'running' });
    startScore.mockReset();
    retry.mockReset();
    await TestBed.configureTestingModule({
      imports: [Process],
      providers: [
        provideRouter([{ path: 'score', children: [] }]),
        {
          provide: RunStore,
          useValue: {
            extract,
            startScore,
            retry,
            processDone: computed(() => extract().status === 'done'),
            candidates: computed(() => extract().result?.features.length ?? 0),
          },
        },
      ],
    }).compileComponents();
  });

  const mount = async () => {
    const fixture = TestBed.createComponent(Process);
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const lines = () =>
      Array.from(el.querySelectorAll('[data-line]')).map((l) =>
        l.textContent?.replace(/\s+/g, ' ').trim(),
      );
    const button = (text: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        b.textContent?.includes(text),
      );
    return { fixture, el, lines, button };
  };

  const done = (features: JobResult['features']) =>
    extract.set({
      ...idle,
      status: 'done',
      events: [
        ev(1, 'downloading'),
        ev(2, 'detrending'),
        ev(3, 'period_search'),
      ],
      result: result(features),
    });

  it('shows the events from detrending on', async () => {
    extract.set({
      ...idle,
      status: 'running',
      events: [
        ev(1, 'downloading'),
        ev(2, 'detrending'),
        ev(3, 'period_search'),
      ],
    });
    const { lines, button } = await mount();
    expect(lines()).toEqual([
      `${clock(at)} detrending`,
      `${clock(at)} period_search`,
    ]);
    expect(button('Score candidates')).toBeUndefined();
  });

  it('shows the done line and the two actions once processing is done', async () => {
    done([{ a: 1 }, { a: 2 }]);
    const { fixture, lines, button } = await mount();
    expect(lines().at(-1)).toBe(`${clock(at)} done: 2 candidates extracted`);
    const score = button('Score candidates');
    expect(score?.hasAttribute('disabled')).toBe(false);
    expect(button('Download extracted features')).toBeDefined();
    const tooltip = fixture.debugElement
      .query(By.css('[data-score]'))
      .injector.get(BrnTooltip);
    expect(tooltip.brnTooltip()).toBe(SCORE_TOOLTIP);
    expect(SCORE_TOOLTIP).toBe(
      'Runs the extracted data through our machine learning model and shows how likely each candidate is to be a real exoplanet',
    );
  });

  it('starts scoring and navigates to /score', async () => {
    done([{ a: 1 }]);
    const { button } = await mount();
    const navigate = vi.spyOn(TestBed.inject(Router), 'navigateByUrl');
    button('Score candidates')?.click();
    expect(startScore).toHaveBeenCalledOnce();
    expect(navigate).toHaveBeenCalledWith('/score');
  });

  it('downloads the features as features.csv', async () => {
    done([{ a: 'x,y', b: null }]);
    const { button } = await mount();
    const blobs: Blob[] = [];
    URL.createObjectURL = vi.fn((b: Blob) => {
      blobs.push(b);
      return 'blob:features';
    });
    URL.revokeObjectURL = vi.fn();
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(function (this: HTMLAnchorElement) {
        expect(this.download).toBe('features.csv');
        expect(this.href).toBe('blob:features');
        expect(this.isConnected).toBe(true);
      });
    vi.useFakeTimers();
    button('Download extracted features')?.click();
    expect(click).toHaveBeenCalledOnce();
    expect(document.querySelector('a[download]')).toBeNull();
    expect(URL.revokeObjectURL).not.toHaveBeenCalled();
    vi.runAllTimers();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:features');
    expect(blobs[0].type).toContain('text/csv');
    expect(await read(blobs[0])).toBe('a,b\r\n"x,y",\r\n');
    click.mockRestore();
  });

  it('shows No candidates found and disables scoring with zero candidates', async () => {
    done([]);
    const { el, button } = await mount();
    expect(el.textContent).toContain('No candidates found');
    expect(button('Score candidates')?.hasAttribute('disabled')).toBe(true);
  });

  it('shows a single loaded from cache line on a cached run', async () => {
    extract.set({
      ...idle,
      status: 'done',
      cached: true,
      result: result([{ a: 1 }]),
    });
    const { lines } = await mount();
    expect(lines()).toEqual(['loaded from cache']);
  });

  it('offers Retry when the job failed', async () => {
    extract.set({ ...idle, status: 'failed', events: [ev(1, 'detrending')] });
    const { button } = await mount();
    button('Retry')?.click();
    expect(retry).toHaveBeenCalledOnce();
  });
});
