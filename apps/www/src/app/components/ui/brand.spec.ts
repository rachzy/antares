import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { Brand, BrandState } from './brand';

describe('Brand', () => {
  beforeEach(() =>
    TestBed.configureTestingModule({
      providers: [
        provideRouter([
          { path: '', children: [] },
          { path: 'select', children: [] },
        ]),
      ],
    }),
  );

  const mount = async (url: string) => {
    await TestBed.inject(Router).navigateByUrl(url);
    const fixture = TestBed.createComponent(Brand);
    await fixture.whenStable();
    const link = () =>
      (fixture.nativeElement as HTMLElement).querySelector<HTMLAnchorElement>(
        'a',
      )!;
    return { fixture, link };
  };

  it('shows the name large on the welcome page', async () => {
    const { link } = await mount('/');
    expect(link().textContent).toBe('Antares');
    expect(link().getAttribute('href')).toBe('/');
    expect(link().dataset['compact']).toBe('false');
  });

  it('shrinks into the corner on every other page', async () => {
    const { link } = await mount('/select');
    expect(link().dataset['compact']).toBe('true');
  });

  it('moves up as soon as the welcome page starts leaving', async () => {
    const { fixture, link } = await mount('/');
    TestBed.inject(BrandState).leaving.set(true);
    await fixture.whenStable();
    expect(link().dataset['compact']).toBe('true');
  });

  it('grows back when the user returns to the welcome page', async () => {
    const { fixture, link } = await mount('/select');
    await TestBed.inject(Router).navigateByUrl('/');
    await fixture.whenStable();
    expect(link().dataset['compact']).toBe('false');
  });
});
