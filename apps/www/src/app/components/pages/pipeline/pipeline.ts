import { Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';

@Component({
  imports: [RouterOutlet],
  selector: 'an-pipeline',
  template: `
    <div class="flex h-full min-h-0 flex-col p-4">
      <router-outlet />
    </div>
  `,
})
export class Pipeline {}
