import { TestBed } from '@angular/core/testing';
import { ManualSelect } from './manual-select';

describe('ManualSelect', () => {
  const mount = async () => {
    const fixture = TestBed.createComponent(ManualSelect);
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const lookups: unknown[] = [];
    fixture.componentInstance.lookup.subscribe((q) => lookups.push(q));
    const fill = (id: string, value: string) => {
      const input = el.querySelector<HTMLInputElement>(`#manual-${id}`)!;
      input.value = value;
      input.dispatchEvent(new Event('input'));
    };
    const submit = async () => {
      el.querySelector('form')!.dispatchEvent(new Event('submit'));
      await fixture.whenStable();
    };
    return { el, lookups, fill, submit };
  };

  it('emits the entered name', async () => {
    const { lookups, fill, submit } = await mount();
    fill('name', 'Kepler-90');
    await submit();
    expect(lookups).toEqual([{ query: 'Kepler-90' }]);
  });

  it('emits the coordinates in degrees', async () => {
    const { lookups, fill, submit } = await mount();
    fill('ra', '298.65');
    fill('dec', '43.95');
    await submit();
    expect(lookups).toEqual([{ ra: 298.65, dec: 43.95 }]);
  });

  it('shows what is wrong and emits nothing', async () => {
    const { el, lookups, fill, submit } = await mount();
    fill('ra', '10');
    await submit();
    expect(lookups).toEqual([]);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Enter both RA and Dec.',
    );
  });

  it('clears the error once the entry is valid', async () => {
    const { el, fill, submit } = await mount();
    await submit();
    fill('name', 'Kepler-90');
    await submit();
    expect(el.querySelector('[role="alert"]')).toBeNull();
  });
});
