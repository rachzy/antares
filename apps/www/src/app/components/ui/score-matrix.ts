import { Component, computed, input } from '@angular/core';
import { HlmTableImports } from '@spartan-ng/helm/table';
import { fixed, heat, periodLabel, type ScoredRow } from '../../utils/scoring';

@Component({
  imports: [HlmTableImports],
  selector: 'an-score-matrix',
  template: `
    <p class="text-muted-foreground mb-2 text-xs">
      Each model scores every candidate; the run uses
      <b class="text-foreground">stack</b> for the final decision.
    </p>
    <div hlmTableContainer>
      <table hlmTable class="tabular-nums">
        <thead hlmTHead>
          <tr hlmTr>
            <th hlmTh class="text-muted-foreground">Candidate</th>
            @for (m of columns(); track m) {
              <th hlmTh class="text-muted-foreground text-center">{{ m }}</th>
            }
            <th hlmTh class="text-muted-foreground text-center">Decision</th>
          </tr>
        </thead>
        <tbody hlmTBody>
          @for (r of rows(); track $index) {
            <tr hlmTr>
              <td hlmTd class="font-mono">{{ periodLabel(r.period) }}</td>
              @for (m of columns(); track m) {
                @let p = r.models[m] ?? null;
                <td hlmTd>
                  <div
                    data-cell
                    class="relative rounded px-1.5 py-2.5 text-center"
                    [class.font-semibold]="m === 'stack'"
                  >
                    @if (p !== null) {
                      <span
                        data-heat
                        class="bg-primary absolute inset-0 rounded"
                        [style.opacity]="heat(p)"
                      ></span>
                    }
                    <span class="relative">{{ fixed(p, 2) }}</span>
                  </div>
                </td>
              }
              <td hlmTd class="text-center">
                <span
                  data-tag
                  class="rounded-full border px-2 py-px text-[11px]"
                  [class]="
                    r.decision === 'POTENTIAL'
                      ? 'border-primary text-primary'
                      : 'border-border text-muted-foreground'
                  "
                  >{{ r.decision }}</span
                >
              </td>
            </tr>
          }
        </tbody>
      </table>
    </div>
  `,
})
export class ScoreMatrix {
  readonly rows = input.required<ScoredRow[]>();
  readonly models = input.required<string[]>();

  protected readonly columns = computed(() => [...this.models(), 'stack']);
  protected readonly periodLabel = periodLabel;
  protected readonly fixed = fixed;
  protected readonly heat = heat;
}
