import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { Pipeline } from './pipeline';
import { pipelineRoutes } from './pipeline.routes';

describe('Pipeline', () => {
  it('renders the routed page in an outlet', async () => {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(Pipeline);
    await fixture.whenStable();
    expect(
      (fixture.nativeElement as HTMLElement).querySelector('router-outlet'),
    ).not.toBeNull();
  });
});

describe('pipelineRoutes', () => {
  it('guards the four steps', () => {
    expect(pipelineRoutes.map((r) => r.path)).toEqual([
      'select',
      'download',
      'process',
      'score',
    ]);
    expect(pipelineRoutes.every((r) => r.canActivate?.length === 1)).toBe(true);
  });

  it('is used as a parent route', async () => {
    TestBed.configureTestingModule({
      providers: [
        provideRouter([
          {
            path: '',
            component: Pipeline,
            children: [{ path: 'x', children: [] }],
          },
        ]),
      ],
    });
    const router = TestBed.inject(Router);
    await router.navigateByUrl('/x');
    expect(router.url).toBe('/x');
  });
});
