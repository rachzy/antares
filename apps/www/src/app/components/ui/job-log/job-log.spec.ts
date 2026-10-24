import { TestBed } from '@angular/core/testing';
import type { SubscriptionEvent } from '../../../api/api.types';
import { clock, JobLog, splitByStage } from './job-log';

const at = '2026-10-18T10:02:11Z';
const ev = (
  seq: number,
  stage: string,
  extra: Partial<SubscriptionEvent> = {},
): SubscriptionEvent => ({ seq, type: 'progress', stage, at, ...extra });

describe('splitByStage', () => {
  it('splits at the first detrending event', () => {
    const events = [
      ev(1, 'queued', { type: 'state' }),
      ev(2, 'downloading'),
      ev(3, 'downloading'),
      ev(4, 'detrending'),
      ev(5, 'period_search'),
    ];
    const { download, process } = splitByStage(events);
    expect(download.map((e) => e.seq)).toEqual([1, 2, 3]);
    expect(process.map((e) => e.seq)).toEqual([4, 5]);
  });

  it('keeps everything in download until detrending arrives', () => {
    const { download, process } = splitByStage([ev(1, 'downloading')]);
    expect(download).toHaveLength(1);
    expect(process).toEqual([]);
  });
});

describe('JobLog', () => {
  const mount = async (inputs: Record<string, unknown>) => {
    const fixture = TestBed.createComponent(JobLog);
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const lines = () =>
      Array.from(el.querySelectorAll<HTMLElement>('[data-line]')).map((l) =>
        l.textContent?.replace(/\s+/g, ' ').trim(),
      );
    return { fixture, el, lines };
  };

  it('prints each event with its time, stage, message and fraction', async () => {
    const { lines } = await mount({
      events: [
        ev(1, 'downloading', {
          type: 'state',
          message: 'Downloading KIC 1 from Kepler',
        }),
        ev(2, 'period_search', { fraction: 0.62 }),
      ],
      status: 'running',
    });
    expect(lines()).toEqual([
      `${clock(at)} downloading: Downloading KIC 1 from Kepler`,
      `${clock(at)} period_search 62%`,
    ]);
  });

  it('skips result events and adds an orange done line when finished', async () => {
    const { el, lines } = await mount({
      events: [ev(1, 'detrending'), ev(2, 'done', { type: 'result' })],
      status: 'done',
      done: '5 candidates extracted',
    });
    expect(lines()).toEqual([
      `${clock(at)} detrending`,
      `${clock(at)} done: 5 candidates extracted`,
    ]);
    expect(el.querySelector('[data-done]')?.className).toContain(
      'text-primary',
    );
  });

  it('uses the mono font', async () => {
    const { el } = await mount({ events: [], status: 'running' });
    expect(el.querySelector('[data-log]')?.className).toContain('font-mono');
  });

  it('prints error events', async () => {
    const { lines } = await mount({
      events: [ev(1, 'downloading', { type: 'error', message: 'boom' })],
      status: 'failed',
    });
    expect(lines()).toEqual([`${clock(at)} error: boom`]);
  });

  it('shows extra lines given by the page', async () => {
    const { lines } = await mount({
      events: [],
      status: 'done',
      note: 'loaded from cache',
    });
    expect(lines()[0]).toBe('loaded from cache');
  });

  it('shows the waiting line until the first event arrives', async () => {
    const { fixture, lines } = await mount({
      events: [],
      status: 'running',
      waiting: 'Establishing connection to Shaula...',
    });
    expect(lines()).toEqual(['Establishing connection to Shaula...']);
    fixture.componentRef.setInput('events', [ev(1, 'queued')]);
    await fixture.whenStable();
    expect(lines()).toEqual([`${clock(at)} queued`]);
  });

  it('shows no waiting line when the job is not running', async () => {
    const { lines } = await mount({
      events: [],
      status: 'idle',
      waiting: 'Establishing connection to Shaula...',
    });
    expect(lines()).toEqual([]);
  });

  it('scrolls to the bottom', async () => {
    const { fixture, el } = await mount({ events: [], status: 'running' });
    const box = el.querySelector<HTMLElement>('[data-log]') as HTMLElement;
    Object.defineProperty(box, 'scrollHeight', { value: 500 });
    fixture.componentRef.setInput('events', [ev(1, 'downloading')]);
    await fixture.whenStable();
    expect(box.scrollTop).toBe(500);
  });
});
