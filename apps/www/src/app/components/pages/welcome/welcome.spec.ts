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
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [Welcome],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  it('introduces Antares', async () => {
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('.welcome__title')?.textContent).toBe(
      'Antares',
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

  it('opens the select page when the stage is clicked', async () => {
    const navigate = vi
      .spyOn(TestBed.inject(Router), 'navigate')
      .mockResolvedValue(true);
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    compiled.querySelector<HTMLButtonElement>('.welcome__expand')?.click();

    expect(navigate).toHaveBeenCalledWith(['/select']);
  });
});
