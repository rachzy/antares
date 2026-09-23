import { TestBed } from '@angular/core/testing';
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

describe('SkyViewer', () => {
  beforeEach(async () => {
    aladinMock.mockClear();
    pix2worldMock.mockReset();
    initPromise = Promise.resolve();

    await TestBed.configureTestingModule({
      imports: [SkyViewer],
    }).compileComponents();
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

  it('shows the clicked RA/Dec when the sky is clicked', async () => {
    pix2worldMock.mockReturnValue([83.8221, -5.3911]);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    skyDiv.dispatchEvent(new MouseEvent('click'));
    fixture.detectChanges();
    await fixture.whenStable();

    expect(pix2worldMock).toHaveBeenCalledWith(0, 0);
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('RA 83.8221°, Dec -5.3911°');
  });

  it('does not show a panel when the click falls outside the sky', async () => {
    pix2worldMock.mockReturnValue(null);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    skyDiv.dispatchEvent(new MouseEvent('click'));
    fixture.detectChanges();
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent?.trim()).toBe('');
  });

  it('removes the click listener when the component is destroyed', async () => {
    pix2worldMock.mockReturnValue([1, 2]);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    fixture.destroy();

    expect(() => skyDiv.dispatchEvent(new MouseEvent('click'))).not.toThrow();
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
