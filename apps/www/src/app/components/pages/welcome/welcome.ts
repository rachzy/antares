import { Component, inject } from '@angular/core';
import { Router } from '@angular/router';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';

@Component({
  selector: 'an-welcome',
  imports: [SkyViewer],
  templateUrl: './welcome.html',
  styleUrl: './welcome.css',
})
export class Welcome {
  private readonly router = inject(Router);

  protected expand(): void {
    void this.router.navigate(['/select']);
  }
}
