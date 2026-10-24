import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { Welcome } from './welcome';

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
});
