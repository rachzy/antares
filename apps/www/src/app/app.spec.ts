import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { App } from './app';
import { appRoutes } from './app.routes';
import { SkyViewer } from './components/containers/sky-viewer/sky-viewer';
import { HealthStore } from './stores/health.store';
import { SubscriptionStore } from './stores/subscription.store';

describe('App', () => {
  const health = { check: vi.fn() };
  const subscription = { resume: vi.fn() };

  beforeEach(async () => {
    health.check.mockClear();
    subscription.resume.mockClear();
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [
        provideRouter(appRoutes),
        { provide: HealthStore, useValue: health },
        { provide: SubscriptionStore, useValue: subscription },
      ],
    }).compileComponents();
  });

  it('checks api health and resumes a saved job on init', async () => {
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    expect(health.check).toHaveBeenCalledOnce();
    expect(subscription.resume).toHaveBeenCalledOnce();
  });

  it('should render a router outlet', async () => {
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('router-outlet')).not.toBeNull();
  });
});

describe('appRoutes', () => {
  it('routes the empty path to SkyViewer', async () => {
    const [route] = appRoutes;
    expect(route.path).toBe('');
    const loadComponent = route.loadComponent as () => Promise<
      typeof SkyViewer
    >;
    await expect(loadComponent()).resolves.toBe(SkyViewer);
  });
});
