import { Component, computed, input, signal } from '@angular/core';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmTableImports } from '@spartan-ng/helm/table';
import type { JobResult } from '../../api/api.types';
import {
  DASH,
  decisionModel,
  featureNames,
  fixed,
  margin,
  modelScore,
  percent,
  periodLabel,
  scoreColumns,
  type ScoredRow,
} from '../../utils/scoring';

const FEATURES = 7;

const text = (value: unknown): string | null =>
  typeof value === 'string' && value ? value : null;

const cell = (value: unknown): string => {
  if (value === null || value === undefined) return DASH;
  if (typeof value === 'number') {
    return Number.isFinite(value) ? String(+value.toPrecision(4)) : DASH;
  }
  return String(value);
};

const signed = (value: number): string => {
  if (!Number.isFinite(value)) return DASH;
  const abs = Math.abs(value).toFixed(3);
  return value >= 0 ? `+${abs}` : `−${abs}`;
};

@Component({
  imports: [HlmButtonImports, HlmTableImports],
  selector: 'an-score-details',
  template: `
    <div class="mb-3.5 flex flex-wrap gap-2">
      @for (chip of chips(); track chip.label) {
        <span
          data-chip
          class="bg-muted inline-flex gap-1 rounded-full px-3 py-1 text-xs"
        >
          <span class="text-muted-foreground">{{ chip.label }}</span>
          <span [class.font-mono]="chip.mono">{{ chip.value }}</span>
        </span>
      }
    </div>

    <h4 class="mb-1.5 text-[13px] font-semibold">
      How each candidate was decided
    </h4>
    <div hlmTableContainer>
      <table hlmTable data-table="decision" class="tabular-nums">
        <thead hlmTHead>
          <tr hlmTr class="text-muted-foreground">
            <th hlmTh class="text-muted-foreground">Candidate</th>
            <th hlmTh class="text-muted-foreground">Star</th>
            <th hlmTh class="text-muted-foreground">Score ({{ model() }})</th>
            <th hlmTh class="text-muted-foreground">Threshold</th>
            <th hlmTh class="text-muted-foreground">Margin</th>
            <th hlmTh class="text-muted-foreground">Decision</th>
          </tr>
        </thead>
        <tbody hlmTBody>
          @for (d of decisions(); track $index) {
            <tr hlmTr>
              <td hlmTd class="font-mono">{{ d.period }}</td>
              <td hlmTd class="font-mono">{{ d.star }}</td>
              <td hlmTd>{{ d.score }}</td>
              <td hlmTd>{{ d.threshold }}</td>
              <td hlmTd [class.text-primary]="d.positive">{{ d.margin }}</td>
              <td hlmTd>
                <span
                  class="rounded-full border px-2 py-px text-[11px]"
                  [class]="
                    d.decision === 'POTENTIAL'
                      ? 'border-primary text-primary'
                      : 'border-border text-muted-foreground'
                  "
                  >{{ d.decision }}</span
                >
              </td>
            </tr>
          }
        </tbody>
      </table>
    </div>

    <h4 class="mt-4.5 mb-1.5 text-[13px] font-semibold">Per-model scores</h4>
    <div hlmTableContainer>
      <table hlmTable data-table="models" class="tabular-nums">
        <thead hlmTHead>
          <tr hlmTr>
            <th hlmTh class="text-muted-foreground">Candidate</th>
            <th hlmTh class="text-muted-foreground">Model</th>
            <th hlmTh class="text-muted-foreground">Probability</th>
            <th hlmTh class="text-muted-foreground">
              vs threshold ({{ fixed(threshold(), 2) }})
            </th>
            <th hlmTh></th>
          </tr>
        </thead>
        <tbody hlmTBody>
          @for (group of groups(); track $index) {
            @for (m of group.models; track m.name; let first = $first) {
              <tr hlmTr>
                @if (first) {
                  <td
                    hlmTd
                    class="align-top font-mono"
                    [attr.rowspan]="group.models.length"
                  >
                    {{ group.period }}
                  </td>
                }
                <td hlmTd [class.font-semibold]="m.used">{{ m.name }}</td>
                <td hlmTd [class.font-semibold]="m.used">
                  {{ fixed(m.p, 2) }}
                </td>
                <td hlmTd class="w-full">
                  <div class="bg-muted relative h-2 min-w-40 rounded">
                    <div
                      data-fill
                      class="absolute inset-y-0 left-0 rounded"
                      [class]="
                        m.used ? 'bg-primary' : 'bg-muted-foreground opacity-50'
                      "
                      [style.width.%]="percent(m.p)"
                    ></div>
                    @if (group.threshold >= 0) {
                      <div
                        data-tick
                        class="bg-foreground/70 absolute -inset-y-[3px] w-0.5"
                        [style.left.%]="percent(group.threshold)"
                      ></div>
                    }
                  </div>
                </td>
                <td hlmTd>
                  @if (m.used) {
                    <span
                      class="border-primary text-primary rounded-full border px-2 py-px text-[11px]"
                      >Used for decision</span
                    >
                  }
                </td>
              </tr>
            }
          }
        </tbody>
      </table>
    </div>
    <p class="text-muted-foreground mt-2 text-xs">
      Fang applies one threshold, to the {{ model() }}. Base models are shown
      against it for reference only.
    </p>

    <h4 class="mt-4.5 mb-1.5 text-[13px] font-semibold">
      Features the models saw
    </h4>
    <div hlmTableContainer>
      <table hlmTable data-table="features" class="tabular-nums">
        <thead hlmTHead>
          <tr hlmTr>
            <th hlmTh class="text-muted-foreground">Feature</th>
            @for (r of rows(); track $index) {
              <th hlmTh class="text-muted-foreground text-right">
                {{ periodLabel(r.period) }}
              </th>
            }
          </tr>
        </thead>
        <tbody hlmTBody>
          @for (name of shown(); track name) {
            <tr hlmTr>
              <td hlmTd class="font-mono">{{ name }}</td>
              @for (r of rows(); track $index) {
                @let value = cell(r.row[name]);
                <td
                  hlmTd
                  class="text-right"
                  [class.text-muted-foreground]="value === dash"
                >
                  {{ value }}
                </td>
              }
            </tr>
          }
        </tbody>
      </table>
    </div>
    <div class="mt-2.5 flex items-center gap-3">
      <p class="text-muted-foreground text-xs">
        Showing {{ shown().length }} of {{ features().length }} features;
        {{ dash }} means the value was missing.
      </p>
      @if (features().length > limit) {
        <button
          hlmBtn
          variant="secondary"
          size="sm"
          type="button"
          (click)="all.set(!all())"
        >
          {{ all() ? 'Show fewer' : 'Show all' }}
        </button>
      }
    </div>
  `,
})
export class ScoreDetails {
  readonly rows = input.required<ScoredRow[]>();
  readonly result = input.required<JobResult>();

