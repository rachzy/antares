import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { RunStore } from '../../stores/run.store';
import { TargetStore } from '../../stores/target.store';
import { ResetButton } from './reset-button';

describe('ResetButton', () => {
  const star = signal<object | null>(null);
  const reset = vi.fn();
  const resetTarget = vi.fn();

  beforeEach(() => {
    star.set(null);
    reset.mockClear();
    resetTarget.mockClear();
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'select', children: [] }]),
        { provide: RunStore, useValue: { star, reset } },
        { provide: TargetStore, useValue: { reset: resetTarget } },
      ],
    });
  });

  const mount = async () => {
    const fixture = TestBed.createComponent(ResetButton);
    await fixture.whenStable();
    return fixture;
  };

  it('is hidden until a star is selected', async () => {
    const fixture = await mount();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('button')).toBeNull();
    star.set({ display_name: 'Kepler-186' });
    await fixture.whenStable();
    expect(el.querySelector('button')?.getAttribute('aria-label')).toBe(
      'Reset',
    );
  });

  it('clears everything and goes back to the first step', async () => {
    star.set({ display_name: 'Kepler-186' });
    const navigate = vi
      .spyOn(TestBed.inject(Router), 'navigateByUrl')
      .mockResolvedValue(true);
    const fixture = await mount();
    (fixture.nativeElement as HTMLElement)
      .querySelector<HTMLButtonElement>('button')!
      .click();
    expect(reset).toHaveBeenCalledOnce();
    expect(resetTarget).toHaveBeenCalledOnce();
    expect(navigate).toHaveBeenCalledWith('/select');
  });
});
