import { TestBed } from '@angular/core/testing';
import { SkyViewer } from './sky-viewer';

const aladinMock = vi.fn();
const pix2worldMock = vi.fn();
const getFovMock = vi.fn();
const getRaDecMock = vi.fn();
const callbacks = new Map<string, () => void>();
let initPromise: Promise<void>;

vi.mock('aladin-lite', () => ({
  default: {
    get init() {
      return initPromise;
    },
    aladin: (element: HTMLElement, options: unknown) => {
      aladinMock(element, options);
      return {
        pix2world: pix2worldMock,
        getFov: getFovMock,
        getRaDec: getRaDecMock,
        on: (event: string, cb: () => void) => callbacks.set(event, cb),
      };
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

describe('SkyViewer', () => {
  beforeEach(async () => {
    aladinMock.mockClear();
    pix2worldMock.mockReset();
    getFovMock.mockReset();
    getRaDecMock.mockReset();
    callbacks.clear();
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
        mode: 'dark',
        showReticle: true,
        reticleColor: '#f54900',
        reticleSize: 24,
      });
    });
  });

  it('tracks the field of view as the user zooms', async () => {
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    expect(fixture.componentInstance.fovDeg()).toBe(60);

    await vi.waitFor(() => expect(callbacks.has('zoomChanged')).toBe(true));
    getFovMock.mockReturnValue([0.02, 0.01]);
    callbacks.get('zoomChanged')?.();

    expect(fixture.componentInstance.fovDeg()).toBe(0.02);
  });

  it('reports the sky center once Aladin is ready', async () => {
    let resolveInit!: () => void;
    initPromise = new Promise((resolve) => {
      resolveInit = resolve;
    });
    getRaDecMock.mockReturnValue([83.82, -5.39]);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    expect(fixture.componentInstance.center()).toBeNull();

    resolveInit();
    await vi.waitFor(() =>
      expect(fixture.componentInstance.center()).toEqual([83.82, -5.39]),
    );
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
