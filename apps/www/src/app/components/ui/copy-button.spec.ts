import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { CopyButton } from './copy-button';

@Component({
  imports: [CopyButton],
  template: '<an-copy-button value="abc-123" label="Copy token" />',
})
class Host {}

describe('CopyButton', () => {
  const writeText = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    writeText.mockClear();
    vi.stubGlobal('navigator', { clipboard: { writeText } });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  const mount = async () => {
    const fixture = TestBed.createComponent(Host);
    await fixture.whenStable();
    const button = (fixture.nativeElement as HTMLElement).querySelector(
      'button',
    )!;
    return { fixture, button };
  };

  it('is a labelled button with the copy icon', async () => {
    const { button } = await mount();
    expect(button.getAttribute('aria-label')).toBe('Copy token');
    expect(button.querySelector('[data-icon="copy"]')).not.toBeNull();
  });

  it('copies the value and shows a check for a moment', async () => {
    const { fixture, button } = await mount();
    vi.useFakeTimers();
    button.click();
    await vi.advanceTimersByTimeAsync(0);
    fixture.detectChanges();
    expect(writeText).toHaveBeenCalledWith('abc-123');
    expect(button.querySelector('[data-icon="check"]')).not.toBeNull();
    expect(button.getAttribute('aria-label')).toBe('Copied');

    await vi.advanceTimersByTimeAsync(2000);
    fixture.detectChanges();
    expect(button.querySelector('[data-icon="copy"]')).not.toBeNull();
  });

  it('stays quiet when the clipboard is blocked', async () => {
    writeText.mockRejectedValueOnce(new Error('denied'));
    const { fixture, button } = await mount();
    button.click();
    await fixture.whenStable();
    expect(button.querySelector('[data-icon="copy"]')).not.toBeNull();
  });
});
