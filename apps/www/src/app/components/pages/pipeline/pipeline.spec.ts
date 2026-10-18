import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { BrnTooltip } from '@spartan-ng/brain/tooltip';
import { provideRouter, Router } from '@angular/router';
import { type Step, RunStore } from '../../../stores/run.store';
import { Pipeline } from './pipeline';
import { pipelineRoutes } from './pipeline.routes';

describe('Pipeline', () => {
  const unlocked = signal<Step>('process');

  beforeEach(async () => {
    unlocked.set('process');
    await TestBed.configureTestingModule({
      imports: [Pipeline],
      providers: [
        provideRouter(pipelineRoutes),
        {
          provide: RunStore,
          useValue: {
            unlocked,
            ready: signal(true),
            extract: signal({ status: 'running', events: [], cached: false }),
            downloadDone: signal(false),
          },
        },
      ],
    }).compileComponents();
  });

  const mount = async (url = '/download') => {
    await TestBed.inject(Router).navigateByUrl(url);
    const fixture = TestBed.createComponent(Pipeline);
    await fixture.whenStable();
    return fixture;
  };
  const render = async (url = '/download') =>
    (await mount(url)).nativeElement as HTMLElement;

  const steps = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLElement>('[data-step]'));

  it('labels the four steps', async () => {
    const el = await render();
    expect(
      steps(el).map((s) => s.textContent?.replace(/^\s*(\d+)?\s*/, '').trim()),
    ).toEqual([
      'Select star',
      'Download light curves',
      'Process data',
      'Score candidates',
    ]);
  });

  it('marks passed steps done with a check, the open one current, the rest locked', async () => {
    const el = await render('/download');
    const [select, download, process, score] = steps(el);
    expect(select.dataset['state']).toBe('done');
    expect(select.querySelector('[data-check]')).not.toBeNull();
    expect(download.dataset['state']).toBe('current');
    expect(download.getAttribute('aria-current')).toBe('step');
    expect(process.dataset['state']).toBe('open');
    expect(score.dataset['state']).toBe('locked');
    expect(score.getAttribute('aria-disabled')).toBe('true');
    expect(select.getAttribute('aria-disabled')).toBeNull();
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
    expect(score.getAttribute('aria-disabled')).toBe('true');
    expect(select.tagName).toBe('A');
    expect(select.getAttribute('href')).toBe('/select');
  });

  it('renders the routed page in an outlet', async () => {
    const el = await render();
    expect(el.querySelector('router-outlet')).not.toBeNull();
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
});
