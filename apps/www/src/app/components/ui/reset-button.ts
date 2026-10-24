import { Component, inject } from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideRotateCcw } from '@ng-icons/lucide';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { RunStore } from '../../stores/run.store';
import { TargetStore } from '../../stores/target.store';

@Component({
  selector: 'an-reset-button',
  imports: [NgIcon, HlmButtonImports],
  providers: [provideIcons({ lucideRotateCcw })],
  template: `
    @if (run.star()) {
      <button
        hlmBtn
        variant="outline"
        size="icon"
        type="button"
        aria-label="Reset"
        title="Reset"
        (click)="reset()"
      >
        <ng-icon name="lucideRotateCcw" />
      </button>
    }
  `,
})
export class ResetButton {
  protected readonly run = inject(RunStore);
  private readonly targets = inject(TargetStore);
  private readonly router = inject(Router);

  protected reset(): void {
    this.run.reset();
    this.targets.reset();
    void this.router.navigateByUrl('/select');
  }
}
