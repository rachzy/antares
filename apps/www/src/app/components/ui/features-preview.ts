import { Component, computed, input } from '@angular/core';
import { HlmTableImports } from '@spartan-ng/helm/table';
import type { FeatureRow } from '../../api/api.types';

function show(value: unknown): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'number') {
    return Number.isInteger(value)
      ? String(value)
      : String(+value.toPrecision(6));
  }
  return typeof value === 'object' ? JSON.stringify(value) : String(value);
}

@Component({
  selector: 'an-features-preview',
  imports: [HlmTableImports],
  host: { class: 'flex min-h-0 flex-1 flex-col gap-1.5' },
  template: `
    <p class="text-muted-foreground text-xs">
      Extracted features · {{ rows().length }}
      {{ rows().length === 1 ? 'row' : 'rows' }}
    </p>
    <div
      hlmTableContainer
      class="border-border bg-card min-h-0 flex-1 overflow-auto rounded-[10px] border"
    >
      <table hlmTable class="font-mono text-xs tabular-nums">
        <thead hlmTHead class="bg-card sticky top-0">
          <tr hlmTr>
            @for (column of columns(); track column) {
              <th hlmTh class="text-muted-foreground whitespace-nowrap">
                {{ column }}
              </th>
            }
          </tr>
        </thead>
        <tbody hlmTBody>
          @for (row of rows(); track $index) {
            <tr hlmTr>
              @for (column of columns(); track column) {
                <td hlmTd class="whitespace-nowrap">{{ show(row[column]) }}</td>
              }
            </tr>
          }
        </tbody>
      </table>
    </div>
  `,
})
export class FeaturesPreview {
  readonly rows = input.required<FeatureRow[]>();
  protected readonly columns = computed(() => [
    ...new Set(this.rows().flatMap((row) => Object.keys(row))),
  ]);
  protected readonly show = show;
}
