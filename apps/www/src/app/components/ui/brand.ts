import { Component, inject, Injectable, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink } from '@angular/router';
import { filter, map } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class BrandState {
  readonly leaving = signal(false);
}

@Component({
  selector: 'an-brand',
  imports: [RouterLink],
  styleUrl: './brand.css',
  template: `
    @if (atWelcome() !== null) {
      <a
        routerLink="/"
        class="brand"
        [attr.data-compact]="atWelcome() === false || state.leaving()"
        >Antares</a
      >
    }
  `,
})
export class Brand {
  protected readonly state = inject(BrandState);
  private readonly router = inject(Router);
  protected readonly atWelcome = toSignal<boolean | null, boolean | null>(
    this.router.events.pipe(
      filter((e) => e instanceof NavigationEnd),
      map((e) => e.urlAfterRedirects === '/'),
    ),
    {
      initialValue: this.router.navigated ? this.router.url === '/' : null,
    },
  );
}
