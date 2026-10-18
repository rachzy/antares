import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { App } from './app';
import { appRoutes } from './app.routes';
import { Welcome } from './components/pages/welcome/welcome';
import { HealthStore } from './stores/health.store';
import { RunStore } from './stores/run.store';

describe('App', () => {
  const health = { check: vi.fn() };
  const run = { restore: vi.fn() };

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
    }).compileComponents();
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
  it('routes the empty path to Welcome', async () => {
    const [route] = appRoutes;
    expect(route.path).toBe('');
    const loadComponent = route.loadComponent as () => Promise<typeof Welcome>;
    await expect(loadComponent()).resolves.toBe(Welcome);
  });
});
