import { Route } from '@angular/router';

export const appRoutes: Route[] = [
  {
    path: '',
    loadComponent: () => import('./sky-viewer/sky-viewer').then((m) => m.SkyViewer),
  },
];
