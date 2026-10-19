import {
  afterRenderEffect,
  Component,
  computed,
  ElementRef,
  inject,
  Injectable,
  signal,
  viewChild,
} from '@angular/core';
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
      <a routerLink="/" #link class="brand" [attr.data-compact]="compact()"
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
  protected readonly compact = computed(
    () => this.atWelcome() === false || this.state.leaving(),
  );
  private readonly link = viewChild<ElementRef<HTMLElement>>('link');
  private previous: boolean | null = null;

  constructor() {
    afterRenderEffect(() => {
      const compact = this.compact();
      const el = this.link()?.nativeElement;
      const was = this.previous;
      this.previous = el ? compact : null;
      if (!el || was === null || was === compact) return;
      this.flip(el, was);
    });
  }

  // FLIP: layout jumps straight to the new state and only transform animates, avoiding per-frame reflow.
  private flip(el: HTMLElement, wasCompact: boolean): void {
    if (
      typeof el.animate !== 'function' ||
      matchMedia('(prefers-reduced-motion: reduce)').matches
    )
      return;
    const after = el.getBoundingClientRect();
    el.dataset['compact'] = String(wasCompact);
    const before = el.getBoundingClientRect();
    el.dataset['compact'] = String(!wasCompact);
    if (!after.width || !before.width) return;
    el.animate(
      [
        {
          transform: `translate(${before.left - after.left}px, ${before.top - after.top}px) scale(${before.width / after.width})`,
        },
        { transform: 'none' },
      ],
      { duration: 700, easing: 'cubic-bezier(0.16, 1, 0.3, 1)' },
    );
  }
}
