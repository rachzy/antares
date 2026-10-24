import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { BrnTooltip } from '@spartan-ng/brain/tooltip';
import { provideRouter, Router } from '@angular/router';
import { type Step, RunStore } from '../../stores/run.store';
import { Sidebar } from './sidebar';

describe('Sidebar', () => {
  const unlocked = signal<Step>('process');

  beforeEach(async () => {
    unlocked.set('process');
    await TestBed.configureTestingModule({
      imports: [Sidebar],
      providers: [
        provideRouter([{ path: '**', children: [] }]),
        {
          provide: RunStore,
          useValue: {
            unlocked,
            star: signal(null),
            extract: signal({ token: null }),
            score: signal({ token: null }),
          },
        },
      ],
    }).compileComponents();
  });

  const mount = async (url: string) => {
    await TestBed.inject(Router).navigateByUrl(url);
    const fixture = TestBed.createComponent(Sidebar);
    await fixture.whenStable();
    return fixture;
  };
  const render = async (url: string) =>
    (await mount(url)).nativeElement as HTMLElement;

  const steps = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLElement>('[data-step]'));

  it('links the sky, the classification model and the api', async () => {
    const el = await render('/');
    const hrefs = Array.from(
      el.querySelectorAll<HTMLAnchorElement>('nav[aria-label="Main"] > a'),
    ).map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/', '/model/hyperparameters', '/api']);
  });

  it('shows the pipeline under the sky only', async () => {
    expect((await render('/')).querySelector('[data-pipeline]')).not.toBeNull();
    expect(
      (await render('/download')).querySelector('[data-pipeline]'),
    ).not.toBeNull();
    expect((await render('/api')).querySelector('[data-pipeline]')).toBeNull();
    expect(
      (await render('/model/dataset')).querySelector('[data-pipeline]'),
    ).toBeNull();
  });

  it('shows the model pages under the classification model only', async () => {
    const el = await render('/model/dataset');
    const pages = Array.from(el.querySelectorAll('[data-model-pages] a'));
    expect(pages.map((a) => a.textContent?.trim())).toEqual([
      'Hyperparameters',
      'Dataset',
    ]);
    expect(pages[1].getAttribute('aria-current')).toBe('page');
    expect(pages[0].getAttribute('aria-current')).toBeNull();
    expect((await render('/')).querySelector('[data-model-pages]')).toBeNull();
  });

  it('marks the api link current on the api page', async () => {
    const el = await render('/api');
    expect(
      el.querySelector('nav > a[href="/api"]')?.getAttribute('aria-current'),
    ).toBe('page');
  });

  it('labels the four steps', async () => {
    const el = await render('/');
    expect(steps(el).map((s) => s.textContent?.trim())).toEqual([
      'Select',
      'Download',
      'Process',
      'Score',
    ]);
  });

  it('marks passed steps done, the next one unlocked, the routed one current and the rest locked', async () => {
    unlocked.set('process');
    const el = await render('/download');
    const [select, download, process, score] = steps(el);
    expect(select.dataset['state']).toBe('done');
    expect(select.querySelector('[data-check]')).not.toBeNull();
    expect(download.dataset['state']).toBe('current');
    expect(download.getAttribute('aria-current')).toBe('step');
    expect(process.dataset['state']).toBe('open');
    expect(process.querySelector('[data-unlocked]')).not.toBeNull();
    expect(score.dataset['state']).toBe('locked');
    expect(score.querySelector('[data-locked]')).not.toBeNull();
    expect(score.getAttribute('aria-disabled')).toBe('true');
    expect(select.getAttribute('aria-disabled')).toBeNull();
  });

  it('starts with only the first step open on the sky', async () => {
    unlocked.set('select');
    const el = await render('/');
    expect(steps(el).map((s) => s.dataset['state'])).toEqual([
      'open',
      'locked',
      'locked',
      'locked',
    ]);
  });

  it('shows the locked tooltip on locked steps only', async () => {
    const fixture = await mount('/download');
    const tooltips = fixture.debugElement
      .queryAll(By.css('[data-step]'))
      .map((d) => d.injector.get(BrnTooltip, null));
    expect(tooltips.map((t) => !!t && !t.tooltipDisabled())).toEqual([
      false,
      false,
      false,
      true,
    ]);
    expect(tooltips[3]?.brnTooltip()).toBe(
      'Locked until the previous step finishes',
    );
  });

  it('keeps locked steps focusable but not navigable', async () => {
    const el = await render('/download');
    const [select, , , score] = steps(el);
    expect(score.tagName).toBe('SPAN');
    expect(score.getAttribute('href')).toBeNull();
    expect(score.getAttribute('tabindex')).toBe('0');
    expect(score.getAttribute('role')).toBe('link');
    expect(select.tagName).toBe('A');
    expect(select.getAttribute('href')).toBe('/select');
  });

  it('opens and closes the menu on small screens, closing it after navigating', async () => {
    const fixture = await mount('/');
    const el = fixture.nativeElement as HTMLElement;
    const toggle = el.querySelector<HTMLButtonElement>('header button')!;
    const aside = el.querySelector('aside')!;
    expect(aside.classList.contains('hidden')).toBe(true);
    toggle.click();
    await fixture.whenStable();
    expect(toggle.getAttribute('aria-expanded')).toBe('true');
    expect(aside.classList.contains('hidden')).toBe(false);
    await TestBed.inject(Router).navigateByUrl('/api');
    await fixture.whenStable();
    expect(aside.classList.contains('hidden')).toBe(true);
  });
});
