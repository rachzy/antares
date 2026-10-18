import { Component, computed, inject } from '@angular/core';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmTabsImports } from '@spartan-ng/helm/tabs';
import { RunStore } from '../../../stores/run.store';
import { rank } from '../../../utils/scoring';
import { JobLog } from '../../ui/job-log/job-log';
import { ScoreDetails } from '../../ui/score-details';
import { ScoreMatrix } from '../../ui/score-matrix';
import { ScoreRanking } from '../../ui/score-ranking';

@Component({
  imports: [
    HlmButtonImports,
    HlmTabsImports,
    JobLog,
    ScoreDetails,
    ScoreMatrix,
    ScoreRanking,
  ],
  selector: 'an-score',
  templateUrl: './score.html',
  host: { class: 'flex min-h-0 flex-1 flex-col gap-3' },
})
export class Score {
  protected readonly run = inject(RunStore);
  protected readonly predictions = computed(
    () => this.run.score().result?.predictions ?? [],
  );
  protected readonly rows = computed(() => rank(this.predictions()));
  protected readonly done = computed(() => {
    const n = this.predictions().length;
    return `${n} candidate${n === 1 ? '' : 's'} scored`;
  });
}
