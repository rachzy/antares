import { ApplicationRef, computed, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import {
  ActivatedRoute,
  convertToParamMap,
  provideRouter,
  Router,
} from '@angular/router';
import { toast } from '@spartan-ng/brain/sonner';
import { ApiRequestError } from '../../../api/api.service';
import type { StarDetails, Target } from '../../../api/api.types';
import { RunStore } from '../../../stores/run.store';
import { StarInfoStore } from '../../../stores/star-info.store';
import { TargetStore } from '../../../stores/target.store';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';
import { KEPLER_ZOOM_FOV_DEG, MAX_CONFIRM_FOV_DEG, Select } from './select';

vi.mock('@spartan-ng/brain/sonner', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  toast: { error: vi.fn(), success: vi.fn() },
}));

const callbacks = new Map<string, () => void>();
let fov = 60;
let center: [number, number] = [291.03, 48.14];
const gotoObject = vi.fn();
const setFov = vi.fn();

vi.mock('aladin-lite', () => ({
  default: {
    get init() {
      return Promise.resolve();
    },
    aladin: () => ({
      pix2world: vi.fn(),
      getFov: () => [fov, fov / 2],
      getRaDec: () => center,
      gotoObject,
      setFov,
      on: (event: string, cb: () => void) => callbacks.set(event, cb),
    }),
  },
}));

const star: Target = {
  catalog: 'KIC',
  catalog_id: '11446443',
  display_name: 'Kepler-1 b',
  designation: 'KIC 11446443',
  ra_deg: 286.81,
  dec_deg: 49.32,
};

