import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ApiRequestError } from '../../../api/api.service';
import type { Health, JobResult, Target } from '../../../api/api.types';
import { HealthStore } from '../../../stores/health.store';
import { PipelineStore } from '../../../stores/pipeline.store';
import { TargetStore } from '../../../stores/target.store';
import { SkyViewer } from './sky-viewer';

const aladinMock = vi.fn();
const pix2worldMock = vi.fn();
let initPromise: Promise<void>;

vi.mock('aladin-lite', () => ({
  default: {
    get init() {
      return initPromise;
    },
    aladin: (element: HTMLElement, options: unknown) => {
      aladinMock(element, options);
      return { pix2world: pix2worldMock };
    },
  },
}));

function clickSkyCanvas(
  skyDiv: HTMLDivElement,
  clientX: number,
  clientY: number,
): void {
  const canvas = document.createElement('canvas');
  skyDiv.appendChild(canvas);
  canvas.dispatchEvent(
    new MouseEvent('click', { bubbles: true, clientX, clientY }),
  );
}

const kepler: Target = {
  catalog: 'KIC',
  catalog_id: '8120608',
  display_name: 'Kepler-62',
  designation: 'KIC 8120608',
  ra_deg: 283.2,
  dec_deg: 45.3,
};

function fakeStores() {
  return {
    health: {
      health: signal<Health | null>(null),
      ready: signal(false),
      error: signal<ApiRequestError | null>(null),
    },
    target: {
      target: signal<Target | null>(null),
      loading: signal(false),
      error: signal<ApiRequestError | null>(null),
      resolveByPosition: vi.fn(),
    },
    pipeline: {
      result: signal<JobResult | null>(null),
      position: signal<number | null>(null),
      loading: signal(false),
      error: signal<ApiRequestError | null>(null),
      status: signal('idle'),
      progress: signal<{
        message?: string;
        fraction?: number;
        stage?: string;
      } | null>(null),
      retryAfterSeconds: signal<number | null>(null),
      analyze: vi.fn(),
    },
  };
}

let stores: ReturnType<typeof fakeStores>;

async function renderLoaded() {
  const fixture = TestBed.createComponent(SkyViewer);
  await fixture.whenStable();
  await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());
  fixture.detectChanges();
  await fixture.whenStable();
  return fixture;
}

async function rerender(fixture: {
  detectChanges(): void;
  whenStable(): Promise<unknown>;
}) {
  fixture.detectChanges();
  await fixture.whenStable();
}

