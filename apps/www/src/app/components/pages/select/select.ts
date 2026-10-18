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
import { toast } from '@spartan-ng/brain/sonner';
import { HlmAlertDialogImports } from '@spartan-ng/helm/alert-dialog';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import type { Target } from '../../../api/api.types';
import { RunStore } from '../../../stores/run.store';
import { TargetStore } from '../../../stores/target.store';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';

// The API matches within 4 arcsec; 0.05 deg (180 arcsec) across the view keeps the ring a few arcsec wide.
export const MAX_CONFIRM_FOV_DEG = 0.05;

@Component({
  selector: 'an-select',
  imports: [SkyViewer, HlmButtonImports, HlmAlertDialogImports],
  templateUrl: './select.html',
  host: { class: 'flex min-h-0 flex-1 flex-col gap-3 md:flex-row' },
})
export class Select {
  private readonly targets = inject(TargetStore);
  private readonly run = inject(RunStore);
  private readonly router = inject(Router);
  private readonly sky = viewChild(SkyViewer);
  private readonly awaiting = signal(false);
  protected readonly pending = signal<Target | null>(null);

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
        untracked(() => {
          const busy = [this.run.extract(), this.run.score()].some(
            (s) => s.status === 'running',
          );
          if (busy) this.pending.set(target);
          else this.run.confirm(target);
        });
      } else if (error) {
        this.awaiting.set(false);
      }
    });
    let toasted = false;
    effect(() => {
      if (toasted || !this.sky()?.loadError()) return;
      toasted = true;
      toast.error('Unable to load the sky viewer.');
    });
  }

  protected replace(): void {
    const target = this.pending();
    this.pending.set(null);
    if (target) this.run.confirm(target);
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
