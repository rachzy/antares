import {
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideArrowRight, lucideSearch } from '@ng-icons/lucide';
import { toast } from '@spartan-ng/brain/sonner';
import { HlmAutocompleteImports } from '@spartan-ng/helm/autocomplete';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { RunStore } from '../../../stores/run.store';
import { StarInfoStore } from '../../../stores/star-info.store';
import type { ManualLookup } from '../../containers/manual-select/manual-query';
import { ManualSelect } from '../../containers/manual-select/manual-select';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';
import { type SelectMode, ModeSwitch } from '../../ui/mode-switch';
import { suggestKeplerStars } from '../select/kepler-suggestions';

@Component({
  selector: 'an-welcome',
  imports: [
    RouterLink,
    NgIcon,
    SkyViewer,
    HlmAutocompleteImports,
    HlmButtonImports,
    ManualSelect,
    ModeSwitch,
  ],
  providers: [provideIcons({ lucideArrowRight, lucideSearch })],
  templateUrl: './welcome.html',
  host: { class: 'relative block h-full min-h-[36rem] overflow-hidden' },
})
export class Welcome {
  private readonly router = inject(Router);
  private readonly starInfo = inject(StarInfoStore);
  private readonly run = inject(RunStore);
  private entry: ManualLookup | null = null;
  private closedAt = 0;
  protected readonly locating = signal(false);
  protected readonly mode = signal<SelectMode>('aladin');
  protected readonly query = signal('');
  protected readonly suggestionsOpen = signal(false);
  protected readonly suggestions = computed(() =>
    suggestKeplerStars(this.query()),
  );
  protected readonly examples = ['Kepler-90', 'Kepler-186', 'Kepler-22'];

  constructor() {
    effect(() => {
      const target = this.starInfo.target();
      const error = this.starInfo.error();
      if (!untracked(this.locating)) return;
      if (!target && !error) return;
      this.locating.set(false);
      untracked(() => {
        const entry = this.entry;
        this.entry = null;
        if (error) {
          toast.error(error.message);
          return;
        }
        // A running job is kept: the select page asks before replacing it.
        const busy = [this.run.extract(), this.run.score()].some(
          (s) => s.status === 'running',
        );
        if (busy && entry) {
          void this.router.navigate(['/select'], {
            queryParams: this.entryParams(entry),
          });
          return;
        }
        if (target) this.run.confirm(target);
        void this.router.navigate(['/select']);
      });
    });
  }

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

  // The select page opens only once the star is found, so it lands with the star card ready.
  protected lookup(entry: ManualLookup): void {
    if (this.locating()) return;
    this.entry = entry;
    this.locating.set(true);
    if ('query' in entry) this.starInfo.lookup(entry.query);
    else this.starInfo.locate(entry.ra, entry.dec);
  }

  private entryParams(entry: ManualLookup) {
    return 'query' in entry
      ? { star: entry.query }
      : { ra: entry.ra, dec: entry.dec };
  }

  protected submit(): void {
    if (this.suggestions().length === 0) void this.go(this.query());
  }

  private go(query: string): Promise<boolean> {
    const q = query.trim();
    return this.router.navigate(['/select'], q ? { queryParams: { q } } : {});
  }
}
