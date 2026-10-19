import {
  afterRenderEffect,
  Component,
  computed,
  type ElementRef,
  input,
  viewChild,
} from '@angular/core';
import type { SubscriptionEvent } from '../../../api/api.types';
import type { Stage } from '../../../stores/run.store';

export function splitByStage(events: SubscriptionEvent[]): {
  download: SubscriptionEvent[];
  process: SubscriptionEvent[];
} {
  const i = events.findIndex((e) => e.stage === 'detrending');
  if (i < 0) return { download: events, process: [] };
  return { download: events.slice(0, i), process: events.slice(i) };
}

const pad = (n: number) => String(n).padStart(2, '0');

export function clock(at: string): string {
  const d = new Date(at);
  if (Number.isNaN(d.getTime())) return '';
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function summary(e: SubscriptionEvent): string {
  if (e.type === 'error') return `error: ${e.message ?? e.stage ?? 'failed'}`;
  const parts = [
    e.message ? `${e.stage ?? e.type}: ${e.message}` : (e.stage ?? e.type),
  ];
  if (e.fraction !== undefined) parts.push(`${Math.round(e.fraction * 100)}%`);
  return parts.join(' ');
}

@Component({
  selector: 'an-job-log',
  templateUrl: './job-log.html',
  host: { class: 'flex min-h-0 flex-1 flex-col' },
})
export class JobLog {
  readonly events = input<SubscriptionEvent[]>([]);
  readonly status = input<Stage['status']>('idle');
  readonly done = input<string | null>(null);
  readonly note = input<string | null>(null);

  private readonly box = viewChild.required<ElementRef<HTMLElement>>('box');

  protected readonly lines = computed(() =>
    this.events()
      .filter((e) => e.type !== 'result')
      .map((e) => ({
        seq: e.seq,
        time: clock(e.at),
        text: summary(e),
        error: e.type === 'error',
      })),
  );

  protected readonly finish = computed(() => {
    const done = this.done();
    if (this.status() !== 'done' || !done) return null;
    const last = this.events().at(-1);
    return `${last ? clock(last.at) + ' ' : ''}done: ${done}`;
  });

  constructor() {
    afterRenderEffect(() => {
      this.lines();
      this.finish();
      const el = this.box().nativeElement;
      el.scrollTop = el.scrollHeight;
    });
  }
}
