import { Route } from '@angular/router';
import { modelRoutes } from './components/pages/model/model.routes';
import { pipelineRoutes } from './components/pages/pipeline/pipeline.routes';

export const appRoutes: Route[] = [
  {
    path: '',
    loadComponent: () =>
      import('./components/pages/shell/shell').then((m) => m.Shell),
    children: [
      {
        path: '',
        pathMatch: 'full',
        loadComponent: () =>
          import('./components/pages/welcome/welcome').then((m) => m.Welcome),
      },
      {
        path: '',
        loadComponent: () =>
          import('./components/pages/pipeline/pipeline').then(
            (m) => m.Pipeline,
          ),
        children: pipelineRoutes,
      },
      { path: 'model', children: modelRoutes },
      {
        path: 'api',
        loadComponent: () =>
          import('./components/pages/api/api').then((m) => m.Api),
      },
      { path: '**', redirectTo: '' },
    ],
  },
];
