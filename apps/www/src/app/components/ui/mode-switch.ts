import { Component, model } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideGlobe, lucideKeyboard } from '@ng-icons/lucide';

export type SelectMode = 'aladin' | 'manual';

@Component({
  selector: 'an-mode-switch',
  imports: [NgIcon],
  providers: [provideIcons({ lucideGlobe, lucideKeyboard })],
  template: `
    <div
      role="group"
      aria-label="How to choose a star"
      class="bg-secondary border-border inline-flex gap-0.5 rounded-lg border p-[3px]"
    >
      <button
        type="button"
        data-mode="aladin"
        [attr.aria-pressed]="mode() === 'aladin'"
        [class]="buttonClass('aladin')"
        (click)="mode.set('aladin')"
      >
        <ng-icon name="lucideGlobe" size="16" />
        Aladin
      </button>
      <button
        type="button"
        data-mode="manual"
        [attr.aria-pressed]="mode() === 'manual'"
        [class]="buttonClass('manual')"
        (click)="mode.set('manual')"
      >
        <ng-icon name="lucideKeyboard" size="16" />
        Manual select
      </button>
    </div>
  `,
  host: { class: 'inline-block' },
})
export class ModeSwitch {
  readonly mode = model<SelectMode>('aladin');

  protected buttonClass(value: SelectMode): string {
    const base =
      'flex h-[34px] items-center gap-2 rounded-md px-3.5 text-sm font-medium transition-colors motion-reduce:transition-none';
    return this.mode() === value
      ? `${base} bg-primary text-primary-foreground`
      : `${base} text-muted-foreground hover:text-foreground`;
  }
}
