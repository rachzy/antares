import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RunStore } from '../../../stores/run.store';
import { Shell } from './shell';

describe('Shell', () => {
  it('shows the sidebar next to the routed page', async () => {
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        {
          provide: RunStore,
          useValue: {
            unlocked: signal('select'),
            star: signal(null),
            extract: signal({ token: null }),
            score: signal({ token: null }),
          },
        },
      ],
    });
    const fixture = TestBed.createComponent(Shell);
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('an-sidebar')).not.toBeNull();
    expect(el.querySelector('main router-outlet')).not.toBeNull();
  });
});
