import {
  afterNextRender,
  Component,
  computed,
  DestroyRef,
  ElementRef,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import type { AladinInstance } from 'aladin-lite';

const INITIAL_FOV_DEG = 60;
// A direct HiPS URL skips Aladin's registry lookup, which also probes mirrors that send no CORS headers.
export const SURVEY_URL = 'https://alasky.cds.unistra.fr/DSS/DSSColor';

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
  readonly ready = computed(() => !this.loading() && !this.loadError());
  private readonly fov = signal(INITIAL_FOV_DEG);
  readonly fovDeg = this.fov.asReadonly();

  private aladin: AladinInstance | null = null;
  private destroyed = false;
  private element: HTMLDivElement | null = null;
  private detachClick: (() => void) | null = null;

  constructor() {
    afterNextRender(() => {
      this.element = this.skyDiv().nativeElement;
      void this.initAladin(this.element);
    });
    inject(DestroyRef).onDestroy(() => {
      this.destroyed = true;
      this.detachClick?.();
      this.aladin = null;
      this.element?.replaceChildren();
    });
  }

  center(): [number, number] | null {
    return this.aladin?.getRaDec() ?? null;
  }

  goto(query: string): Promise<boolean> {
    const aladin = this.aladin;
    if (!aladin) return Promise.resolve(false);
    return new Promise((resolve) =>
      aladin.gotoObject(query, {
        success: () => resolve(true),
        error: () => resolve(false),
      }),
    );
  }

  private async initAladin(element: HTMLDivElement): Promise<void> {
    try {
      const { default: A } = await import('aladin-lite');
      if (this.destroyed) return;
      await A.init;
      if (this.destroyed) return;
      const aladin = A.aladin(element, {
        survey: SURVEY_URL,
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
      this.attachClickHandler(aladin, element);
    } catch (error) {
      if (this.destroyed) return;
      console.error('Failed to initialize the sky viewer', error);
      this.loadError.set(true);
    }
  }

  private attachClickHandler(
    aladin: AladinInstance,
    element: HTMLDivElement,
  ): void {
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
    this.detachClick = () => element.removeEventListener('click', onClick);
  }
}
