import { TestBed } from '@angular/core/testing';
import { ModeSwitch } from './mode-switch';

describe('ModeSwitch', () => {
  const mount = () => {
    const fixture = TestBed.createComponent(ModeSwitch);
    const el = fixture.nativeElement as HTMLElement;
    const pressed = (mode: string) =>
      el.querySelector(`[data-mode="${mode}"]`)?.getAttribute('aria-pressed');
    return { fixture, el, pressed };
  };

  it('starts on Aladin', async () => {
    const { fixture, pressed } = mount();
    await fixture.whenStable();
    expect(pressed('aladin')).toBe('true');
    expect(pressed('manual')).toBe('false');
  });

  it('switches to manual select and back', async () => {
    const { fixture, el, pressed } = mount();
    await fixture.whenStable();
    el.querySelector<HTMLButtonElement>('[data-mode="manual"]')!.click();
    await fixture.whenStable();
    expect(fixture.componentInstance.mode()).toBe('manual');
    expect(pressed('manual')).toBe('true');
    el.querySelector<HTMLButtonElement>('[data-mode="aladin"]')!.click();
    await fixture.whenStable();
    expect(fixture.componentInstance.mode()).toBe('aladin');
  });
});
