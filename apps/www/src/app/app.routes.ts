import { Route } from '@angular/router';
import { pipelineRoutes } from './components/pages/pipeline/pipeline.routes';

export const appRoutes: Route[] = [
  {
    path: '',
    loadComponent: () =>
      import('./components/pages/welcome/welcome').then((m) => m.Welcome),
  },
  {
    path: '',
    loadComponent: () =>
      import('./components/pages/pipeline/pipeline').then((m) => m.Pipeline),
    children: pipelineRoutes,
  },
  { path: '**', redirectTo: '' },
];
