import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideRouter, Router } from '@angular/router';
import { ApiRequestError } from '../../../api/api.service';
import type { Target } from '../../../api/api.types';
import { RunStore } from '../../../stores/run.store';
import { TargetStore } from '../../../stores/target.store';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';
import { MAX_CONFIRM_FOV_DEG, Select } from './select';

const callbacks = new Map<string, () => void>();
let fov = 60;

vi.mock('aladin-lite', () => ({
  default: {
    get init() {
      return Promise.resolve();
    },
    aladin: () => ({
      pix2world: vi.fn(),
      getFov: () => [fov, fov / 2],
      getRaDec: () => [291.03, 48.14],
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
  const confirm = vi.fn();
  const startExtract = vi.fn();

  beforeEach(async () => {
    callbacks.clear();
    fov = 60;
    target.set(null);
    loading.set(false);
    error.set(null);
    resolveByPosition.mockReset();
    confirm.mockReset();
    startExtract.mockReset();

    await TestBed.configureTestingModule({
      imports: [Select],
      providers: [
        provideRouter([]),
        {
          provide: TargetStore,
          useValue: { target, loading, error, resolveByPosition },
        },
        { provide: RunStore, useValue: { confirm, startExtract } },
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

  const confirmed = async () => {
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

  it('disables Confirm while resolving', async () => {
    const { fixture, zoom, button } = await mount();
    await zoom(MAX_CONFIRM_FOV_DEG / 2);
    loading.set(true);
    await fixture.whenStable();
    expect(button('Confirm star')?.hasAttribute('disabled')).toBe(true);
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
    const { el, button } = await confirmed();
    expect(el.querySelector('h2')?.textContent).toContain('Kepler-1 b');
    expect(el.textContent).toContain('KIC 11446443');
    const link = button('View on Stellar Catalog') as HTMLAnchorElement;
    expect(link.getAttribute('href')).toBe(
      'https://www.stellarcatalog.com/stars/Kepler-1%20b',
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
    const { button } = await confirmed();
    button('Extract lightcurve features')?.click();
    expect(startExtract).toHaveBeenCalledTimes(1);
    expect(navigate).toHaveBeenCalledWith(['/download']);
  });
});
