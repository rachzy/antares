import { Component, computed, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { RunStore } from '../../../stores/run.store';
import { JobLog, splitByStage } from '../../ui/job-log/job-log';

@Component({
  imports: [RouterLink, HlmButtonImports, JobLog],
  selector: 'an-download',
  templateUrl: './download.html',
  host: { class: 'flex min-h-0 flex-1 flex-col gap-4' },
})
export class Download {
  protected readonly run = inject(RunStore);
  protected readonly events = computed(
    () => splitByStage(this.run.extract().events).download,
  );
  protected readonly status = computed(() =>
    this.run.downloadDone() ? 'done' : this.run.extract().status,
  );
}
