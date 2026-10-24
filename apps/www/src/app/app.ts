import { Component, inject, OnInit } from '@angular/core';
import { RouterModule } from '@angular/router';
import { ErrorToaster } from './components/ui/error-toaster';
import { HealthStore } from './stores/health.store';
import { RunStore } from './stores/run.store';

@Component({
  imports: [RouterModule, ErrorToaster],
  selector: 'an-root',
  templateUrl: './app.html',
})
export class App implements OnInit {
  private readonly health = inject(HealthStore);
  private readonly run = inject(RunStore);

  ngOnInit(): void {
    this.health.check();
    this.run.restore();
  }
}
