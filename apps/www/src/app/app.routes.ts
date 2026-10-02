import { Route } from '@angular/router';

export const appRoutes: Route[] = [
  {
    path: '',
    loadComponent: () =>
      import('./components/pages/welcome/welcome').then((m) => m.Welcome),
  },
];
