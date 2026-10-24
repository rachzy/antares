import {
  ChangeDetectionStrategy,
  Component,
  effect,
  inject,
} from '@angular/core';
import { toast } from '@spartan-ng/brain/sonner';
import { HlmToaster } from '@spartan-ng/helm/sonner';
import { ApiRequestError } from '../../api/api.service';
import { HealthStore } from '../../stores/health.store';
import { MlStore } from '../../stores/ml.store';
import { ModelStore } from '../../stores/model.store';
import { PipelineStore } from '../../stores/pipeline.store';
import { RunStore } from '../../stores/run.store';
import { TargetStore } from '../../stores/target.store';

@Component({
  selector: 'an-error-toaster',
  imports: [HlmToaster],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: '<hlm-toaster />',
})
export class ErrorToaster {
  private readonly target = inject(TargetStore);
  private readonly pipeline = inject(PipelineStore);
  private readonly health = inject(HealthStore);
  private readonly ml = inject(MlStore);
  private readonly model = inject(ModelStore);
  private readonly run = inject(RunStore);
  private readonly shown = new WeakSet<ApiRequestError>();

  constructor() {
    effect(() => {
      const retryAfter = this.pipeline.retryAfterSeconds();
      const errors = [
        this.target.error(),
        this.pipeline.error(),
        this.health.error(),
        this.ml.error(),
        this.model.error(),
        this.run.extract().error,
        this.run.score().error,
      ];
      for (const error of errors) {
        if (!error || this.shown.has(error)) continue;
        this.shown.add(error);
        const seconds = error.retryAfter ?? retryAfter;
        const hint =
          error.code === 'busy' && seconds ? ` Try again in ${seconds} s.` : '';
        const notify = error.code === 'busy' ? toast.warning : toast.error;
        notify(error.message + hint);
      }
    });
  }
}
