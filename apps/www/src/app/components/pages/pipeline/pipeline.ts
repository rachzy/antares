import { Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import {
  NavigationEnd,
  Router,
  RouterLink,
  RouterOutlet,
} from '@angular/router';
import { HlmTooltipImports } from '@spartan-ng/helm/tooltip';
import { filter, map } from 'rxjs';
import { RunStore } from '../../../stores/run.store';
import { PIPELINE_STEPS } from './step.guard';

const LABELS = {
  select: 'Select star',
  download: 'Download light curves',
  process: 'Process data',
  score: 'Score candidates',
};

@Component({
  imports: [RouterLink, RouterOutlet, HlmTooltipImports],
  selector: 'an-pipeline',
  templateUrl: './pipeline.html',
})
export class Pipeline {
  private readonly router = inject(Router);
  private readonly run = inject(RunStore);
  private readonly url = toSignal(
    this.router.events.pipe(
      filter((e) => e instanceof NavigationEnd),
      map((e) => e.urlAfterRedirects),
    ),
    { initialValue: this.router.url },
  );

  protected readonly steps = computed(() => {
    const reached = PIPELINE_STEPS.indexOf(this.run.unlocked());
    const path = this.url().split(/[?#]/)[0].split('/').pop();
    return PIPELINE_STEPS.map((id, i) => ({
      id,
      label: LABELS[id],
      number: i + 1,
      state:
        id === path
          ? ('current' as const)
          : i > reached
            ? ('locked' as const)
            : i < reached
              ? ('done' as const)
              : ('open' as const),
    }));
  });
}
