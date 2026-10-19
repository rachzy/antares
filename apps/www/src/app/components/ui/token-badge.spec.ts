import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { RunStore } from '../../stores/run.store';
import { TokenBadge } from './token-badge';

describe('TokenBadge', () => {
  const extract = signal<{ token: string | null }>({ token: null });
  const score = signal<{ token: string | null }>({ token: null });

  const mount = async () => {
    const fixture = TestBed.createComponent(TokenBadge);
    await fixture.whenStable();
    return fixture.nativeElement as HTMLElement;
  };

  beforeEach(() => {
    extract.set({ token: null });
    score.set({ token: null });
    TestBed.configureTestingModule({
      providers: [{ provide: RunStore, useValue: { extract, score } }],
    });
  });

  it('renders nothing without a token', async () => {
    expect((await mount()).textContent?.trim()).toBe('');
  });

  it('shows the extraction token, shortened, with the full value on hover', async () => {
    extract.set({ token: 'qRx6ZbxbLsNhepHlSSNUrCjyWtRjFs-YAWnNJnsqfEY' });
    const el = await mount();
    const box = el.querySelector<HTMLElement>('[data-token]')!;
    expect(box.textContent).toContain('qRx6Zbxb');
    expect(box.textContent).not.toContain('YAWnNJnsqfEY');
    expect(box.title).toBe('qRx6ZbxbLsNhepHlSSNUrCjyWtRjFs-YAWnNJnsqfEY');
  });

  it('copies the full token from the copy button', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { clipboard: { writeText } });
    extract.set({ token: 'qRx6ZbxbLsNhepHlSSNUrCjyWtRjFs-YAWnNJnsqfEY' });
    const el = await mount();
    const button = el.querySelector<HTMLButtonElement>(
      'an-copy-button button',
    )!;
    expect(button.getAttribute('aria-label')).toBe('Copy token');
    button.click();
    expect(writeText).toHaveBeenCalledWith(
      'qRx6ZbxbLsNhepHlSSNUrCjyWtRjFs-YAWnNJnsqfEY',
    );
    vi.unstubAllGlobals();
  });

  it('prefers the scoring token once scoring started', async () => {
    extract.set({ token: 'extract-token-aaaaaaaa' });
    score.set({ token: 'score-token-bbbbbbbbbb' });
    const box = (await mount()).querySelector<HTMLElement>('[data-token]')!;
    expect(box.title).toBe('score-token-bbbbbbbbbb');
  });
});
