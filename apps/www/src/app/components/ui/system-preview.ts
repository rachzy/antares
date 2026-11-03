import {
  Component,
  computed,
  effect,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { StarSystemComponent } from '@larawag/angular';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmCheckboxImports } from '@spartan-ng/helm/checkbox';
import type { StarDetails, Target } from '../../api/api.types';
import type { ScoredRow } from '../../utils/scoring';
import { hasStarData, toSystemPreview } from '../../utils/system-preview';

@Component({
  imports: [HlmButtonImports, HlmCheckboxImports, StarSystemComponent],
  selector: 'an-system-preview',
  host: { class: 'flex min-h-0 min-w-0 flex-col gap-2.5' },
  template: `
    <div class="flex items-baseline gap-2">
      <h3 class="text-sm font-medium">System preview</h3>
      <span class="text-muted-foreground text-xs">{{ subtitle() }}</span>
    </div>
    @if (!ready()) {
      <p data-empty class="text-muted-foreground text-sm">
        There is not enough data about this star to draw its system.
      </p>
    } @else if (preview().planets.length === 0) {
      <p data-empty class="text-muted-foreground text-sm">
        No candidates to show. Include the unlikely ones to see them.
      </p>
    }
    @if (ready() && preview().planets.length > 0) {
      <div class="h-[26rem] min-h-64 overflow-hidden rounded-lg bg-black">
        <larawag-star-system
          [system]="preview().system"
          [scaleMode]="trueScale() ? 'true' : 'compressed'"
          [showHabitableZone]="habitableZone()"
          [playing]="playing()"
          [followOnClick]="true"
          [overlays]="{ badge: { text: badge } }"
          (followChange)="followed($event)"
        />
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button
          hlmBtn
          variant="outline"
          size="sm"
          type="button"
          (click)="playing.set(!playing())"
        >
          {{ playing() ? 'Pause' : 'Play' }}
        </button>
        <button
          hlmBtn
          variant="outline"
          size="sm"
          type="button"
          (click)="view('home')"
        >
          Home
        </button>
        <button
          hlmBtn
          variant="outline"
          size="sm"
          type="button"
          (click)="view('transit')"
        >
          Transit view
        </button>
        <div class="flex items-center gap-1.5 text-xs">
          <hlm-checkbox
            inputId="preview-trueScale"
            [checked]="trueScale()"
            (checkedChange)="trueScale.set($event)"
          /><label for="preview-trueScale">True scale</label>
        </div>
        <div class="flex items-center gap-1.5 text-xs">
          <hlm-checkbox
            inputId="preview-habitableZone"
            [checked]="habitableZone()"
            (checkedChange)="habitableZone.set($event)"
          /><label for="preview-habitableZone">Habitable zone</label>
        </div>
      </div>
    }
    <div class="flex items-center gap-1.5 text-xs">
      <hlm-checkbox
        inputId="preview-includeUnlikely"
        [checked]="includeUnlikely()"
        (checkedChange)="includeUnlikely.set($event)"
      /><label for="preview-includeUnlikely"
        >Include unlikely and provisional candidates</label
      >
    </div>
  `,
})
export class SystemPreview {
  readonly target = input.required<Target>();
  readonly details = input<StarDetails | null>(null);
  readonly rows = input.required<ScoredRow[]>();
  readonly selected = input<ScoredRow | null>(null);
  readonly picked = output<ScoredRow | null>();

  protected readonly badge = "Hypothetical · artist's impression";
  protected readonly playing = signal(true);
  protected readonly trueScale = signal(false);
  protected readonly habitableZone = signal(true);
  protected readonly includeUnlikely = signal(false);
  private readonly star = viewChild(StarSystemComponent);

  protected readonly ready = computed(() => hasStarData(this.details()));
  protected readonly preview = computed(() =>
    toSystemPreview(
      this.target(),
      this.details(),
      this.rows(),
      this.includeUnlikely(),
    ),
  );
  protected readonly subtitle = computed(() => {
    const d = this.details();
    return [
      this.target().display_name,
      d?.spectral_type,
      d?.teff_k ? `${Math.round(d.teff_k)} K` : null,
    ]
      .filter(Boolean)
      .join(' · ');
  });

  constructor() {
    // Selecting a candidate elsewhere (the ranking) moves the camera onto its planet.
    effect(() => {
      const view = this.star()?.view();
      const selected = this.selected();
      if (!view) return;
      const index = this.preview().planets.findIndex(
        (p) => p.scored === selected,
      );
      if (index >= 0 && view.following !== index) view.follow(index);
      else if (selected === null && view.following !== null) view.unfollow();
    });
  }

  protected view(preset: 'home' | 'transit'): void {
    this.star()?.setView(preset);
  }

  protected followed(index: number | null): void {
    this.picked.emit(
      index === null ? null : (this.preview().planets[index]?.scored ?? null),
    );
  }
}
