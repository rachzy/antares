import { Component, inject } from '@angular/core';
import { Router } from '@angular/router';
import { BrandState } from '../../ui/brand';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';

// Matches the brand's move-up transition so the title lands before the page changes.
const LEAVE_MS = 600;

@Component({
  selector: 'an-welcome',
  imports: [SkyViewer],
  templateUrl: './welcome.html',
  styleUrl: './welcome.css',
})
export class Welcome {
  private readonly router = inject(Router);
  protected readonly brand = inject(BrandState);

  protected expand(): void {
    if (this.brand.leaving()) return;
    this.brand.leaving.set(true);
    const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
    setTimeout(
      () =>
        void this.router
          .navigate(['/select'])
          .finally(() => this.brand.leaving.set(false)),
      reduced ? 0 : LEAVE_MS,
    );
  }
}
