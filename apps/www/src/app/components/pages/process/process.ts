import { Component, computed, inject } from '@angular/core';
import { Router } from '@angular/router';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmTooltipImports } from '@spartan-ng/helm/tooltip';
import { RunStore } from '../../../stores/run.store';
import { featuresToCsv } from '../../../utils/features-csv';
import { JobLog, splitByStage } from '../../ui/job-log/job-log';

export const SCORE_TOOLTIP =
  'Runs the extracted data through our machine learning model and shows how likely each candidate is to be a real exoplanet';

@Component({
  imports: [HlmButtonImports, HlmTooltipImports, JobLog],
  selector: 'an-process',
  templateUrl: './process.html',
  host: { class: 'flex min-h-0 flex-1 flex-col gap-4' },
})
export class Process {
  protected readonly run = inject(RunStore);
  private readonly router = inject(Router);
  protected readonly tooltip = SCORE_TOOLTIP;
  protected readonly events = computed(
    () => splitByStage(this.run.extract().events).process,
  );
  protected readonly done = computed(() => {
    const n = this.run.candidates();
    return `${n} candidate${n === 1 ? '' : 's'} extracted`;
  });

  protected score() {
    this.run.startScore();
    void this.router.navigateByUrl('/score');
  }

  protected download() {
    const csv = featuresToCsv(this.run.extract().result?.features ?? []);
    const url = URL.createObjectURL(
      new Blob([csv], { type: 'text/csv;charset=utf-8' }),
    );
    const a = document.createElement('a');
    a.href = url;
    a.download = 'features.csv';
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url));
  }
}
