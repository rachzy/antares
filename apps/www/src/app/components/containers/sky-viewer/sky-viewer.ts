import {
  afterNextRender,
  Component,
  DestroyRef,
  ElementRef,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import type { AladinInstance } from 'aladin-lite';

const INITIAL_FOV_DEG = 60;

@Component({
  selector: 'an-sky-viewer',
  templateUrl: './sky-viewer.html',
  styleUrl: './sky-viewer.css',
})
export class SkyViewer {
  protected readonly skyDiv =
    viewChild.required<ElementRef<HTMLDivElement>>('skyDiv');
  protected readonly selectedCoord = signal<{ ra: number; dec: number } | null>(
    null,
  );
  protected readonly loading = signal(true);
  readonly loadError = signal(false);
  private readonly fov = signal(INITIAL_FOV_DEG);
  readonly fovDeg = this.fov.asReadonly();

  private readonly destroyRef = inject(DestroyRef);
  private aladin: AladinInstance | null = null;

  constructor() {
    afterNextRender(() => void this.initAladin());
  }

  center(): [number, number] | null {
    return this.aladin?.getRaDec() ?? null;
  }

  private async initAladin(): Promise<void> {
    try {
      const { default: A } = await import('aladin-lite');
      await A.init;
      const aladin = A.aladin(this.skyDiv().nativeElement, {
        survey: 'P/DSS2/color',
        fov: INITIAL_FOV_DEG,
        cooFrame: 'equatorial',
        mode: 'dark',
        showReticle: true,
        reticleColor: '#f54900',
        reticleSize: 24,
      });
      aladin.on('zoomChanged', () => this.fov.set(aladin.getFov()[0]));
      this.aladin = aladin;
      this.loading.set(false);
      this.attachClickHandler(aladin);
    } catch (error) {
      console.error('Failed to initialize the sky viewer', error);
      this.loadError.set(true);
    }
  }

  private attachClickHandler(aladin: AladinInstance): void {
    const element = this.skyDiv().nativeElement;
    const onClick = (event: MouseEvent) => {
      if (!(event.target instanceof HTMLCanvasElement)) {
        return;
      }

      const rect = element.getBoundingClientRect();
      const x = event.clientX - rect.left;
      const y = event.clientY - rect.top;

      let coords: [number, number] | null | undefined;
      try {
        coords = aladin.pix2world(x, y, 'icrs');
      } catch {
        return;
      }
      if (!coords) {
        return;
      }
      const [ra, dec] = coords;
      this.selectedCoord.set({ ra, dec });
    };

    element.addEventListener('click', onClick);
    this.destroyRef.onDestroy(() =>
      element.removeEventListener('click', onClick),
    );
  }
}
