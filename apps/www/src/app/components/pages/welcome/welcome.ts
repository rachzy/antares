import { Component, computed, inject, signal } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideArrowRight, lucideSearch } from '@ng-icons/lucide';
import { HlmAutocompleteImports } from '@spartan-ng/helm/autocomplete';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';
import { suggestKeplerStars } from '../select/kepler-suggestions';

@Component({
  selector: 'an-welcome',
  imports: [
    RouterLink,
    NgIcon,
    SkyViewer,
    HlmAutocompleteImports,
    HlmButtonImports,
  ],
  providers: [provideIcons({ lucideArrowRight, lucideSearch })],
  templateUrl: './welcome.html',
  host: { class: 'relative block h-full min-h-[36rem] overflow-hidden' },
})
export class Welcome {
  private readonly router = inject(Router);
  private closedAt = 0;
  protected readonly query = signal('');
  protected readonly suggestionsOpen = signal(false);
  protected readonly suggestions = computed(() =>
    suggestKeplerStars(this.query()),
  );
  protected readonly examples = ['Kepler-90', 'Kepler-186', 'Kepler-22'];

  protected setSuggestionsState(state: string): void {
    if (state !== 'open') this.closedAt = Date.now();
    this.suggestionsOpen.set(state === 'open');
  }

  protected openSuggestions(event: Event): void {
    if (!(event.target instanceof HTMLInputElement)) return;
    // Closing the popup hands focus back to the input, which must not reopen it.
    if (event.type === 'focusin' && Date.now() - this.closedAt < 300) return;
    this.suggestionsOpen.set(true);
  }

  protected pick(name: string | null): void {
    if (name) void this.go(name);
  }

  protected submit(): void {
    if (this.suggestions().length === 0) void this.go(this.query());
  }

  private go(query: string): Promise<boolean> {
    const q = query.trim();
    return this.router.navigate(['/select'], q ? { queryParams: { q } } : {});
  }
}
