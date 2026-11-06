import type { Meta, StoryObj } from '@storybook/angular';
import { events } from '../../../../stories/fixtures';
import { JobLog } from './job-log';

const meta: Meta<JobLog> = {
  title: 'Pipeline/Job log',
  component: JobLog,
  render: (args) => ({
    props: args,
    template: `<an-job-log [events]="events" [status]="status" [done]="done" [note]="note" [waiting]="waiting" />`,
  }),
};
export default meta;
type Story = StoryObj<JobLog>;

export const Waiting: Story = {
  args: {
    status: 'running',
    waiting: 'Establishing connection to Shaula...',
  },
};

export const Downloading: Story = {
  args: { status: 'running', events: events.download() },
};

export const Failed: Story = {
  args: {
    status: 'failed',
    events: [
      ...events.download().slice(0, 3),
      {
        seq: 99,
        type: 'error',
        message: 'no light curves found for KIC 1',
        at: '2026-11-06T14:03:00Z',
      },
    ],
  },
};

export const Finished: Story = {
  args: {
    status: 'done',
    events: [...events.download(), ...events.process()],
    done: '2 candidates extracted',
  },
};

export const FromCache: Story = {
  args: { status: 'done', note: 'loaded from cache' },
};
