import { Component, inject } from '@angular/core';
import { ModelStore } from '../../stores/model.store';

@Component({
  selector: 'an-run-picker',
  template: `
    <div class="flex flex-wrap items-center gap-2.5">
      <label for="run-picker" class="text-muted-foreground text-sm">Run</label>
      <select
        id="run-picker"
        class="border-input bg-card focus-visible:ring-ring/50 h-10 max-w-full rounded-md border px-3 font-mono text-sm outline-none focus-visible:ring-[3px]"
        [value]="model.runId() ?? ''"
        (change)="pick($event)"
      >
        @for (run of model.runs(); track run.run_id; let first = $first) {
          <option
            [value]="run.run_id"
            [selected]="run.run_id === model.runId()"
          >
            {{ run.run_id }}{{ first ? ' (newest)' : '' }}
          </option>
        }
      </select>
    </div>
  `,
})
export class RunPicker {
  protected readonly model = inject(ModelStore);

  protected pick(event: Event): void {
    this.model.selectRun((event.target as HTMLSelectElement).value);
  }
}
