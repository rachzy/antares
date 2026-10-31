import { Component, input, output, signal } from '@angular/core';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { manualQuery } from './manual-query';

@Component({
  selector: 'an-manual-select',
  imports: [HlmButtonImports, HlmInputImports],
  template: `
    <form class="flex flex-col gap-3.5" novalidate (submit)="submit($event)">
      <div class="flex flex-col gap-1.5">
        <label
          [for]="idPrefix() + '-name'"
          class="text-muted-foreground text-xs font-medium"
          >Star name or catalog ID</label
        >
        <input
          hlmInput
          [id]="idPrefix() + '-name'"
          name="name"
          autocomplete="off"
          placeholder="Kepler-186, KIC 8120608…"
          [value]="name()"
          (input)="name.set($any($event.target).value)"
        />
      </div>
      <div
        class="text-muted-foreground flex items-center gap-3 text-xs"
        aria-hidden="true"
      >
        <span class="border-border flex-1 border-t"></span>
        or coordinates (J2000)
        <span class="border-border flex-1 border-t"></span>
      </div>
      <div class="grid grid-cols-2 gap-3">
        <div class="flex min-w-0 flex-col gap-1.5">
          <label
            [for]="idPrefix() + '-ra'"
            class="text-muted-foreground text-xs font-medium"
            >RA</label
          >
          <input
            hlmInput
            class="font-mono"
            [id]="idPrefix() + '-ra'"
            name="ra"
            autocomplete="off"
            placeholder="298.65 or 19h 54m 36.7s"
            [value]="ra()"
            (input)="ra.set($any($event.target).value)"
          />
        </div>
        <div class="flex min-w-0 flex-col gap-1.5">
          <label
            [for]="idPrefix() + '-dec'"
            class="text-muted-foreground text-xs font-medium"
            >Dec</label
          >
          <input
            hlmInput
            class="font-mono"
            [id]="idPrefix() + '-dec'"
            name="dec"
            autocomplete="off"
            placeholder="+43.95 or +43° 57′ 18″"
            [value]="dec()"
            (input)="dec.set($any($event.target).value)"
          />
        </div>
      </div>
      @if (error(); as message) {
        <p role="alert" class="text-destructive text-sm">{{ message }}</p>
      }
      <button hlmBtn type="submit" class="self-end">Look up star</button>
    </form>
  `,
  host: { class: 'block' },
})
export class ManualSelect {
  readonly idPrefix = input('manual');
  readonly lookup = output<string>();

  protected readonly name = signal('');
  protected readonly ra = signal('');
  protected readonly dec = signal('');
  protected readonly error = signal<string | null>(null);

  protected submit(event: Event): void {
    event.preventDefault();
    const result = manualQuery(this.name(), this.ra(), this.dec());
    if ('error' in result) {
      this.error.set(result.error);
      return;
    }
    this.error.set(null);
    this.lookup.emit(result.query);
  }
}
