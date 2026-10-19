import {
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideLoader2, lucideSearch } from '@ng-icons/lucide';
import { toast } from '@spartan-ng/brain/sonner';
import { HlmAlertDialogImports } from '@spartan-ng/helm/alert-dialog';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmCheckboxImports } from '@spartan-ng/helm/checkbox';
import { HlmInputImports } from '@spartan-ng/helm/input';
import type { Target } from '../../../api/api.types';
import { RunStore } from '../../../stores/run.store';
import { TargetStore } from '../../../stores/target.store';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';
import { simbadUrl } from './simbad-url';

// The API matches within 4 arcsec; 0.05 deg (180 arcsec) across the view keeps the ring a few arcsec wide.
export const MAX_CONFIRM_FOV_DEG = 0.05;

@Component({
  selector: 'an-select',
  imports: [
    SkyViewer,
    FormsModule,
    NgIcon,
    HlmButtonImports,
    HlmCheckboxImports,
    HlmInputImports,
    HlmAlertDialogImports,
  ],
  providers: [provideIcons({ lucideLoader2, lucideSearch })],
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
  protected readonly query = signal('');
  protected readonly searching = signal(false);
  protected readonly downloadAll = signal(false);
  protected readonly resolving = this.targets.loading;

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
  protected readonly catalogUrl = computed(() => {
    const star = this.star();
    return star ? simbadUrl(star) : '';
  });

  constructor() {
    effect(() => {
      const target = this.targets.target();
      const error = this.targets.error();
      if (!untracked(this.awaiting)) return;
      if (target) {
        this.awaiting.set(false);
        untracked(() => {
          toast.success(`Found ${target.display_name}`);
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

  protected async search(): Promise<void> {
    const query = this.query().trim();
    if (!query || this.searching()) return;
    this.searching.set(true);
    const found = await this.sky()?.goto(query);
    this.searching.set(false);
    if (!found) toast.error(`No object named "${query}" found`);
  }

  protected extract(): void {
    this.run.startExtract(this.downloadAll());
    void this.router.navigate(['/download']);
  }
}
