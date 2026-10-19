import { TestBed } from '@angular/core/testing';
import { FeaturesPreview } from './features-preview';

describe('FeaturesPreview', () => {
  const mount = async (rows: Record<string, unknown>[]) => {
    const fixture = TestBed.createComponent(FeaturesPreview);
    fixture.componentRef.setInput('rows', rows);
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    const cells = (sel: string) =>
      Array.from(el.querySelectorAll(sel)).map((c) => c.textContent?.trim());
    return { el, cells };
  };

  it('shows every column across all rows, in first-seen order', async () => {
    const { cells } = await mount([
      { a: 1, b: 2 },
      { b: 3, c: 4 },
    ]);
    expect(cells('thead th')).toEqual(['a', 'b', 'c']);
    expect(cells('tbody tr:nth-child(1) td')).toEqual(['1', '2', '—']);
    expect(cells('tbody tr:nth-child(2) td')).toEqual(['—', '3', '4']);
  });

  it('shortens long decimals and keeps text and nulls readable', async () => {
    const { cells } = await mount([
      { x: 0.123456789, y: 12, z: 'KIC 1', n: null, o: { k: 1 } },
    ]);
    expect(cells('tbody td')).toEqual([
      '0.123457',
      '12',
      'KIC 1',
      '—',
      '{"k":1}',
    ]);
  });

  it('counts the rows', async () => {
    const { el } = await mount([{ a: 1 }, { a: 2 }]);
    expect(el.textContent).toContain('2 rows');
  });

  it('uses the singular for one row', async () => {
    const { el } = await mount([{ a: 1 }]);
    expect(el.textContent).toContain('1 row');
    expect(el.textContent).not.toContain('1 rows');
  });
});
