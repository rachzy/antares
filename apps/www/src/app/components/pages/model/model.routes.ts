import { Route } from '@angular/router';

export const modelRoutes: Route[] = [
  { path: '', pathMatch: 'full', redirectTo: 'hyperparameters' },
  {
    path: 'hyperparameters',
    loadComponent: () =>
      import('./hyperparameters').then((m) => m.Hyperparameters),
  },
  {
    path: 'dataset',
    loadComponent: () => import('./dataset').then((m) => m.Dataset),
  },
];
