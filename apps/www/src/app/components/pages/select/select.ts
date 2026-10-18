import {
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { Router } from '@angular/router';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { RunStore } from '../../../stores/run.store';
import { TargetStore } from '../../../stores/target.store';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';

// The API matches within 4 arcsec; 0.05 deg (180 arcsec) across the view keeps the ring a few arcsec wide.
export const MAX_CONFIRM_FOV_DEG = 0.05;

@Component({
  selector: 'an-select',
  imports: [SkyViewer, HlmButtonImports],
  templateUrl: './select.html',
  host: { class: 'flex min-h-0 flex-1 gap-3' },
})
export class Select {
  private readonly targets = inject(TargetStore);
  private readonly run = inject(RunStore);
  private readonly router = inject(Router);
  private readonly sky = viewChild(SkyViewer);
  private readonly awaiting = signal(false);

  protected readonly star = this.run.star;
  protected readonly tooWide = computed(
    () => (this.sky()?.fovDeg() ?? Infinity) > MAX_CONFIRM_FOV_DEG,
  );
  protected readonly canConfirm = computed(
    () =>
      !!this.sky() &&
      !this.tooWide() &&
      !this.targets.loading() &&
      !this.sky()?.loadError(),
  );
  protected readonly catalogUrl = computed(
    () =>
      `https://www.stellarcatalog.com/stars/${encodeURIComponent(this.star()?.display_name ?? '')}`,
  );

  constructor() {
    effect(() => {
      const target = this.targets.target();
      const error = this.targets.error();
      if (!untracked(this.awaiting)) return;
      if (target) {
        this.awaiting.set(false);
        untracked(() => this.run.confirm(target));
      } else if (error) {
        this.awaiting.set(false);
      }
    });
  }

  protected confirm(): void {
    const center = this.sky()?.center();
    if (!center) return;
    this.awaiting.set(true);
    this.targets.resolveByPosition(center[0], center[1]);
  }

  protected extract(): void {
    this.run.startExtract();
    void this.router.navigate(['/download']);
  }
}