describe('Select', () => {
  const target = signal<Target | null>(null);
  const loading = signal(false);
  const error = signal<ApiRequestError | null>(null);
  const resolveByPosition = vi.fn();
  const located = signal<Target | null>(null);
  const locateError = signal<ApiRequestError | null>(null);
  const locate = vi.fn();
  const confirmed = signal<Target | null>(null);
  const confirm = vi.fn((t: Target) => confirmed.set(t));
  const reset = vi.fn(() => confirmed.set(null));
  const loadInfo = vi.fn();
  const resetInfo = vi.fn();
  const details = signal<StarDetails | null>(null);
  const startExtract = vi.fn();
  const extractStatus = signal('idle');
  const scoreStatus = signal('idle');

  beforeEach(async () => {
    Element.prototype.scrollIntoView = vi.fn();
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe = vi.fn();
        unobserve = vi.fn();
        disconnect = vi.fn();
      },
    );
    callbacks.clear();
    fov = 60;
    center = [291.03, 48.14];
    confirmed.set(null);
    target.set(null);
    loading.set(false);
    error.set(null);
    resolveByPosition.mockReset();
    located.set(null);
    locateError.set(null);
    locate.mockReset();
    confirm.mockClear();
    startExtract.mockReset();
    loadInfo.mockReset();
    resetInfo.mockReset();
    details.set(null);
    extractStatus.set('idle');
    scoreStatus.set('idle');
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.success).mockClear();
    gotoObject.mockReset();
    setFov.mockReset();
    reset.mockClear();

    await TestBed.configureTestingModule({
      imports: [Select],
      providers: [
        provideRouter([]),
        {
          provide: TargetStore,
          useValue: { target, loading, error, resolveByPosition },
        },
        {
          provide: StarInfoStore,
          useValue: {
            target: located,
            error: locateError,
            details,
            loading: signal(false),
            failed: signal(false),
            locate,
            load: loadInfo,
            reset: resetInfo,
          },
        },
        {
          provide: RunStore,
          useValue: {
            star: confirmed,
            confirm,
            reset,
            startExtract,
            extract: computed(() => ({ status: extractStatus() })),
            score: computed(() => ({ status: scoreStatus() })),
          },
        },
      ],
    }).compileComponents();
  });

  const mount = async () => {
    const fixture = TestBed.createComponent(Select);
    await fixture.whenStable();
    await vi.waitFor(() => expect(callbacks.has('zoomChanged')).toBe(true));
    const el = fixture.nativeElement as HTMLElement;
    const zoom = async (deg: number) => {
      fov = deg;
      callbacks.get('zoomChanged')?.();
      await fixture.whenStable();
    };
    const button = (text: string) =>
      Array.from(el.querySelectorAll<HTMLElement>('button, a')).find((b) =>
        b.textContent?.includes(text),
      );
    return { fixture, el, zoom, button };
  };

  const hit = async () => {
    const page = await mount();
    await page.zoom(MAX_CONFIRM_FOV_DEG / 2);
    page.button('Confirm star')?.click();
    located.set(star);
    await page.fixture.whenStable();
    return page;
  };

  it('disables Confirm and hints to zoom while the view is too wide', async () => {
    const { el, button } = await mount();
    expect(button('Confirm star')?.hasAttribute('disabled')).toBe(true);
    expect(el.textContent).toContain(
      'Zoom in until a single star sits inside the ring.',
    );
  });

  it('enables Confirm once the view is narrow enough', async () => {
    const { el, zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    expect(button('Confirm star')?.hasAttribute('disabled')).toBe(false);
    expect(el.textContent).not.toContain('Zoom in until');
  });

  it('looks up the sky center without asking Shaula when Confirm is clicked', async () => {
    const { zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    button('Confirm star')?.click();
    expect(locate).toHaveBeenCalledWith(291.03, 48.14);
    expect(resolveByPosition).not.toHaveBeenCalled();
  });

  it('reads the sky center at click time', async () => {
    const { zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    center = [10.5, -20.25];
    button('Confirm star')?.click();
    expect(locate).toHaveBeenCalledWith(10.5, -20.25);
  });

  it('shows the already confirmed star on return', async () => {
    confirmed.set(star);
    const { el, button } = await mount();
    expect(el.querySelector('h2')?.textContent).toContain('Kepler-1 b');
    expect(button('Extract lightcurve features')).toBeTruthy();
    expect(confirm).not.toHaveBeenCalled();
  });

  it('replaces the card when another star is confirmed', async () => {
    const page = await hit();
    const other: Target = {
      ...star,
      display_name: 'Kepler-2',
      designation: 'KIC 2',
    };
    page.button('Confirm star')?.click();
    located.set(null);
    located.set(other);
    await page.fixture.whenStable();
    expect(page.el.querySelector('h2')?.textContent).toContain('Kepler-2');
    expect(confirm).toHaveBeenCalledTimes(2);
    expect(confirm).toHaveBeenLastCalledWith(other);
  });

  it('disables Confirm while looking the star up', async () => {
    const { fixture, zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    button('Confirm star')?.click();
    await fixture.whenStable();
    expect(button('Confirm star')?.hasAttribute('disabled')).toBe(true);
  });

  it('shows a spinner on Confirm while looking the star up', async () => {
    const { fixture, el, zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    expect(button('Confirm star')?.querySelector('[data-spinner]')).toBeNull();
    button('Confirm star')?.click();
    await fixture.whenStable();
    expect(
      button('Confirm star')?.querySelector('[data-spinner]'),
    ).toBeTruthy();
    expect(el.querySelector('[data-spinner]')?.className).toContain(
      'animate-spin',
    );
  });

  it('toasts a success when a star is found', async () => {
    await hit();
    expect(toast.success).toHaveBeenCalledWith('Found Kepler-1 b');
  });

  it('searches the sky by name', async () => {
    gotoObject.mockImplementation((_q, cb) => cb.success([1, 2]));
    const { fixture, el } = await mount();
    const input = el.querySelector<HTMLInputElement>('input[role="combobox"]')!;
    input.value = ' Kepler-186 ';
    input.dispatchEvent(new Event('input'));
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    await fixture.whenStable();
    expect(gotoObject).toHaveBeenCalledWith('Kepler-186', expect.any(Object));
    expect(toast.error).not.toHaveBeenCalled();
  });

  const search = async (text: string) => {
    gotoObject.mockImplementation((_q, cb) => cb.success([1, 2]));
    const page = await mount();
    const input = page.el.querySelector<HTMLInputElement>(
      'input[role="combobox"]',
    )!;
    input.value = text;
    input.dispatchEvent(new Event('input'));
    page.el.querySelector('form')!.dispatchEvent(new Event('submit'));
    await page.fixture.whenStable();
  };

  it('drops the previous star when a new search starts', async () => {
    confirmed.set(star);
    await search('Vega');
    expect(reset).toHaveBeenCalled();
    expect(confirmed()).toBeNull();
  });

  it('keeps a running job when searching again', async () => {
    confirmed.set(star);
    extractStatus.set('running');
    await search('Vega');
    expect(reset).not.toHaveBeenCalled();
  });

  it('zooms in on a Kepler star only', async () => {
    await search('Kepler-186');
    expect(setFov).toHaveBeenCalledWith(KEPLER_ZOOM_FOV_DEG);
    expect(gotoObject).toHaveBeenCalledWith('Kepler-186', expect.any(Object));
  });

  it('leaves the zoom alone for other objects', async () => {
    await search('Vega');
    expect(setFov).not.toHaveBeenCalled();
  });

  it('toasts when the search finds nothing', async () => {
    gotoObject.mockImplementation((_q, cb) => cb.error());
    const { fixture, el } = await mount();
    const input = el.querySelector<HTMLInputElement>('input[role="combobox"]')!;
    input.value = 'zzz';
    input.dispatchEvent(new Event('input'));
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    await fixture.whenStable();
    expect(toast.error).toHaveBeenCalledWith('No object named "zzz" found');
  });

  it('lists matching Kepler stars when the search is focused', async () => {
    const { fixture, el } = await mount();
    const input = el.querySelector<HTMLInputElement>('input[role="combobox"]')!;
    input.dispatchEvent(new FocusEvent('focusin', { bubbles: true }));
    input.value = 'Kepler-18';
    input.dispatchEvent(new Event('input'));
    await fixture.whenStable();
    const options = [...document.querySelectorAll('[role="option"]')].map((o) =>
      o.textContent?.trim(),
    );
    expect(options).toContain('Kepler-186');
    expect(options).toContain('Kepler-1866');
    expect(options).not.toContain('Kepler-118');
  });

  it('flies to the star named in the q query parameter once the sky is ready', async () => {
    gotoObject.mockImplementation((_q, cb) => cb.success([1, 2]));
    TestBed.overrideProvider(ActivatedRoute, {
      useValue: {
        snapshot: { queryParamMap: convertToParamMap({ q: 'Kepler-186' }) },
      },
    });
    const { el } = await mount();
    await vi.waitFor(() =>
      expect(gotoObject).toHaveBeenCalledWith('Kepler-186', expect.any(Object)),
    );
    expect(gotoObject).toHaveBeenCalledTimes(1);
    expect(
      el.querySelector<HTMLInputElement>('input[role="combobox"]')?.value,
    ).toBe('Kepler-186');
  });

  it('searches the sky when a suggestion is picked', async () => {
    gotoObject.mockImplementation((_q, cb) => cb.success([1, 2]));
    const { fixture, el } = await mount();
    const input = el.querySelector<HTMLInputElement>('input[role="combobox"]')!;
    input.dispatchEvent(new FocusEvent('focusin', { bubbles: true }));
    input.value = 'Kepler-186';
    input.dispatchEvent(new Event('input'));
    await fixture.whenStable();
    document.querySelector<HTMLElement>('[role="option"]')!.click();
    await fixture.whenStable();
    expect(gotoObject).toHaveBeenCalledWith('Kepler-186', expect.any(Object));
    input.dispatchEvent(new FocusEvent('focusin', { bubbles: true }));
    await fixture.whenStable();
    expect(document.querySelector('[role="option"]')).toBeNull();
  });

  it('searches for what was typed, not an earlier highlighted suggestion, on Enter', async () => {
    gotoObject.mockImplementation((_q, cb) => cb.success([1, 2]));
    const { fixture, el } = await mount();
    const input = el.querySelector<HTMLInputElement>('input[role="combobox"]')!;
    input.dispatchEvent(new FocusEvent('focusin', { bubbles: true }));
    for (const text of ['K', 'Kepler-1', 'Kepler-11']) {
      input.value = text;
      input.dispatchEvent(new Event('input'));
      await fixture.whenStable();
    }
    input.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }),
    );
    await fixture.whenStable();
    expect(gotoObject).toHaveBeenCalledTimes(1);
    expect(gotoObject).toHaveBeenCalledWith('Kepler-11', expect.any(Object));
  });

  it('searches the suggestion chosen with the arrow keys once on Enter', async () => {
    gotoObject.mockImplementation((_q, cb) => cb.success([1, 2]));
    const { fixture, el } = await mount();
    const input = el.querySelector<HTMLInputElement>('input[role="combobox"]')!;
    input.dispatchEvent(new FocusEvent('focusin', { bubbles: true }));
    input.value = 'Kepler-18';
    input.dispatchEvent(new Event('input'));
    await fixture.whenStable();
    const key = (k: string) =>
      input.dispatchEvent(
        new KeyboardEvent('keydown', { key: k, bubbles: true }),
      );
    key('ArrowDown');
    await fixture.whenStable();
    key('Enter');
    await fixture.whenStable();
    expect(gotoObject).toHaveBeenCalledTimes(1);
    expect(gotoObject).toHaveBeenCalledWith('Kepler-18', expect.any(Object));
  });

  it('stays closed after an outside click until the input is focused again', async () => {
    const { fixture, el } = await mount();
    const input = el.querySelector<HTMLInputElement>('input[role="combobox"]')!;
    input.dispatchEvent(new FocusEvent('focusin', { bubbles: true }));
    await fixture.whenStable();
    expect(document.querySelector('[role="option"]')).not.toBeNull();
    document.body.dispatchEvent(
      new PointerEvent('pointerdown', { bubbles: true }),
    );
    document.body.click();
    input.dispatchEvent(new FocusEvent('focusin', { bubbles: true }));
    await fixture.whenStable();
    expect(document.querySelector('[role="option"]')).toBeNull();
  });

  it('ignores an empty search', async () => {
    const { fixture, el } = await mount();
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    await fixture.whenStable();
    expect(gotoObject).not.toHaveBeenCalled();
  });

  it('disables Confirm when the sky failed to load', async () => {
    const { fixture, zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    fixture.debugElement
      .query(By.directive(SkyViewer))
      .componentInstance.loadError.set(true);
    await fixture.whenStable();
    expect(button('Confirm star')?.hasAttribute('disabled')).toBe(true);
  });

  it('shows the star card and confirms the star once on a hit', async () => {
    const { el, button } = await hit();
    expect(el.querySelector('h2')?.textContent).toContain('Kepler-1 b');
    expect(el.textContent).toContain('KIC 11446443');
    const link = button('View on SIMBAD') as HTMLAnchorElement;
    expect(link.getAttribute('href')).toBe(
      'https://simbad.cds.unistra.fr/simbad/sim-id?Ident=KIC%2011446443',
    );
    expect(link.target).toBe('_blank');
    expect(link.rel).toContain('noopener');
    expect(button('Extract lightcurve features')).toBeTruthy();
    expect(confirm).toHaveBeenCalledTimes(1);
    expect(confirm).toHaveBeenCalledWith(star);
  });

  it('greys the designation after a common name', async () => {
    confirmed.set({
      ...star,
      display_name: 'Antares',
      designation: 'HIP 80763',
    });
    const { el } = await mount();
    expect(el.querySelector('h2')?.textContent).toContain('Antares');
    const grey = el.querySelector('h2 [data-designation]');
    expect(grey?.textContent?.trim()).toBe('(HIP 80763)');
    expect(grey?.className).toContain('text-muted-foreground');
  });

  it('shows only the designation when the star has no common name', async () => {
    confirmed.set({
      ...star,
      display_name: 'HIP 80763',
      designation: 'HIP 80763',
    });
    const { el } = await mount();
    expect(el.querySelector('h2')?.textContent?.trim()).toBe('HIP 80763');
    expect(el.querySelector('[data-designation]')).toBeNull();
  });

  it('loads the details of the confirmed star and shows them in the card', async () => {
    const { el } = await hit();
    expect(loadInfo).toHaveBeenCalledWith(star);
    details.set({
      aliases: ['Kepler-1'],
      spectral_type: 'G0V',
      planets: [],
      sources: ['SIMBAD'],
    });
    await TestBed.inject(ApplicationRef).whenStable();
    expect(el.querySelector('an-star-details')?.textContent).toContain('G0V');
  });

  it('clears the details while no star is confirmed', async () => {
    await mount();
    expect(resetInfo).toHaveBeenCalled();
    expect(loadInfo).not.toHaveBeenCalled();
  });

  it('ignores a star located before Confirm was clicked', async () => {
    located.set(star);
    const { el } = await mount();
    expect(el.textContent).not.toContain('Extract lightcurve features');
    expect(confirm).not.toHaveBeenCalled();
  });

  it('keeps the full sky and toasts the error when no star is found', async () => {
    const { fixture, el, zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    button('Confirm star')?.click();
    locateError.set(
      new ApiRequestError(404, 'not_found', 'No star found at that position'),
    );
    await fixture.whenStable();
    expect(el.querySelector('h2')).toBeNull();
    expect(toast.error).toHaveBeenCalledWith('No star found at that position');
    expect(confirm).not.toHaveBeenCalled();
    expect(button('Confirm star')?.hasAttribute('disabled')).toBe(false);
  });

  it('asks Shaula for the star only when Extract is clicked', async () => {
    const { button } = await hit();
    expect(resolveByPosition).not.toHaveBeenCalled();
    button('Extract lightcurve features')?.click();
    expect(resolveByPosition).toHaveBeenCalledWith(star.ra_deg, star.dec_deg);
    expect(startExtract).not.toHaveBeenCalled();
  });

  it('starts the extraction with the resolved star and moves to Download', async () => {
    const navigate = vi
      .spyOn(TestBed.inject(Router), 'navigate')
      .mockResolvedValue(true);
    const { fixture, button } = await hit();
    button('Extract lightcurve features')?.click();
    const resolved: Target = { ...star, catalog: 'TIC' };
    target.set(resolved);
    await fixture.whenStable();
    expect(startExtract).toHaveBeenCalledTimes(1);
    expect(startExtract).toHaveBeenCalledWith(false, resolved);
    expect(navigate).toHaveBeenCalledWith(['/download']);
  });

  it('shows the error and stays on the page when the star has no light curve data', async () => {
    const navigate = vi
      .spyOn(TestBed.inject(Router), 'navigate')
      .mockResolvedValue(true);
    const { fixture, el, button } = await hit();
    button('Extract lightcurve features')?.click();
    await fixture.whenStable();
    expect(
      button('Extract lightcurve features')?.hasAttribute('disabled'),
    ).toBe(true);
    error.set(new ApiRequestError(422, 'unprocessable', 'No light curve data'));
    await fixture.whenStable();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'No light curve data',
    );
    expect(startExtract).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
    expect(
      button('Extract lightcurve features')?.hasAttribute('disabled'),
    ).toBe(false);
  });

  it('leaves Download all data points unchecked and sends the choice on Extract', async () => {
    vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    const { el, fixture, button } = await hit();
    const box = el.querySelector<HTMLElement>('hlm-checkbox button');
    expect(el.textContent).toContain('Download all data points');
    expect(box?.getAttribute('aria-checked')).toBe('false');
    button('Extract lightcurve features')?.click();
    target.set(star);
    await fixture.whenStable();
    expect(startExtract).toHaveBeenLastCalledWith(false, star);
    box?.click();
    await fixture.whenStable();
    target.set(null);
    button('Extract lightcurve features')?.click();
    target.set(star);
    await fixture.whenStable();
    expect(startExtract).toHaveBeenLastCalledWith(true, star);
  });

  const dialog = () =>
    document.body.querySelector<HTMLElement>(
      '[data-slot="alert-dialog-content"]',
    );
  const dialogButton = (text: string) =>
    Array.from(document.body.querySelectorAll<HTMLElement>('button')).find(
      (b) => b.textContent?.includes(text),
    );
  const other: Target = { ...star, display_name: 'Kepler-2', catalog_id: '2' };
  const pick = async (page: Awaited<ReturnType<typeof hit>>) => {
    page.button('Confirm star')?.click();
    located.set(null);
    located.set(other);
    await page.fixture.whenStable();
  };

  it('replaces the star without asking when no job is running', async () => {
    extractStatus.set('done');
    scoreStatus.set('failed');
    const page = await hit();
    await pick(page);
    expect(dialog()).toBeNull();
    expect(confirm).toHaveBeenLastCalledWith(other);
  });

  it.each([
    ['extract', extractStatus],
    ['score', scoreStatus],
  ] as const)(
    'asks before discarding a running %s and replaces on confirm',
    async (_, status) => {
      const page = await hit();
      status.set('running');
      await pick(page);
      expect(confirm).toHaveBeenCalledTimes(1);
      await vi.waitFor(() => expect(dialog()).not.toBeNull());
      dialogButton('Replace and discard the running job')?.click();
      await page.fixture.whenStable();
      expect(confirm).toHaveBeenCalledTimes(2);
      expect(confirm).toHaveBeenLastCalledWith(other);
    },
  );

  it('keeps the running job when the replacement is declined', async () => {
    const page = await hit();
    extractStatus.set('running');
    await pick(page);
    await vi.waitFor(() => expect(dialog()).not.toBeNull());
    dialogButton('Keep current job')?.click();
    await page.fixture.whenStable();
    await vi.waitFor(() => expect(dialog()).toBeNull());
    expect(confirm).toHaveBeenCalledTimes(1);
    expect(page.el.querySelector('h2')?.textContent).toContain('Kepler-1 b');
  });

  it('toasts once when the sky viewer fails to load', async () => {
    const { fixture } = await mount();
    const sky = fixture.debugElement.query(By.directive(SkyViewer))
      .componentInstance as SkyViewer;
    sky.loadError.set(true);
    await fixture.whenStable();
    sky.loadError.set(false);
    await fixture.whenStable();
    sky.loadError.set(true);
    await fixture.whenStable();
    expect(toast.error).toHaveBeenCalledTimes(1);
    expect(toast.error).toHaveBeenCalledWith('Unable to load the sky viewer.');
  });
});
