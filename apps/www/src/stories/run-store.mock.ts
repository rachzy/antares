import { signal } from '@angular/core';
import type { Provider } from '@angular/core';
import { ApiRequestError } from '../app/api/api.service';
import { RunStore, type Stage } from '../app/stores/run.store';
import { StarInfoStore } from '../app/stores/star-info.store';
import { starDetails, target } from './fixtures';

interface RunFixture {
  extract: Stage;
  score: Stage;
}

/** Stands in for RunStore, so a page renders the state it would have mid-pipeline. */
export function provideRun({ extract, score }: RunFixture): Provider[] {
  const candidates = extract.result?.features.length ?? 0;
  const log = (name: string) => () => console.info(`[run] ${name}`);
  const downloadDone =
    extract.status === 'done' ||
    extract.events.some((e) => e.stage === 'detrending');
  return [
    {
      provide: RunStore,
      useValue: {
        star: signal(target),
        extract: signal(extract),
        score: signal(score),
        downloadDone: signal(downloadDone),
        processDone: signal(extract.status === 'done'),
        scoreDone: signal(score.status === 'done'),
        candidates: signal(candidates),
        startScore: log('startScore'),
        retry: log('retry'),
      },
    },
    {
      provide: StarInfoStore,
      useValue: {
        details: signal(starDetails),
        loading: signal(false),
        failed: signal(false),
        load: log('starInfo.load'),
      },
    },
  ];
}

export const failure = (message: string) =>
  new ApiRequestError(0, 'internal', message);