  protected readonly all = signal(false);
  protected readonly limit = FEATURES;
  protected readonly dash = DASH;
  protected readonly fixed = fixed;
  protected readonly percent = percent;
  protected readonly periodLabel = periodLabel;
  protected readonly cell = cell;

  private readonly first = computed(() => this.rows()[0]);

  protected readonly model = computed(() =>
    decisionModel(this.rows().map((r) => r.row)),
  );
  protected readonly threshold = computed(() => this.first()?.threshold ?? NaN);

  protected readonly chips = computed(() => {
    const result = this.result();
    const run =
      text(result.model_run_id) ?? text(this.first()?.row['model_run_id']);
    return [
      { label: 'Model', value: this.model(), mono: false },
      { label: 'Threshold', value: fixed(this.threshold(), 2), mono: false },
      { label: 'Run', value: run ?? DASH, mono: true },
      {
        label: 'Schema',
        value:
          result.schema_version === undefined
            ? DASH
            : `v${result.schema_version}`,
        mono: false,
      },
      {
        label: 'Features',
        value: result.cached ? 'from cache' : 'fresh',
        mono: false,
      },
    ];
  });

  protected readonly decisions = computed(() =>
    this.rows().map((r) => {
      const m = margin(r.row);
      return {
        period: periodLabel(r.period),
        star:
          text(r.row['star_id']) ?? this.result().target.designation ?? DASH,
        score: fixed(r.probability, 3),
        threshold: fixed(r.threshold, 3),
        margin: signed(m),
        positive: m >= 0,
        decision: r.decision,
      };
    }),
  );

  protected readonly groups = computed(() => {
    const model = this.model();
    const names = scoreColumns(this.rows().map((r) => r.row));
    return this.rows().map((r) => ({
      period: periodLabel(r.period),
      threshold: r.threshold,
      models: names.map((name) => ({
        name,
        used: name === model,
        p: modelScore(r, name, model),
      })),
    }));
  });

  protected readonly features = computed(() =>
    featureNames(this.rows().map((r) => r.row)),
  );
  protected readonly shown = computed(() =>
    this.all() ? this.features() : this.features().slice(0, FEATURES),
  );
}
