import type { Meta, StoryObj } from '@storybook/angular';
import { starDetails } from '../../../../stories/fixtures';
import { StarDetailsView } from './star-details';

const meta: Meta<StarDetailsView> = {
  title: 'Select/Star card',
  component: StarDetailsView,
};
export default meta;
type Story = StoryObj<StarDetailsView>;

export const Loaded: Story = { args: { details: starDetails } };
export const Loading: Story = { args: { loading: true } };
export const Failed: Story = { args: { failed: true } };
export const NoCatalogData: Story = {
  args: { details: { aliases: [], planets: [], sources: [] } },
};
