import { computed, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import type { SubscriptionEvent } from '../../../api/api.types';
import { RunStore, type Stage } from '../../../stores/run.store';
import { clock } from '../../ui/job-log/job-log';
import { Download } from './download';

const at = '2026-10-18T10:02:11Z';
const ev = (
  seq: number,
  stage: string,
  message?: string,
): SubscriptionEvent => ({
  seq,
  type: 'state',
  stage,
  message,
  at,
});

const idle: Stage = {
  status: 'idle',
  token: null,
  events: [],
  result: null,
  error: null,
  cached: false,
};

describe('Download', () => {
  const extract = signal<Stage>(idle);
  const retry = vi.fn();

  beforeEach(async () => {
    extract.set({ ...idle, status: 'running' });
    retry.mockReset();
    await TestBed.configureTestingModule({
      imports: [Download],
      providers: [
        provideRouter([]),
        {
          provide: RunStore,
          useValue: {
            extract,
            retry,
            downloadDone: computed(
              () =>
                extract().status === 'done' ||
                extract().events.some((e) => e.stage === 'detrending'),
            ),
          },
        },
      ],
    }).compileComponents();
  });

  const mount = async () => {
    const fixture = TestBed.createComponent(Download);
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const lines = () =>
      Array.from(el.querySelectorAll('[data-line]')).map((l) =>
        l.textContent?.replace(/\s+/g, ' ').trim(),
      );
    const control = (text: string) =>
      Array.from(el.querySelectorAll<HTMLElement>('a, button')).find((b) =>
        b.textContent?.includes(text),
      );
    return { fixture, el, lines, control };
  };

  it('shows the download events with their time and hides later stages', async () => {
    extract.set({
      ...idle,
      status: 'running',
      events: [
        ev(1, 'queued'),
        ev(2, 'downloading', 'Downloading KIC 1'),
        ev(3, 'detrending', 'Detrending'),
      ],
    });
    const { lines } = await mount();
    expect(lines()).toEqual([
      `${clock(at)} queued`,
      `${clock(at)} downloading: Downloading KIC 1`,
      `${clock(at)} done: light curves downloaded`,
    ]);
  });

  it('shows a link to Process once the download is done', async () => {
    extract.set({ ...idle, status: 'running', events: [ev(1, 'downloading')] });
    const { fixture, control } = await mount();
    expect(control('Continue to processing')).toBeUndefined();
    extract.update((s) => ({
      ...s,
      events: [...s.events, ev(2, 'detrending')],
    }));
    await fixture.whenStable();
    expect(control('Continue to processing')?.getAttribute('href')).toBe(
      '/process',
    );
  });

  it('shows a single loaded from cache line on a cached run', async () => {
    extract.set({ ...idle, status: 'done', cached: true });
    const { lines } = await mount();
    expect(lines()).toEqual(['loaded from cache']);
  });

  it('offers Retry when the job failed', async () => {
    extract.set({ ...idle, status: 'failed', events: [ev(1, 'downloading')] });
    const { control } = await mount();
    control('Retry')?.click();
    expect(retry).toHaveBeenCalledOnce();
  });
});
