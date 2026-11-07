import {
  applicationConfig,
  type Meta,
  type StoryObj,
} from '@storybook/angular';
import { events, jobResult, stage } from '../../../../stories/fixtures';
import { failure, provideRun } from '../../../../stories/run-store.mock';
import { Process } from './process';

const run = (extract: Parameters<typeof stage>[0]) =>
  applicationConfig({
    providers: [...provideRun({ extract: stage(extract), score: stage({}) })],
  });

const all = () => [...events.download(), ...events.process()];

const meta: Meta<Process> = { title: 'Pipeline/2 Process', component: Process };
export default meta;
type Story = StoryObj<Process>;

export const Searching: Story = {
  decorators: [run({ status: 'running', events: all().slice(0, 9) })],
};

export const CandidatesFound: Story = {
  decorators: [
    run({ status: 'done', events: all(), result: jobResult(false) }),
  ],
};

export const NoCandidates: Story = {
  decorators: [
    run({
      status: 'done',
      events: all(),
      result: { ...jobResult(false), features: [] },
    }),
  ],
};

export const Failed: Story = {
  decorators: [
    run({
      status: 'failed',
      events: all().slice(0, 8),
      error: failure('extraction timed out'),
    }),
  ],
};

export const FromCache: Story = {
  decorators: [run({ status: 'done', cached: true, result: jobResult(false) })],
};
