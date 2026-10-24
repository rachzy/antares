import { Component, computed, inject } from '@angular/core';
import { CopyButton } from './copy-button';
import { RunStore } from '../../stores/run.store';

@Component({
  selector: 'an-token-badge',
  imports: [CopyButton],
  template: `
    @if (token(); as token) {
      <div
        data-token
        [title]="token"
        class="bg-card/90 border-border text-muted-foreground flex items-center gap-1.5 rounded-lg border py-0.5 pr-1 pl-2.5 font-mono text-[11px] whitespace-nowrap backdrop-blur-sm"
      >
        <span>Token</span>
        <span class="text-foreground min-w-0 flex-1 truncate">{{
          short()
        }}</span>
        <an-copy-button [value]="token" label="Copy token" />
      </div>
    }
  `,
})
export class TokenBadge {
  private readonly run = inject(RunStore);
  protected readonly token = computed(
    () => this.run.score().token ?? this.run.extract().token,
  );
  protected readonly short = computed(() => {
    const token = this.token() ?? '';
    return token.length > 14
      ? `${token.slice(0, 8)}…${token.slice(-4)}`
      : token;
  });
}
