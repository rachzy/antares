import { Component, input } from '@angular/core';
import { HlmTableImports } from '@spartan-ng/helm/table';
import {
  DASH,
  fixed,
  percent,
  periodLabel,
  type ScoredRow,
} from '../../utils/scoring';

@Component({
  imports: [HlmTableImports],
  selector: 'an-score-ranking',
  host: { class: 'block w-full' },
  template: `
    <div hlmTableContainer>
      <table hlmTable class="tabular-nums">
        <thead hlmTHead>
          <tr hlmTr>
            <th hlmTh class="text-muted-foreground">Period</th>
            <th hlmTh class="text-muted-foreground">Depth (MES)</th>
            <th hlmTh class="text-muted-foreground">Planet probability</th>
            <th hlmTh></th>
          </tr>
        </thead>
        <tbody hlmTBody>
          @for (r of rows(); track $index) {
            <tr hlmTr>
              <td hlmTd class="font-mono">{{ periodLabel(r.period) }}</td>
              <td hlmTd>{{ depth(r) }}</td>
              <td hlmTd class="w-full">
                <div class="bg-muted relative h-2 min-w-40 rounded">
                  <div
                    data-fill
                    class="bg-primary absolute inset-y-0 left-0 rounded"
                    [style.width.%]="percent(r.probability)"
                  ></div>
                  @if (r.threshold >= 0) {
                    <div
                      data-tick
                      class="bg-foreground/70 absolute -inset-y-[3px] w-0.5"
                      [style.left.%]="percent(r.threshold)"
                    ></div>
                  }
                </div>
              </td>
              <td hlmTd>
                <span
                  data-tag
                  class="rounded-full border px-2 py-px text-[11px]"
                  [class]="
                    r.decision === 'POTENTIAL'
                      ? 'border-primary text-primary'
                      : 'border-border text-muted-foreground'
                  "
                >
                  {{ r.decision }} {{ label(r.probability) }}
                </span>
              </td>
            </tr>
          }
        </tbody>
      </table>
    </div>
    <p class="text-muted-foreground mt-2.5 text-xs">
      The white tick is the model's decision threshold.
    </p>
  `,
})
export class ScoreRanking {
  readonly rows = input.required<ScoredRow[]>();

  protected readonly periodLabel = periodLabel;
  protected readonly percent = percent;

  protected depth(r: ScoredRow): string {
    const mes = r.row['MES'];
    return typeof mes === 'number' ? fixed(mes, 1) : DASH;
  }

  protected label(p: number): string {
    return Number.isFinite(p) ? `${Math.round(p * 100)}%` : DASH;
  }
}
