import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, effect, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  lucideActivity,
  lucideChartColumn,
  lucideCheck,
  lucideChevronDown,
  lucideChevronRight,
  lucideCode,
  lucideDatabase,
  lucideDownload,
  lucideLayers,
  lucideLock,
  lucideLockOpen,
  lucideMenu,
  lucideSlidersHorizontal,
  lucideStar,
  lucideTarget,
  lucideTelescope,
  lucideX,
} from '@ng-icons/lucide';
import { HlmTooltipImports } from '@spartan-ng/helm/tooltip';
import { filter, map } from 'rxjs';
import { PIPELINE_STEPS } from '../pages/pipeline/step.guard';
import { RunStore } from '../../stores/run.store';
import { ResetButton } from './reset-button';
import { TokenBadge } from './token-badge';

const STEPS = {
  select: { label: 'Select', icon: 'lucideTarget' },
  download: { label: 'Download', icon: 'lucideDownload' },
  process: { label: 'Process', icon: 'lucideActivity' },
  score: { label: 'Score', icon: 'lucideChartColumn' },
};

const MODEL_PAGES = [
  {
    path: '/model/hyperparameters',
    label: 'Hyperparameters',
    icon: 'lucideSlidersHorizontal',
  },
  { path: '/model/dataset', label: 'Dataset', icon: 'lucideDatabase' },
];

type Section = 'sky' | 'model' | 'api';

@Component({
  selector: 'an-sidebar',
  imports: [
    NgTemplateOutlet,
    NgIcon,
    RouterLink,
    HlmTooltipImports,
    ResetButton,
    TokenBadge,
  ],
  providers: [
    provideIcons({
      lucideActivity,
      lucideChartColumn,
      lucideCheck,
      lucideChevronDown,
      lucideChevronRight,
      lucideCode,
      lucideDatabase,
      lucideDownload,
      lucideLayers,
      lucideLock,
      lucideLockOpen,
      lucideMenu,
      lucideSlidersHorizontal,
      lucideStar,
      lucideTarget,
      lucideTelescope,
      lucideX,
    }),
  ],
  templateUrl: './sidebar.html',
  host: { class: 'contents' },
})
export class Sidebar {
  private readonly router = inject(Router);
  private readonly run = inject(RunStore);
  protected readonly url = toSignal(
    this.router.events.pipe(
      filter((e) => e instanceof NavigationEnd),
      map((e) => e.urlAfterRedirects),
    ),
    { initialValue: this.router.url },
  );
  protected readonly open = signal(false);
  protected readonly modelPages = MODEL_PAGES;

  protected readonly section = computed<Section>(() => {
    const path = this.url().split(/[?#]/)[0];
    if (path === '/model' || path.startsWith('/model/')) return 'model';
    if (path === '/api' || path.startsWith('/api/')) return 'api';
    return 'sky';
  });

  protected readonly steps = computed(() => {
    const reached = PIPELINE_STEPS.indexOf(this.run.unlocked());
    const current = this.url().split(/[?#]/)[0].split('/')[1];
    return PIPELINE_STEPS.map((id, i) => ({
      id,
      ...STEPS[id],
      state:
        id === current
          ? ('current' as const)
          : i > reached
            ? ('locked' as const)
            : i < reached
              ? ('done' as const)
              : ('open' as const),
    }));
  });

  constructor() {
    effect(() => {
      this.url();
      this.open.set(false);
    });
  }

  protected isModelPage(path: string): boolean {
    return this.url().split(/[?#]/)[0] === path;
  }
}
