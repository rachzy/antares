import {
  applicationConfig,
  type Meta,
  type StoryObj,
} from '@storybook/angular';
import { events, jobResult, stage } from '../../../../stories/fixtures';
import { failure, provideRun } from '../../../../stories/run-store.mock';
import { Score } from './score';

const extract = stage({ status: 'done', result: jobResult(false) });
const run = (score: Parameters<typeof stage>[0]) =>
  applicationConfig({
    providers: [...provideRun({ extract, score: stage(score) })],
  });

const meta: Meta<Score> = { title: 'Pipeline/3 Score', component: Score };
export default meta;
type Story = StoryObj<Score>;

export const NotScoredYet: Story = {
  decorators: [run({ status: 'idle', token: null })],
};

export const Scoring: Story = {
  decorators: [
    run({ status: 'running', events: events.predict().slice(0, 2) }),
  ],
};

export const Scored: Story = {
  decorators: [
    run({ status: 'done', events: events.predict(), result: jobResult(true) }),
  ],
};

export const NothingScored: Story = {
  decorators: [
    run({
      status: 'done',
      events: events.predict(),
      result: { ...jobResult(true), predictions: [] },
    }),
  ],
};

export const Failed: Story = {
  decorators: [
    run({
      status: 'failed',
      events: events.predict().slice(0, 2),
      error: failure('prediction failed: schema mismatch'),
    }),
  ],
};
