import {
  applicationConfig,
  componentWrapperDecorator,
  type Preview,
} from '@storybook/angular';
import { provideZonelessChangeDetection } from '@angular/core';
import { provideRouter, withDisabledInitialNavigation } from '@angular/router';

if (typeof document !== 'undefined') {
  document.documentElement.classList.add('dark');
}

const preview: Preview = {
  parameters: { layout: 'fullscreen', controls: { disable: true } },
  decorators: [
    applicationConfig({
      providers: [
        provideZonelessChangeDetection(),
        provideRouter([], withDisabledInitialNavigation()),
      ],
    }),
    componentWrapperDecorator(
      (story) =>
        `<div class="bg-background text-foreground flex h-screen flex-col p-4">${story}</div>`,
    ),
  ],
};

export default preview;
