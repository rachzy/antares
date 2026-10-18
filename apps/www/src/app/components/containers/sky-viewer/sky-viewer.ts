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
import { HlmButtonImports } from '@spartan-ng/helm/button';
import type { AladinInstance } from 'aladin-lite';
import type { Mission } from '../../../api/api.types';
import { HealthStore } from '../../../stores/health.store';
import { PipelineStore } from '../../../stores/pipeline.store';
import { TargetStore } from '../../../stores/target.store';

const MISSIONS: Record<string, Mission> = {
  KIC: 'Kepler',
  EPIC: 'K2',
  TIC: 'TESS',
};

const PREDICTION_FIELDS = /^(prob_|period_days$)/;

@Component({
  selector: 'an-sky-viewer',
  imports: [HlmButtonImports],
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
  protected readonly loadError = signal(false);

  protected readonly health = inject(HealthStore);
  protected readonly target = inject(TargetStore);
  protected readonly pipeline = inject(PipelineStore);

  protected readonly notReady = computed(
    () => this.health.health() !== null && !this.health.ready(),
  );
  protected readonly percent = computed(() => {
    const fraction = this.pipeline.progress()?.fraction;
    return fraction === undefined
      ? null
      : Math.min(100, Math.max(0, Math.round(fraction * 100)));
  });
  protected readonly topPrediction = computed(() => {
    const row = this.pipeline.result()?.predictions?.[0];
    if (!row) return null;
    const entries = Object.entries(row);
    let fields = entries.filter(([key]) => PREDICTION_FIELDS.test(key));
    if (!fields.length) {
      fields = entries.filter(
        ([key, value]) =>
          key !== 'star_id' &&
          ['string', 'number', 'boolean'].includes(typeof value),
      );
    }
    if (!fields.length) return null;
    return fields.map(([key, value]) => ({
      key,
      value:
        typeof value === 'number' && key.startsWith('prob_')
          ? value.toFixed(3)
          : String(value),
    }));
  });
  protected readonly panelVisible = computed(
    () =>
      this.notReady() ||
      this.selectedCoord() !== null ||
      this.target.target() !== null ||
      this.target.loading() ||
      this.target.error() !== null ||
      this.pipeline.status() !== 'idle' ||
      this.pipeline.loading() ||
      this.pipeline.result() !== null ||
      this.pipeline.error() !== null,
  );

  private readonly destroyRef = inject(DestroyRef);

  constructor() {
    afterNextRender(() => void this.initAladin());
  }

  protected select(ra: number, dec: number): void {
    this.selectedCoord.set({ ra, dec });
    this.target.resolveByPosition(ra, dec);
  }

  protected analyze(): void {
    const target = this.target.target();
    if (!target) return;
    this.pipeline.analyze({
      ra_deg: target.ra_deg,
      dec_deg: target.dec_deg,
      mission: MISSIONS[target.catalog],
    });
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
      this.select(ra, dec);
    };

    element.addEventListener('click', onClick);
    this.destroyRef.onDestroy(() =>
      element.removeEventListener('click', onClick),
    );
  }
}
