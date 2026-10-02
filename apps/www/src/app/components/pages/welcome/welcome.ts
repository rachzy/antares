import { Component, HostListener, signal } from '@angular/core';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';

@Component({
  selector: 'an-welcome',
  imports: [SkyViewer],
  templateUrl: './welcome.html',
  styleUrl: './welcome.css',
})
export class Welcome {
  protected readonly expanded = signal(false);

  protected expand(): void {
    this.expanded.set(true);
  }

  protected collapse(): void {
    this.expanded.set(false);
  }

  @HostListener('document:keydown.escape')
  protected onEscape(): void {
    this.collapse();
  }
}
