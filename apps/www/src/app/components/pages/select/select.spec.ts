import { computed, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideRouter, Router } from '@angular/router';
import { toast } from '@spartan-ng/brain/sonner';
import { ApiRequestError } from '../../../api/api.service';
import type { Target } from '../../../api/api.types';
import { RunStore } from '../../../stores/run.store';
import { TargetStore } from '../../../stores/target.store';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';
import { MAX_CONFIRM_FOV_DEG, Select } from './select';

vi.mock('@spartan-ng/brain/sonner', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  toast: { error: vi.fn(), success: vi.fn() },
}));

const callbacks = new Map<string, () => void>();
let fov = 60;
let center: [number, number] = [291.03, 48.14];
const gotoObject = vi.fn();

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
  const confirmed = signal<Target | null>(null);
  const confirm = vi.fn((t: Target) => confirmed.set(t));
  const startExtract = vi.fn();
  const extractStatus = signal('idle');
  const scoreStatus = signal('idle');

  beforeEach(async () => {
    callbacks.clear();
    fov = 60;
    center = [291.03, 48.14];
    confirmed.set(null);
    target.set(null);
    loading.set(false);
    error.set(null);
    resolveByPosition.mockReset();
    confirm.mockClear();
    startExtract.mockReset();
    extractStatus.set('idle');
    scoreStatus.set('idle');
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.success).mockClear();
    gotoObject.mockReset();

    await TestBed.configureTestingModule({
      imports: [Select],
      providers: [
        provideRouter([]),
        {
          provide: TargetStore,
          useValue: { target, loading, error, resolveByPosition },
        },
        {
          provide: RunStore,
          useValue: {
            star: confirmed,
            confirm,
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
    target.set(star);
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

  it('resolves the sky center when Confirm is clicked', async () => {
    const { zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    button('Confirm star')?.click();
    expect(resolveByPosition).toHaveBeenCalledWith(291.03, 48.14);
  });

  it('reads the sky center at click time', async () => {
    const { zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    center = [10.5, -20.25];
    button('Confirm star')?.click();
    expect(resolveByPosition).toHaveBeenCalledWith(10.5, -20.25);
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
    target.set(null);
    target.set(other);
    await page.fixture.whenStable();
    expect(page.el.querySelector('h2')?.textContent).toContain('Kepler-2');
    expect(confirm).toHaveBeenCalledTimes(2);
    expect(confirm).toHaveBeenLastCalledWith(other);
  });

  it('disables Confirm while resolving', async () => {
    const { fixture, zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    loading.set(true);
    await fixture.whenStable();
    expect(button('Confirm star')?.hasAttribute('disabled')).toBe(true);
  });

  it('shows a spinner on Confirm while resolving', async () => {
    const { fixture, el, zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    expect(button('Confirm star')?.querySelector('[data-spinner]')).toBeNull();
    loading.set(true);
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
    const input = el.querySelector<HTMLInputElement>('input[type="search"]')!;
    input.value = ' Kepler-186 ';
    input.dispatchEvent(new Event('input'));
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    await fixture.whenStable();
    expect(gotoObject).toHaveBeenCalledWith('Kepler-186', expect.any(Object));
    expect(toast.error).not.toHaveBeenCalled();
  });

  it('toasts when the search finds nothing', async () => {
    gotoObject.mockImplementation((_q, cb) => cb.error());
    const { fixture, el } = await mount();
    const input = el.querySelector<HTMLInputElement>('input[type="search"]')!;
    input.value = 'zzz';
    input.dispatchEvent(new Event('input'));
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    await fixture.whenStable();
    expect(toast.error).toHaveBeenCalledWith('No object named "zzz" found');
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

  it('ignores a target resolved before Confirm was clicked', async () => {
    target.set(star);
    const { el } = await mount();
    expect(el.textContent).not.toContain('Extract lightcurve features');
    expect(confirm).not.toHaveBeenCalled();
  });

  it('keeps the full sky and shows no inline error on a miss', async () => {
    const { fixture, el, zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    button('Confirm star')?.click();
    error.set(new ApiRequestError(404, 'not_found', 'No star found'));
    await fixture.whenStable();
    expect(el.querySelector('h2')).toBeNull();
    expect(el.textContent).not.toContain('No star found');
    expect(confirm).not.toHaveBeenCalled();
  });

  it('starts the extraction and moves to Download on Extract', async () => {
    const navigate = vi
      .spyOn(TestBed.inject(Router), 'navigate')
      .mockResolvedValue(true);
    const { button } = await hit();
    button('Extract lightcurve features')?.click();
    expect(startExtract).toHaveBeenCalledTimes(1);
    expect(navigate).toHaveBeenCalledWith(['/download']);
  });

  it('leaves Download all data points unchecked and sends the choice on Extract', async () => {
    vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    const { el, fixture, button } = await hit();
    const box = el.querySelector<HTMLElement>('hlm-checkbox button');
    expect(el.textContent).toContain('Download all data points');
    expect(box?.getAttribute('aria-checked')).toBe('false');
    button('Extract lightcurve features')?.click();
    expect(startExtract).toHaveBeenLastCalledWith(false);
    box?.click();
    await fixture.whenStable();
    button('Extract lightcurve features')?.click();
    expect(startExtract).toHaveBeenLastCalledWith(true);
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
    target.set(null);
    target.set(other);
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
