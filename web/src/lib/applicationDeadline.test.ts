import { describe, expect, it } from 'vitest';
import { applicationDeadlineDate, applicationDeadlineError, applicationDeadlineTimestamp } from './applicationDeadline.js';

describe('application deadlines', () => {
  it('ends a Bangladesh calendar day at the next midnight, including year rollover', () => {
    expect(applicationDeadlineTimestamp('2026-12-31')).toBe('2026-12-31T18:00:00.000Z');
    expect(applicationDeadlineDate('2026-12-31T18:00:00Z')).toBe('2026-12-31');
    expect(applicationDeadlineTimestamp('2028-02-29')).toBe('2028-02-29T18:00:00.000Z');
  });
  it('supports unknown and historical deadlines', () => {
    expect(applicationDeadlineTimestamp('')).toBeUndefined();
    expect(applicationDeadlineTimestamp('2020-01-01')).toBe('2020-01-01T18:00:00.000Z');
  });
  it.each(['2026-02-29', '2026-04-31', 'invalid', '0000-01-01'])('rejects impossible dates: %s', (value) => {
    expect(applicationDeadlineError(value)).toBeTruthy();
    expect(() => applicationDeadlineTimestamp(value)).toThrow(RangeError);
  });
});
