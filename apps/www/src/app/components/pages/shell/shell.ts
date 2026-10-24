import { Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { Sidebar } from '../../ui/sidebar';

@Component({
  selector: 'an-shell',
  imports: [RouterOutlet, Sidebar],
  template: `
    <div class="bg-background text-foreground flex h-dvh flex-col md:flex-row">
      <an-sidebar />
      <main class="relative min-h-0 min-w-0 flex-1 overflow-y-auto">
        <router-outlet />
      </main>
    </div>
  `,
})
export class Shell {}
