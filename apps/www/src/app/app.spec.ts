import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { App } from './app';
import { appRoutes } from './app.routes';
import { ErrorToaster } from './components/ui/error-toaster';
import { Shell } from './components/pages/shell/shell';
import { Welcome } from './components/pages/welcome/welcome';
import { HealthStore } from './stores/health.store';
import { RunStore } from './stores/run.store';

@Component({ selector: 'an-error-toaster', template: '' })
class ErrorToasterStub {}

describe('App', () => {
  const health = { check: vi.fn() };
  const run = {
    restore: vi.fn(),
    star: signal(null),
    extract: signal({ token: null }),
    score: signal({ token: null }),
  };

  beforeEach(async () => {
    health.check.mockClear();
    run.restore.mockClear();
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [
        provideRouter(appRoutes),
        { provide: HealthStore, useValue: health },
        { provide: RunStore, useValue: run },
      ],
    })
      .overrideComponent(App, {
        remove: { imports: [ErrorToaster] },
        add: { imports: [ErrorToasterStub] },
      })
      .compileComponents();
  });

  it('checks api health and restores the saved run on init', async () => {
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    expect(health.check).toHaveBeenCalledOnce();
    expect(run.restore).toHaveBeenCalledOnce();
  });

  it('should render a router outlet', async () => {
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('router-outlet')).not.toBeNull();
  });
});

describe('appRoutes', () => {
  it('wraps every page in the shell', async () => {
    const [route] = appRoutes;
    expect(route.path).toBe('');
    const loadComponent = route.loadComponent as () => Promise<typeof Shell>;
    await expect(loadComponent()).resolves.toBe(Shell);
  });

  it('routes the empty path to Welcome', async () => {
    const welcome = appRoutes[0].children?.[0];
    expect(welcome?.path).toBe('');
    const loadComponent = welcome?.loadComponent as () => Promise<
      typeof Welcome
    >;
    await expect(loadComponent()).resolves.toBe(Welcome);
  });

  it('routes the model and api pages', () => {
    const children = appRoutes[0].children ?? [];
    expect(children.map((r) => r.path)).toEqual(['', '', 'model', 'api', '**']);
  });

  it('sends unknown urls to the welcome page', async () => {
    TestBed.configureTestingModule({ providers: [provideRouter(appRoutes)] });
    const router = TestBed.inject(Router);
    await router.navigateByUrl('/nope');
    expect(router.url).toBe('/');
  });
});
