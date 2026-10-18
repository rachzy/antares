import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { toast } from '@spartan-ng/brain/sonner';
import { ApiRequestError } from '../../api/api.service';
import { HealthStore } from '../../stores/health.store';
import { MlStore } from '../../stores/ml.store';
import { PipelineStore } from '../../stores/pipeline.store';
import { RunStore } from '../../stores/run.store';
import { TargetStore } from '../../stores/target.store';
import { ErrorToaster } from './error-toaster';

vi.mock('@spartan-ng/brain/sonner', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  toast: { error: vi.fn() },
}));

describe('ErrorToaster', () => {
  const target = signal<ApiRequestError | null>(null);
  const pipeline = signal<ApiRequestError | null>(null);
  const extract = signal({ error: null as ApiRequestError | null });

  async function setup() {
    TestBed.configureTestingModule({
      providers: [
        { provide: TargetStore, useValue: { error: target } },
        {
          provide: PipelineStore,
          useValue: { error: pipeline, retryAfterSeconds: signal(30) },
        },
        { provide: HealthStore, useValue: { error: signal(null) } },
        { provide: MlStore, useValue: { error: signal(null) } },
        {
          provide: RunStore,
          useValue: { extract, score: signal({ error: null }) },
        },
      ],
    });
    const fixture = TestBed.createComponent(ErrorToaster);
    await fixture.whenStable();
    return fixture;
  }

  beforeEach(() => {
    vi.stubGlobal('matchMedia', () => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }));
    vi.mocked(toast.error).mockClear();
    target.set(null);
    pipeline.set(null);
    extract.set({ error: null });
  });

  it('toasts a new error once', async () => {
    const fixture = await setup();
    const error = new ApiRequestError(404, 'not_found', 'Not found');
    target.set(error);
    await fixture.whenStable();
    expect(toast.error).toHaveBeenCalledExactlyOnceWith('Not found');
    target.set(null);
    await fixture.whenStable();
    target.set(error);
    await fixture.whenStable();
    expect(toast.error).toHaveBeenCalledTimes(1);
  });

  it('appends the retry hint for busy errors', async () => {
    const fixture = await setup();
    target.set(new ApiRequestError(429, 'busy', 'Busy.', 7));
    pipeline.set(new ApiRequestError(429, 'busy', 'Busy.'));
    await fixture.whenStable();
    expect(toast.error).toHaveBeenCalledWith('Busy. Try again in 7 s.');
    expect(toast.error).toHaveBeenCalledWith('Busy. Try again in 30 s.');
  });

  it('toasts an error shared by several sources once', async () => {
    const fixture = await setup();
    const error = new ApiRequestError(500, 'internal', 'Boom');
    pipeline.set(error);
    extract.set({ error });
    await fixture.whenStable();
    expect(toast.error).toHaveBeenCalledTimes(1);
  });
});
