import { Component, computed, input } from '@angular/core';
import { HlmTableImports } from '@spartan-ng/helm/table';
import {
  decisionModel,
  fixed,
  heat,
  modelScore,
  periodLabel,
  scoreColumns,
  type ScoredRow,
} from '../../utils/scoring';

@Component({
  imports: [HlmTableImports],
  selector: 'an-score-matrix',
  host: { class: 'block w-full' },
  template: `
    <p data-copy class="text-muted-foreground mb-2 text-xs">
      Each model scores every candidate; the run uses
      <b class="text-foreground">{{ model() }}</b> for the final decision.
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
                @let p = modelScore(r, m, model());
                <td hlmTd>
                  <div
                    data-cell
                    class="relative rounded px-1.5 py-2.5 text-center"
                    [class.font-semibold]="m === model()"
                  >
                    @if (p >= 0) {
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

  private readonly source = computed(() => this.rows().map((r) => r.row));
  protected readonly model = computed(() => decisionModel(this.source()));
  protected readonly columns = computed(() => scoreColumns(this.source()));
  protected readonly modelScore = modelScore;
  protected readonly periodLabel = periodLabel;
  protected readonly fixed = fixed;
  protected readonly heat = heat;
}
