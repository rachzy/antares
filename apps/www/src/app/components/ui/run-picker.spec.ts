import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import type { Run } from '../../api/api.types';
import { ModelStore } from '../../stores/model.store';
import { RunPicker } from './run-picker';

const run = (run_id: string): Run => ({
  run_id,
  created_at: '2026-01-01T00:00:00Z',
  schema_version: 2,
  selected_model: 'lightgbm',
  threshold: 0.5,
});

describe('RunPicker', () => {
  const runs = signal([run('b'), run('a')]);
  const runId = signal<string | null>('b');
  const selectRun = vi.fn();

  const mount = async () => {
    TestBed.configureTestingModule({
      providers: [
        { provide: ModelStore, useValue: { runs, runId, selectRun } },
      ],
    });
    const fixture = TestBed.createComponent(RunPicker);
    await fixture.whenStable();
    return fixture.nativeElement as HTMLElement;
  };

  beforeEach(() => selectRun.mockClear());

  it('lists the runs newest first and marks the newest', async () => {
    const options = Array.from((await mount()).querySelectorAll('option')).map(
      (o) => o.textContent?.trim(),
    );
    expect(options).toEqual(['b (newest)', 'a']);
  });

  it('selects the chosen run', async () => {
    const select = (await mount()).querySelector('select')!;
    select.value = 'a';
    select.dispatchEvent(new Event('change'));
    expect(selectRun).toHaveBeenCalledWith('a');
  });
});
