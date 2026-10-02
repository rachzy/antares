import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { App } from './app';
import { appRoutes } from './app.routes';
import { Welcome } from './components/pages/welcome/welcome';

describe('App', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [provideRouter(appRoutes)],
    }).compileComponents();
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
