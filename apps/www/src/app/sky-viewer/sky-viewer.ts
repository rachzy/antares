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

@Component({
  selector: 'an-sky-viewer',
  templateUrl: './sky-viewer.html',
  styleUrl: './sky-viewer.css',
})
export class SkyViewer {
  protected readonly skyDiv = viewChild.required<ElementRef<HTMLDivElement>>('skyDiv');
  protected readonly selectedCoord = signal<{ ra: number; dec: number } | null>(null);
  protected readonly loadError = signal(false);

  private readonly destroyRef = inject(DestroyRef);

  constructor() {
    afterNextRender(() => void this.initAladin());
  }

  private async initAladin(): Promise<void> {
    try {
      const { default: A } = await import('aladin-lite');
      await A.init;
      const aladin = A.aladin(this.skyDiv().nativeElement, {
        survey: 'P/DSS2/color',
        fov: 60,
        cooFrame: 'equatorial',
      });
      this.attachClickHandler(aladin);
    } catch (error) {
      console.error('Failed to initialize the sky viewer', error);
      this.loadError.set(true);
    }
  }

  private attachClickHandler(aladin: AladinInstance): void {
    const element = this.skyDiv().nativeElement;
    const onClick = (event: MouseEvent) => {
      const coords = aladin.pix2world(event.offsetX, event.offsetY);
      if (!coords) {
        return;
      }
      const [ra, dec] = coords;
      this.selectedCoord.set({ ra, dec });
    };

    element.addEventListener('click', onClick);
    this.destroyRef.onDestroy(() => element.removeEventListener('click', onClick));
  }
}
