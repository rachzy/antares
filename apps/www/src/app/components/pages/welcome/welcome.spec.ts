import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { toast } from '@spartan-ng/brain/sonner';
import { ApiRequestError } from '../../../api/api.service';
import type { Target } from '../../../api/api.types';
import { RunStore } from '../../../stores/run.store';
import { StarInfoStore } from '../../../stores/star-info.store';
import { Welcome } from './welcome';

vi.mock('@spartan-ng/brain/sonner', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock('aladin-lite', () => ({
  default: {
    get init() {
      return Promise.resolve();
    },
    aladin: () => ({
      pix2world: vi.fn(),
      getFov: () => [60, 30],
      getRaDec: () => [0, 0],
      on: vi.fn(),
    }),
  },
}));

describe('Welcome', () => {
  afterEach(() => vi.unstubAllGlobals());

  beforeEach(async () => {
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe = vi.fn();
        unobserve = vi.fn();
        disconnect = vi.fn();
      },
    );
    await TestBed.configureTestingModule({
      imports: [Welcome],
      providers: [provideRouter([{ path: 'select', children: [] }])],
    }).compileComponents();
  });

  const mount = async () => {
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();
    return fixture.nativeElement as HTMLElement;
  };

  it('explains what Antares does', async () => {
    const el = await mount();
    expect(el.querySelector('h1')?.textContent).toContain('Antares');
    expect(el.querySelector('.welcome-lede')?.textContent).toContain(
      'hunting exoplanets',
    );
  });

  it('offers a star search', async () => {
    const el = await mount();
    expect(el.querySelector('input')?.getAttribute('placeholder')).toContain(
      'Kepler star',
    );
  });

  it('suggests a few stars that open the select page on that star', async () => {
    const el = await mount();
    const links = Array.from(
      el.querySelectorAll<HTMLAnchorElement>('section a[href^="/select?q="]'),
    );
    expect(links.map((a) => a.textContent?.trim())).toEqual([
      'Kepler-90',
      'Kepler-186',
      'Kepler-22',
    ]);
    expect(links[1].getAttribute('href')).toBe('/select?q=Kepler-186');
  });

  it('invites to look closer at the select page', async () => {
    const el = await mount();
    const link = el.querySelector<HTMLAnchorElement>('[data-look-closer]');
    expect(link?.textContent).toContain('Look closer');
    expect(link?.getAttribute('href')).toBe('/select');
  });

  it('opens the select page on a picked star', async () => {
    const navigate = vi
      .spyOn(TestBed.inject(Router), 'navigate')
      .mockResolvedValue(true);
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();
    (fixture.componentInstance as unknown as { pick(n: string): void }).pick(
      'Kepler-186',
    );
    expect(navigate).toHaveBeenCalledWith(['/select'], {
      queryParams: { q: 'Kepler-186' },
    });
  });

  it('opens the select page on a typed name no suggestion matches', async () => {
    const navigate = vi
      .spyOn(TestBed.inject(Router), 'navigate')
      .mockResolvedValue(true);
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();
    const page = fixture.componentInstance as unknown as {
      query: { set(v: string): void };
      submit(): void;
    };
    page.query.set('  Tabby star zzz ');
    page.submit();
    expect(navigate).toHaveBeenCalledWith(['/select'], {
      queryParams: { q: 'Tabby star zzz' },
    });
  });
  describe('manual lookup', () => {
    const located = signal<Target | null>(null);
    const locateError = signal<ApiRequestError | null>(null);
    const lookup = vi.fn();
    const locate = vi.fn();
    const confirm = vi.fn();
    const extractStatus = signal('idle');
    const kepler186 = {
      display_name: 'Kepler-186',
      ra_deg: 298.65,
      dec_deg: 43.95,
    } as Target;

    beforeEach(() => {
      located.set(null);
      locateError.set(null);
      extractStatus.set('idle');
      lookup.mockReset();
      locate.mockReset();
      confirm.mockReset();
      TestBed.overrideProvider(StarInfoStore, {
        useValue: { target: located, error: locateError, lookup, locate },
      });
      TestBed.overrideProvider(RunStore, {
        useValue: {
          confirm,
          extract: () => ({ status: extractStatus() }),
          score: () => ({ status: 'idle' }),
        },
      });
    });

    const open = async () => {
      const navigate = vi
        .spyOn(TestBed.inject(Router), 'navigate')
        .mockResolvedValue(true);
      const fixture = TestBed.createComponent(Welcome);
      await fixture.whenStable();
      const el = fixture.nativeElement as HTMLElement;
      el.querySelector<HTMLButtonElement>('[data-mode="manual"]')!.click();
      await fixture.whenStable();
      const fill = (id: string, value: string) => {
        const input = el.querySelector<HTMLInputElement>(`#${id}`)!;
        input.value = value;
        input.dispatchEvent(new Event('input'));
      };
      const submit = async () => {
        el.querySelector('an-manual-select form')!.dispatchEvent(
          new Event('submit'),
        );
        await fixture.whenStable();
      };
      return { el, fixture, navigate, fill, submit };
    };

    it('looks the name up and stays put while loading', async () => {
      const page = await open();
      expect(page.el.querySelector('[data-look-closer]')).toBeNull();
      page.fill('welcome-name', 'Kepler-186');
      await page.submit();
      expect(lookup).toHaveBeenCalledWith('Kepler-186');
      expect(page.navigate).not.toHaveBeenCalled();
      expect(page.el.querySelector('[data-spinner]')).not.toBeNull();
      expect(
        page.el.querySelector<HTMLInputElement>('#welcome-name')!.disabled,
      ).toBe(true);
    });

    it('confirms the star and opens the select page once found', async () => {
      const page = await open();
      page.fill('welcome-name', 'Kepler-186');
      await page.submit();
      located.set(kepler186);
      await page.fixture.whenStable();
      expect(confirm).toHaveBeenCalledWith(kepler186);
      expect(page.navigate).toHaveBeenCalledWith(['/select']);
    });

    it('looks coordinates up the same way', async () => {
      const page = await open();
      page.fill('welcome-ra', '298.65');
      page.fill('welcome-dec', '43.95');
      await page.submit();
      expect(locate).toHaveBeenCalledWith(298.65, 43.95);
      expect(page.navigate).not.toHaveBeenCalled();
    });

    it('stays on the form and toasts when the lookup fails', async () => {
      const page = await open();
      page.fill('welcome-name', 'Nope');
      await page.submit();
      locateError.set(new ApiRequestError(404, 'not_found', 'No such star'));
      await page.fixture.whenStable();
      expect(toast.error).toHaveBeenCalledWith('No such star');
      expect(page.navigate).not.toHaveBeenCalled();
      expect(page.el.querySelector('[data-spinner]')).toBeNull();
    });

    it('hands over to the select page while a job runs', async () => {
      extractStatus.set('running');
      const page = await open();
      page.fill('welcome-name', 'Kepler-186');
      await page.submit();
      located.set(kepler186);
      await page.fixture.whenStable();
      expect(confirm).not.toHaveBeenCalled();
      expect(page.navigate).toHaveBeenCalledWith(['/select'], {
        queryParams: { star: 'Kepler-186' },
      });
    });
  });
});
