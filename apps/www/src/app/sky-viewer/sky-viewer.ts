import {
  afterNextRender,
  Component,
  ElementRef,
  signal,
  viewChild,
} from '@angular/core';

@Component({
  selector: 'an-sky-viewer',
  templateUrl: './sky-viewer.html',
  styleUrl: './sky-viewer.css',
})
export class SkyViewer {
  protected readonly skyDiv = viewChild.required<ElementRef<HTMLDivElement>>('skyDiv');
  protected readonly selectedCoord = signal<{ ra: number; dec: number } | null>(null);
  protected readonly loadError = signal(false);

  constructor() {
    afterNextRender(() => void this.initAladin());
  }

  private async initAladin(): Promise<void> {
    const { default: A } = await import('aladin-lite');
    await A.init;
    A.aladin(this.skyDiv().nativeElement, {
      survey: 'P/DSS2/color',
      fov: 60,
      cooFrame: 'equatorial',
    });
  }
}
