import { Component, computed, inject } from '@angular/core';
import { RunStore } from '../../stores/run.store';

@Component({
  selector: 'an-token-badge',
  template: `
    @if (token(); as token) {
      <div
        data-token
        [title]="token"
        class="bg-card/90 border-border text-muted-foreground fixed top-3 right-4 z-40 flex items-center gap-2 rounded-lg border px-2.5 py-1 font-mono text-xs backdrop-blur-sm"
      >
        <span>Token</span>
        <span class="text-foreground">{{ short() }}</span>
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
