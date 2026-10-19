import { TestBed } from '@angular/core/testing';
import type { PredictionRow } from '../../api/api.types';
import { rank } from '../../utils/scoring';
import { ScoreRanking } from './score-ranking';

const row = (period: number, p: number, mes: number): PredictionRow => ({
  period_days: period,
  MES: mes,
  potential_probability: p,
  decision_threshold: 0.5,
  prediction: p >= 0.5 ? 'POTENTIAL' : 'UNLIKELY',
});

describe('ScoreRanking', () => {
  const mount = async (rows: PredictionRow[]) => {
    const fixture = TestBed.createComponent(ScoreRanking);
    fixture.componentRef.setInput('rows', rank(rows));
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    return { el, body: () => Array.from(el.querySelectorAll('tbody tr')) };
  };

  it('lists the candidates by probability with period and depth', async () => {
    const { body } = await mount([
      row(4.82, 0.44, 9.1),
      row(129.9, 0.71, 12.5),
    ]);
    const text = body().map((r) => r.textContent?.replace(/\s+/g, ' '));
    expect(text[0]).toContain('129.9 d');
    expect(text[0]).toContain('12.5');
    expect(text[1]).toContain('4.82 d');
  });

  it('shows a bar with a threshold tick and the decision tag', async () => {
    const { body } = await mount([
      row(129.9, 0.71, 12.5),
      row(4.82, 0.44, 9.1),
    ]);
    const [top, low] = body();
    const fill = top.querySelector<HTMLElement>('[data-fill]');
    const tick = top.querySelector<HTMLElement>('[data-tick]');
    expect(fill?.style.width).toBe('71%');
    expect(tick?.style.left).toBe('50%');
    expect(top.querySelector('[data-tag]')?.textContent).toContain('POTENTIAL');
    expect(top.querySelector('[data-tag]')?.textContent).toContain('71%');
    expect(low.querySelector('[data-tag]')?.textContent).toContain('UNLIKELY');
  });

  it('shows a dash for a missing probability', async () => {
    const { body } = await mount([{ period_days: 3 }]);
    const fill = body()[0].querySelector<HTMLElement>('[data-fill]');
    expect(fill?.style.width).toBe('0%');
    expect(body()[0].querySelector('[data-tag]')?.textContent).toContain('—');
  });
});
