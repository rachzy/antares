import { TestBed } from '@angular/core/testing';
import { Welcome } from './welcome';

vi.mock('aladin-lite', () => ({
  default: {
    get init() {
      return Promise.resolve();
    },
    aladin: () => ({ pix2world: vi.fn() }),
  },
}));

describe('Welcome', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [Welcome],
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

  it('starts collapsed, inviting a click to look closer', async () => {
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('.welcome')?.classList).not.toContain(
      'welcome--expanded',
    );
    expect(compiled.querySelector('.welcome__expand')).toBeTruthy();
  });

  it('expands the stage to fill the page when clicked', async () => {
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    compiled.querySelector<HTMLButtonElement>('.welcome__expand')?.click();
    fixture.detectChanges();

    expect(compiled.querySelector('.welcome')?.classList).toContain(
      'welcome--expanded',
    );
    expect(compiled.querySelector('.welcome__collapse')).toBeTruthy();
  });

  it('collapses back when the back control is clicked', async () => {
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    compiled.querySelector<HTMLButtonElement>('.welcome__expand')?.click();
    fixture.detectChanges();

    compiled.querySelector<HTMLButtonElement>('.welcome__collapse')?.click();
    fixture.detectChanges();

    expect(compiled.querySelector('.welcome')?.classList).not.toContain(
      'welcome--expanded',
    );
  });

  it('collapses when Escape is pressed while expanded', async () => {
    const fixture = TestBed.createComponent(Welcome);
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    compiled.querySelector<HTMLButtonElement>('.welcome__expand')?.click();
    fixture.detectChanges();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    fixture.detectChanges();

    expect(compiled.querySelector('.welcome')?.classList).not.toContain(
      'welcome--expanded',
    );
  });
});