describe('SkyViewer', () => {
  beforeEach(async () => {
    aladinMock.mockClear();
    pix2worldMock.mockReset();
    initPromise = Promise.resolve();
    stores = fakeStores();

    await TestBed.configureTestingModule({
      imports: [SkyViewer],
      providers: [
        { provide: HealthStore, useValue: stores.health },
        { provide: TargetStore, useValue: stores.target },
        { provide: PipelineStore, useValue: stores.pipeline },
      ],
    }).compileComponents();
  });

  it('resolves the clicked position through the target store', async () => {
    pix2worldMock.mockReturnValue([83.8221, -5.3911]);
    const fixture = await renderLoaded();

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    clickSkyCanvas(skyDiv, 10, 20);

    expect(stores.target.resolveByPosition).toHaveBeenCalledWith(
      83.8221,
      -5.3911,
    );
  });

  it('shows the resolved target and analyzes it with its mission', async () => {
    stores.target.target.set(kepler);
    const fixture = await renderLoaded();
    const compiled = fixture.nativeElement as HTMLElement;

    expect(compiled.textContent).toContain('Kepler-62');
    expect(compiled.textContent).toContain('KIC 8120608');

    const button = compiled.querySelector(
      'button[hlmBtn]',
    ) as HTMLButtonElement;
    expect(button.disabled).toBe(false);
    button.click();

    expect(stores.pipeline.analyze).toHaveBeenCalledWith({
      ra_deg: 283.2,
      dec_deg: 45.3,
      mission: 'Kepler',
    });
  });

  it('disables Analyze while the pipeline is loading', async () => {
    stores.target.target.set(kepler);
    stores.pipeline.loading.set(true);
    const fixture = await renderLoaded();

    const button = fixture.nativeElement.querySelector(
      'button[hlmBtn]',
    ) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
  });

  it.each(['loading', 'error'] as const)(
    'disables Analyze while the target store has %s',
    async (kind) => {
      stores.target.target.set(kepler);
      if (kind === 'loading') stores.target.loading.set(true);
      else
        stores.target.error.set(
          new ApiRequestError(404, 'not_found', 'No target.'),
        );
      const fixture = await renderLoaded();

      const button = fixture.nativeElement.querySelector(
        'button[hlmBtn]',
      ) as HTMLButtonElement;
      expect(button.disabled).toBe(true);
    },
  );

  it('says the api is unreachable when the health check fails', async () => {
    stores.health.error.set(
      new ApiRequestError(0, 'unavailable', 'Network error'),
    );
    const fixture = await renderLoaded();

    expect(fixture.nativeElement.textContent).toContain(
      'The API is unreachable.',
    );
    expect(fixture.nativeElement.textContent).not.toContain('not ready');
  });

  it('shows the queue position and then the latest progress', async () => {
    stores.target.target.set(kepler);
    stores.pipeline.loading.set(true);
    stores.pipeline.status.set('queued');
    stores.pipeline.position.set(3);
    const fixture = await renderLoaded();
    const compiled = fixture.nativeElement as HTMLElement;

    expect(compiled.textContent).toContain('Queued (position 3)');

    stores.pipeline.status.set('running');
    stores.pipeline.progress.set({
      message: 'Downloading light curves',
      fraction: 0.4,
    });
    await rerender(fixture);

    expect(compiled.textContent).toContain('Running');
    expect(compiled.textContent).toContain('Downloading light curves');
    expect(compiled.textContent).toContain('40%');
    const bar = compiled.querySelector('[role="progressbar"]') as HTMLElement;
    expect(bar.getAttribute('aria-valuenow')).toBe('40');
  });

  it('shows the top prediction when the job is done', async () => {
    stores.target.target.set(kepler);
    stores.pipeline.status.set('done');
    stores.pipeline.result.set({
      target: kepler,
      cached: false,
      shaula_version: '1.0.0',
      features: [],
      predictions: [
        { star_id: 'KIC 8120608', period_days: 129.9, prob_stack: 0.71 },
        { star_id: 'KIC 8120608', period_days: 12.1, prob_stack: 0.2 },
      ],
    });
    const fixture = await renderLoaded();
    const compiled = fixture.nativeElement as HTMLElement;

    expect(compiled.textContent).toContain('prob_stack');
    expect(compiled.textContent).toContain('0.710');
    expect(compiled.textContent).toContain('129.9');
    expect(compiled.textContent).not.toContain('0.200');
  });

  function doneWith(predictions: Record<string, unknown>[] | undefined) {
    stores.target.target.set(kepler);
    stores.pipeline.status.set('done');
    stores.pipeline.result.set({
      target: kepler,
      cached: false,
      shaula_version: '1.0.0',
      features: [],
      predictions,
    });
  }

  it('falls back to the scalar fields when no known prediction field is present', async () => {
    doneWith([
      {
        star_id: 'KIC 8120608',
        label: 'candidate',
        score: 0.9,
        flagged: true,
        extra: { nested: 1 },
      },
    ]);
    const fixture = await renderLoaded();
    const compiled = fixture.nativeElement as HTMLElement;

    const terms = [...compiled.querySelectorAll('dt')].map((dt) =>
      dt.textContent?.trim(),
    );
    expect(terms).toEqual(['label', 'score', 'flagged']);
    expect(compiled.textContent).toContain('candidate');
    expect(compiled.textContent).not.toContain('No predictions returned.');
  });

  it('says no predictions were returned for an empty predictions array', async () => {
    doneWith([]);
    const fixture = await renderLoaded();
    const compiled = fixture.nativeElement as HTMLElement;

    expect(compiled.querySelector('dl')).toBeNull();
    expect(compiled.textContent).toContain('No predictions returned.');
  });

  it('says no predictions were returned when the row has no usable fields', async () => {
    doneWith([{ star_id: 'KIC 8120608', extra: { nested: 1 } }]);
    const fixture = await renderLoaded();
    const compiled = fixture.nativeElement as HTMLElement;

    expect(compiled.querySelector('dl')).toBeNull();
    expect(compiled.textContent).toContain('No predictions returned.');
  });

  it.each([
    [1.4, '100'],
    [-0.2, '0'],
  ])(
    'clamps a progress fraction of %s to %s percent',
    async (fraction, expected) => {
      stores.target.target.set(kepler);
      stores.pipeline.loading.set(true);
      stores.pipeline.status.set('running');
      stores.pipeline.progress.set({ message: 'Working', fraction });
      const fixture = await renderLoaded();
      const compiled = fixture.nativeElement as HTMLElement;

      const bar = compiled.querySelector('[role="progressbar"]') as HTMLElement;
      expect(bar.getAttribute('aria-valuenow')).toBe(expected);
      expect(compiled.textContent).toContain(`(${expected}%)`);
    },
  );

  it('announces status and progress politely and errors as alerts', async () => {
    stores.target.target.set(kepler);
    stores.target.error.set(
      new ApiRequestError(404, 'not_found', 'No target near that position.'),
    );
    stores.pipeline.loading.set(true);
    stores.pipeline.status.set('running');
    stores.pipeline.progress.set({ message: 'Working', fraction: 0.5 });
    stores.pipeline.error.set(
      new ApiRequestError(503, 'busy', 'The queue is full.'),
    );
    const fixture = await renderLoaded();
    const compiled = fixture.nativeElement as HTMLElement;

    const live = [...compiled.querySelectorAll('[aria-live="polite"]')].map(
      (el) => el.textContent,
    );
    expect(live.join(' ')).toContain('Running');
    expect(live.join(' ')).toContain('Working');
    const alerts = [...compiled.querySelectorAll('[role="alert"]')].map(
      (el) => el.textContent,
    );
    expect(alerts.join(' ')).toContain('No target near that position.');
    expect(alerts.join(' ')).toContain('The queue is full.');
  });

  it('shows the pipeline error and the busy retry hint', async () => {
    stores.target.target.set(kepler);
    stores.pipeline.error.set(
      new ApiRequestError(503, 'busy', 'The queue is full.'),
    );
    stores.pipeline.retryAfterSeconds.set(30);
    const fixture = await renderLoaded();
    const compiled = fixture.nativeElement as HTMLElement;

    expect(compiled.textContent).toContain('The queue is full.');
    expect(compiled.textContent).toContain('Try again in 30 s');
  });

  it('shows the target error message', async () => {
    stores.target.error.set(
      new ApiRequestError(404, 'not_found', 'No target near that position.'),
    );
    const fixture = await renderLoaded();

    expect(fixture.nativeElement.textContent).toContain(
      'No target near that position.',
    );
  });

  it('warns when the api reports it is not ready', async () => {
    stores.health.health.set({
      status: 'ok',
      ready: false,
      shaula_version: '',
      fang_version: '',
    });
    const fixture = await renderLoaded();

    expect(fixture.nativeElement.textContent).toContain('not ready');
  });

  it('initializes Aladin with the default survey', async () => {
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();

    await vi.waitFor(() => {
      expect(aladinMock).toHaveBeenCalledWith(expect.any(HTMLDivElement), {
        survey: 'P/DSS2/color',
        fov: 60,
        cooFrame: 'equatorial',
      });
    });
  });

  it('shows a loading message until Aladin finishes initializing', async () => {
    let resolveInit!: () => void;
    initPromise = new Promise((resolve) => {
      resolveInit = resolve;
    });

    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();

    expect(fixture.nativeElement.textContent).toContain('Loading the sky');

    resolveInit();
    await vi.waitFor(() => {
      fixture.detectChanges();
      expect(fixture.nativeElement.textContent?.trim()).toBe('');
    });
  });

  it('shows the clicked RA/Dec when the sky is clicked', async () => {
    pix2worldMock.mockReturnValue([83.8221, -5.3911]);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    clickSkyCanvas(skyDiv, 10, 20);
    fixture.detectChanges();
    await fixture.whenStable();

    expect(pix2worldMock).toHaveBeenCalledWith(10, 20, 'icrs');
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('RA 83.8221°, Dec -5.3911°');
  });

  it('does not show a panel when the click falls outside the sky', async () => {
    pix2worldMock.mockReturnValue(null);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    clickSkyCanvas(skyDiv, 0, 0);
    fixture.detectChanges();
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent?.trim()).toBe('');
  });

  it('does not throw and leaves the panel unchanged when a click falls outside the projected sky disk', async () => {
    pix2worldMock.mockImplementation(() => {
      throw new TypeError('undefined is not iterable');
    });
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    expect(() => clickSkyCanvas(skyDiv, 0, 0)).not.toThrow();
    fixture.detectChanges();
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent?.trim()).toBe('');
  });

  it("ignores clicks on Aladin's own UI controls, not just the sky canvas", async () => {
    pix2worldMock.mockReturnValue([1, 2]);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    const control = document.createElement('button');
    skyDiv.appendChild(control);
    control.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    fixture.detectChanges();
    await fixture.whenStable();

    expect(pix2worldMock).not.toHaveBeenCalled();
  });

  it('removes the click listener when the component is destroyed', async () => {
    pix2worldMock.mockReturnValue([1, 2]);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    fixture.destroy();

    expect(() => clickSkyCanvas(skyDiv, 1, 1)).not.toThrow();
    expect(pix2worldMock).not.toHaveBeenCalled();
  });

  it('shows a fallback message when Aladin fails to initialize', async () => {
    initPromise = Promise.reject(new Error('network error'));
    initPromise.catch(() => {
      /* silence Node's unhandledRejection detection; SkyViewer's own try/catch still observes this rejection */
    });
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();

    await vi.waitFor(() => {
      fixture.detectChanges();
      const compiled = fixture.nativeElement as HTMLElement;
      expect(compiled.textContent).toContain('Unable to load the sky viewer.');
    });
  });
});
