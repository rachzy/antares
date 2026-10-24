import { TestBed } from '@angular/core/testing';
import { API_BASE_URL } from '../../../api/api.config';
import { Api } from './api';

describe('Api', () => {
  const mount = async () => {
    TestBed.configureTestingModule({
      providers: [
        { provide: API_BASE_URL, useValue: 'https://antares.test/api' },
      ],
    });
    const fixture = TestBed.createComponent(Api);
    await fixture.whenStable();
    return fixture.nativeElement as HTMLElement;
  };

  it('shows the base url the page is served with and links the spec', async () => {
    const el = await mount();
    expect(el.querySelector('[data-base]')?.textContent).toBe(
      'https://antares.test/api',
    );
    const links = Array.from(el.querySelectorAll('header a')).map((a) =>
      a.getAttribute('href'),
    );
    expect(links).toEqual([
      'https://antares.test/api/docs',
      'https://antares.test/api/openapi.yaml',
    ]);
  });

  it('guides through extracting with Shaula, following the job and classifying with Fang', async () => {
    const el = await mount();
    const steps = Array.from(el.querySelectorAll('[data-step]'));
    expect(steps.map((s) => s.querySelector('h2')?.textContent)).toEqual([
      'Extract features from a star',
      'Follow the job',
      'Classify the features',
      'Or do both in one call',
    ]);
    expect(steps[0].querySelector('pre')?.textContent).toContain(
      'curl -X POST https://antares.test/api/pipeline/extract',
    );
    expect(steps[2].querySelector('pre')?.textContent).toContain(
      'https://antares.test/api/ml/predict',
    );
  });

  it('lists the endpoints with the service behind each', async () => {
    const el = await mount();
    const rows = Array.from(el.querySelectorAll('[data-endpoint]')).map((r) =>
      Array.from(r.querySelectorAll('td')).map((c) => c.textContent?.trim()),
    );
    expect(rows[0]).toEqual([
      'POST',
      '/targets/resolve',
      'Find a star by name or position',
      'Shaula',
    ]);
    expect(rows.map((r) => r[1])).toContain('/ml/runs/{run_id}/dataset');
    expect(rows.find((r) => r[1] === '/analyze')?.[3]).toBe('Both');
  });
});
