import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { BrandState } from '../../ui/brand';
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
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [Welcome],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  it('explains what Antares does', async () => {
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('.welcome__lede')?.textContent).toContain(
      'hunting exoplanets',
    );
  });

  it('invites a click to look closer', async () => {
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('.welcome__expand')?.textContent).toContain(
      'Look closer',
    );
  });

  it('fades the description and lifts the title, then opens the select page', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }));
    const brand = TestBed.inject(BrandState);
    const navigate = vi
      .spyOn(TestBed.inject(Router), 'navigate')
      .mockResolvedValue(true);
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    vi.useFakeTimers();
    compiled.querySelector<HTMLButtonElement>('.welcome__expand')?.click();
    fixture.detectChanges();

    expect(brand.leaving()).toBe(true);
    expect(compiled.querySelector('.welcome__lede--leaving')).not.toBeNull();
    expect(navigate).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(700);
    expect(navigate).toHaveBeenCalledWith(['/select']);
    expect(brand.leaving()).toBe(false);
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });
});
