import {
  Component,
  computed,
  effect,
  input,
  model,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  lucideChevronLeft,
  lucideChevronRight,
  lucideMaximize2,
  lucideMinimize2,
} from '@ng-icons/lucide';
import { StarSystemComponent } from '@larawag/angular';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmCheckboxImports } from '@spartan-ng/helm/checkbox';
import type { StarDetails, Target } from '../../api/api.types';
import type { ScoredRow } from '../../utils/scoring';
import { hasStarData, toSystemPreview } from '../../utils/system-preview';

export type PreviewMode = 'expanded' | 'collapsed' | 'maximized';

@Component({
  imports: [HlmButtonImports, HlmCheckboxImports, NgIcon, StarSystemComponent],
  selector: 'an-system-preview',
  providers: [
    provideIcons({
      lucideChevronLeft,
      lucideChevronRight,
      lucideMaximize2,
      lucideMinimize2,
    }),
  ],
  host: {
    class:
      'bg-card relative flex min-h-0 min-w-0 overflow-hidden rounded-[10px] transition-[flex-grow,flex-basis,padding] duration-300 ease-in-out motion-reduce:transition-none',
    '[class]': 'hostClass()',
  },
  template: `
    <div
      data-collapsed
      class="absolute inset-0 flex items-start justify-center p-1.5 transition-opacity duration-200 motion-reduce:transition-none"
      [class]="
        collapsed() ? 'opacity-100 delay-100' : 'pointer-events-none opacity-0'
      "
      [attr.inert]="collapsed() ? null : ''"
    >
      <button
        hlmBtn
        variant="outline"
        size="icon-xs"
        type="button"
        title="Expand system preview"
        aria-label="Expand system preview"
        (click)="mode.set('expanded')"
      >
        <ng-icon name="lucideChevronLeft" class="max-lg:rotate-90" />
      </button>
      <span
        class="text-muted-foreground absolute top-11 text-xs [writing-mode:vertical-rl] max-lg:hidden"
        >System preview</span
      >
    </div>
    <div
      data-body
      class="flex min-h-0 min-w-0 flex-1 flex-col gap-2.5 p-3.5 transition-opacity duration-200 motion-reduce:transition-none"
      [class]="collapsed() ? 'pointer-events-none opacity-0' : 'opacity-100'"
      [attr.inert]="collapsed() ? '' : null"
    >
      <div class="flex items-center gap-2">
        <h3 class="text-sm font-medium">System preview</h3>
        <span class="text-muted-foreground min-w-0 truncate text-xs">{{
          subtitle()
        }}</span>
        <div class="ml-auto flex shrink-0 gap-1.5">
          <button
            hlmBtn
            variant="outline"
            size="icon-xs"
            type="button"
            [title]="maximized() ? 'Restore' : 'Maximize'"
            [attr.aria-label]="
              maximized() ? 'Restore system preview' : 'Maximize system preview'
            "
            (click)="mode.set(maximized() ? 'expanded' : 'maximized')"
          >
            <ng-icon
              [name]="maximized() ? 'lucideMinimize2' : 'lucideMaximize2'"
            />
          </button>
          <button
            hlmBtn
            variant="outline"
            size="icon-xs"
            type="button"
            title="Collapse system preview"
            aria-label="Collapse system preview"
            (click)="mode.set('collapsed')"
          >
            <ng-icon name="lucideChevronRight" class="max-lg:rotate-90" />
          </button>
        </div>
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
        <div
          data-canvas
          class="min-h-64 overflow-hidden rounded-lg bg-black transition-[height] duration-300 ease-in-out motion-reduce:transition-none"
          [class]="maximized() ? 'flex-1' : 'h-[26rem]'"
        >
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
            Reset view
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
    </div>
  `,
})
export class SystemPreview {
  readonly target = input.required<Target>();
  readonly details = input<StarDetails | null>(null);
  readonly rows = input.required<ScoredRow[]>();
  readonly selected = input<ScoredRow | null>(null);
  readonly picked = output<ScoredRow | null>();
  readonly mode = model<PreviewMode>('expanded');

  protected readonly badge = "Hypothetical · artist's impression";
  protected readonly playing = signal(true);
  protected readonly trueScale = signal(false);
  protected readonly habitableZone = signal(true);
  protected readonly includeUnlikely = signal(false);
  private readonly star = viewChild(StarSystemComponent);

  protected readonly collapsed = computed(() => this.mode() === 'collapsed');
  protected readonly maximized = computed(() => this.mode() === 'maximized');
  protected readonly hostClass = computed(() =>
    this.collapsed() ? 'grow-0 basis-11' : 'grow basis-0',
  );

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
