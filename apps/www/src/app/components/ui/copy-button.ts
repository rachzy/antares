import { Component, computed, input, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { faSolidCheck, faSolidCopy } from '@ng-icons/font-awesome/solid';
import { HlmButtonImports } from '@spartan-ng/helm/button';

const FLASH_MS = 1500;

@Component({
  selector: 'an-copy-button',
  imports: [NgIcon, HlmButtonImports],
  providers: [provideIcons({ faSolidCopy, faSolidCheck })],
  template: `
    <button
      hlmBtn
      variant="ghost"
      size="icon-xs"
      type="button"
      [attr.aria-label]="copied() ? 'Copied' : label()"
      (click)="copy()"
    >
      @if (copied()) {
        <ng-icon data-icon="check" name="faSolidCheck" class="text-success" />
      } @else {
        <ng-icon data-icon="copy" name="faSolidCopy" />
      }
    </button>
  `,
})
export class CopyButton {
  readonly value = input.required<string>();
  readonly label = input('Copy');
  protected readonly copied = signal(false);

  protected async copy(): Promise<void> {
    try {
      await navigator.clipboard.writeText(this.value());
    } catch {
      return;
    }
    this.copied.set(true);
    setTimeout(() => this.copied.set(false), FLASH_MS);
  }
}
