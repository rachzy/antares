import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
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
        { provide: RunStore, useValue: { unlocked } },
      ],
    }).compileComponents();
  });

  const render = async (url = '/download') => {
    await TestBed.inject(Router).navigateByUrl(url);
    const fixture = TestBed.createComponent(Pipeline);
    await fixture.whenStable();
    return fixture.nativeElement as HTMLElement;
  };

  const steps = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLElement>('[data-step]'));

  it('labels the four steps', async () => {
    const el = await render();
    expect(steps(el).map((s) => s.textContent?.trim())).toEqual([
      expect.stringContaining('Select star'),
      expect.stringContaining('Download light curves'),
      expect.stringContaining('Process data'),
      expect.stringContaining('Score candidates'),
    ]);
  });

  it('marks passed steps done with a check, the open one current, the rest locked', async () => {
    const el = await render('/download');
    const [select, download, process, score] = steps(el);
    expect(select.dataset['state']).toBe('done');
    expect(select.querySelector('svg, [data-check]')).not.toBeNull();
    expect(download.dataset['state']).toBe('current');
    expect(download.getAttribute('aria-current')).toBe('step');
    expect(process.dataset['state']).toBe('open');
    expect(score.dataset['state']).toBe('locked');
    expect(score.getAttribute('aria-disabled')).toBe('true');
    expect(select.getAttribute('aria-disabled')).toBeNull();
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
