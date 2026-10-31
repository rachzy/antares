import {
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideLoader2, lucideSearch } from '@ng-icons/lucide';
import { toast } from '@spartan-ng/brain/sonner';
import { HlmAlertDialogImports } from '@spartan-ng/helm/alert-dialog';
import { HlmAutocompleteImports } from '@spartan-ng/helm/autocomplete';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmCheckboxImports } from '@spartan-ng/helm/checkbox';
import type { Target } from '../../../api/api.types';
import { RunStore } from '../../../stores/run.store';
import { StarInfoStore } from '../../../stores/star-info.store';
import { TargetStore } from '../../../stores/target.store';
import { ManualSelect } from '../../containers/manual-select/manual-select';
import { SkyViewer } from '../../containers/sky-viewer/sky-viewer';
import { StarDetailsView } from '../../containers/star-details/star-details';
import { type SelectMode, ModeSwitch } from '../../ui/mode-switch';
import { isKeplerStar, suggestKeplerStars } from './kepler-suggestions';
import { simbadUrl } from './simbad-url';

// The API matches within 4 arcsec; 0.05 deg (180 arcsec) across the view keeps the ring a few arcsec wide.
export const MAX_CONFIRM_FOV_DEG = 0.05;
export const KEPLER_ZOOM_FOV_DEG = 0.02;

@Component({
  selector: 'an-select',
  imports: [
    SkyViewer,
    StarDetailsView,
    ManualSelect,
    ModeSwitch,
    NgIcon,
    HlmButtonImports,
    HlmCheckboxImports,
    HlmAutocompleteImports,
    HlmAlertDialogImports,
  ],
  providers: [provideIcons({ lucideLoader2, lucideSearch })],
  templateUrl: './select.html',
  host: { class: 'flex min-h-0 flex-1 flex-col gap-3 md:flex-row' },
})
export class Select {
  private readonly targets = inject(TargetStore);
  private readonly run = inject(RunStore);
  protected readonly starInfo = inject(StarInfoStore);
  private readonly router = inject(Router);
  private readonly initialQuery = signal(
    inject(ActivatedRoute).snapshot.queryParamMap.get('q'),
  );
  private readonly sky = viewChild(SkyViewer);
  private readonly awaiting = signal(false);
  private readonly extracting = signal(false);
  protected readonly pending = signal<Target | null>(null);
  private closedAt = 0;
  protected readonly mode = signal<SelectMode>('aladin');
  protected readonly query = signal('');
  protected readonly searching = signal(false);
  protected readonly suggestionsOpen = signal(false);
  protected readonly suggestions = computed(() =>
    suggestKeplerStars(this.query()),
  );
  protected readonly downloadAll = signal(false);
  protected readonly locating = this.awaiting;
  protected readonly resolving = this.extracting;
  protected readonly extractError = signal<string | null>(null);

  protected readonly star = this.run.star;
  protected readonly tooWide = computed(
    () => (this.sky()?.fovDeg() ?? Infinity) > MAX_CONFIRM_FOV_DEG,
  );
  protected readonly canConfirm = computed(
    () =>
      !!this.sky() &&
      !this.tooWide() &&
      !this.awaiting() &&
      !this.sky()?.loadError(),
  );
  protected readonly designation = computed(() => {
    const star = this.star();
    return star ? star.designation || `${star.catalog} ${star.catalog_id}` : '';
  });
  protected readonly catalogUrl = computed(() => {
    const star = this.star();
    return star ? simbadUrl(star) : '';
  });

  constructor() {
    effect(() => {
      const star = this.star();
      untracked(() => {
        this.extractError.set(null);
        if (star) this.starInfo.load(star);
        else this.starInfo.reset();
      });
    });
    effect(() => {
      const target = this.starInfo.target();
      const error = this.starInfo.error();
      if (!untracked(this.awaiting)) return;
      if (target) {
        this.awaiting.set(false);
        untracked(() => {
          toast.success(`Found ${target.display_name}`);
          const busy = [this.run.extract(), this.run.score()].some(
            (s) => s.status === 'running',
          );
          if (busy) this.pending.set(target);
          else this.run.confirm(target);
        });
      } else if (error) {
        this.awaiting.set(false);
        untracked(() => toast.error(error.message));
      }
    });
    effect(() => {
      const target = this.targets.target();
      const error = this.targets.error();
      if (!untracked(this.extracting)) return;
      if (target) {
        this.extracting.set(false);
        untracked(() => {
          this.run.startExtract(this.downloadAll(), target);
          void this.router.navigate(['/download']);
        });
      } else if (error) {
        this.extracting.set(false);
        this.extractError.set(error.message);
      }
    });
    effect(() => {
      const q = this.initialQuery();
      if (!q || !this.sky()?.ready()) return;
      untracked(() => {
        this.initialQuery.set(null);
        this.query.set(q);
        void this.search();
      });
    });
    let toasted = false;
    effect(() => {
      if (toasted || !this.sky()?.loadError()) return;
      toasted = true;
      toast.error('Unable to load the sky viewer.');
    });
  }

  protected keep(): void {
    this.pending.set(null);
    const star = this.star();
    if (star) this.starInfo.load(star);
  }

  protected replace(): void {
    const target = this.pending();
    this.pending.set(null);
    if (target) this.run.confirm(target);
  }

  protected confirm(): void {
    const center = this.sky()?.center();
    if (!center) return;
    this.awaiting.set(true);
    this.starInfo.locate(center[0], center[1]);
  }

  // Back to Aladin with the query filled in, so the lookup reads as the same search.
  protected lookup(query: string): void {
    this.mode.set('aladin');
    this.query.set(query);
    void this.search();
  }

  protected setSuggestionsState(state: string): void {
    if (state !== 'open') this.closedAt = Date.now();
    this.suggestionsOpen.set(state === 'open');
  }

  protected openSuggestions(event: Event): void {
    if (!(event.target instanceof HTMLInputElement)) return;
    // Closing the popup (pick, Escape, outside click) hands focus back to the input, which must not reopen it.
    if (event.type === 'focusin' && Date.now() - this.closedAt < 300) return;
    this.suggestionsOpen.set(true);
  }

  protected pick(name: string | null): void {
    if (!name) return;
    this.query.set(name);
    void this.search();
  }

  // Picking a highlighted suggestion already searches (and sets the guard), so this only covers typed text.
  protected searchTyped(): void {
    void this.search();
  }

  protected async search(): Promise<void> {
    const query = this.query().trim();
    if (!query || this.searching()) return;
    this.searching.set(true);
    const busy = [this.run.extract(), this.run.score()].some(
      (s) => s.status === 'running',
    );
    // A running job is kept: replacing it is the Replace dialog's call.
    if (!busy) this.run.reset();
    const found = await this.sky()?.goto(
      query,
      isKeplerStar(query) ? KEPLER_ZOOM_FOV_DEG : undefined,
    );
    this.searching.set(false);
    if (!found) toast.error(`No object named "${query}" found`);
  }

  // Shaula is only asked now, so a star without light curve data fails here rather than on Confirm.
  protected extract(): void {
    const star = this.star();
    if (!star || this.extracting()) return;
    this.extractError.set(null);
    this.extracting.set(true);
    this.targets.resolveByPosition(star.ra_deg, star.dec_deg);
  }
}
