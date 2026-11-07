import {
  applicationConfig,
  type Meta,
  type StoryObj,
} from '@storybook/angular';
import { events, jobResult, stage } from '../../../../stories/fixtures';
import { failure, provideRun } from '../../../../stories/run-store.mock';
import { Download } from './download';

const run = (extract: Parameters<typeof stage>[0]) =>
  applicationConfig({
    providers: [...provideRun({ extract: stage(extract), score: stage({}) })],
  });

const meta: Meta<Download> = {
  title: 'Pipeline/1 Download',
  component: Download,
};
export default meta;
type Story = StoryObj<Download>;

export const Connecting: Story = {
  decorators: [run({ status: 'running' })],
};

export const Downloading: Story = {
  decorators: [
    run({ status: 'running', events: events.download().slice(0, 4) }),
  ],
};

export const Downloaded: Story = {
  decorators: [
    run({
      status: 'running',
      events: [...events.download(), ...events.process().slice(0, 1)],
    }),
  ],
};

export const Failed: Story = {
  decorators: [
    run({
      status: 'failed',
      events: events.download().slice(0, 3),
      error: failure('no light curves found for KIC 1'),
    }),
  ],
};

export const FromCache: Story = {
  decorators: [run({ status: 'done', cached: true, result: jobResult(false) })],
};
