import { Route } from '@angular/router';
import { stepGuard } from './step.guard';

export const pipelineRoutes: Route[] = [
  {
    path: 'select',
    canActivate: [stepGuard('select')],
    loadComponent: () => import('../select/select').then((m) => m.Select),
  },
  {
    path: 'download',
    canActivate: [stepGuard('download')],
    loadComponent: () => import('../download/download').then((m) => m.Download),
  },
  {
    path: 'process',
    canActivate: [stepGuard('process')],
    loadComponent: () => import('../process/process').then((m) => m.Process),
  },
  {
    path: 'score',
    canActivate: [stepGuard('score')],
    loadComponent: () => import('../score/score').then((m) => m.Score),
  },
];
