import { Component, computed, effect, inject, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideChevronRight } from '@ng-icons/lucide';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmTabsImports } from '@spartan-ng/helm/tabs';
import { RunStore } from '../../../stores/run.store';
import { StarInfoStore } from '../../../stores/star-info.store';
import { rank } from '../../../utils/scoring';
import { JobLog } from '../../ui/job-log/job-log';
import { ScoreDetails } from '../../ui/score-details';
import { ScoreMatrix } from '../../ui/score-matrix';
import { ScoreRanking } from '../../ui/score-ranking';
import { SystemPreview, type PreviewMode } from '../../ui/system-preview';
import type { ScoredRow } from '../../../utils/scoring';

@Component({
  imports: [
    HlmButtonImports,
    NgIcon,
    HlmTabsImports,
    JobLog,
    ScoreDetails,
    ScoreMatrix,
    ScoreRanking,
    SystemPreview,
  ],
  providers: [provideIcons({ lucideChevronRight })],
  selector: 'an-score',
  templateUrl: './score.html',
  host: { class: 'flex min-h-0 flex-1 flex-col gap-3' },
})
export class Score {
  protected readonly run = inject(RunStore);
  protected readonly starInfo = inject(StarInfoStore);
  protected readonly previewMode = signal<PreviewMode>('expanded');
  protected readonly selected = signal<ScoredRow | null>(null);
  protected readonly predictions = computed(
    () => this.run.score().result?.predictions ?? [],
  );
  protected readonly rows = computed(() => rank(this.predictions()));
  protected readonly done = computed(() => {
    const n = this.predictions().length;
    return `${n} candidate${n === 1 ? '' : 's'} scored`;
  });

  constructor() {
    effect(() => {
      const target = this.run.score().result?.target;
      if (target) this.starInfo.load(target);
    });
  }
}
