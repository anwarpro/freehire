import { describe, expect, it } from 'vitest';
import { postingDateError, postingDateTimestamp, postingDateToday } from './postingDate';

const now = new Date('2026-11-05T10:00:00Z');

describe('posting date controls', () => {
  it('uses the Bangladesh calendar day around UTC midnight boundaries', () => {
    expect(postingDateToday(new Date('2026-11-04T17:59:59Z'))).toBe('2026-11-04');
    expect(postingDateToday(new Date('2026-11-04T18:00:00Z'))).toBe('2026-11-05');
  });

  it('leaves an unknown date out of the request instead of inventing one', () => {
    expect(postingDateTimestamp('', now)).toBeUndefined();
    expect(postingDateError('', now)).toBeNull();
  });

  it('stores the selected calendar date at Bangladesh midnight', () => {
    expect(postingDateTimestamp('2026-11-05', now)).toBe('2026-11-04T18:00:00.000Z');
    expect(postingDateTimestamp('2024-02-29', now)).toBe('2024-02-28T18:00:00.000Z');
  });

  it.each(['2026-02-29', '2026-04-31', '2026-13-01', '2026-1-01', 'not-a-date', '0000-01-01'])('rejects invalid date %s', (value) => {
    expect(postingDateError(value, now)).toBe('Choose a valid posting date.');
    expect(() => postingDateTimestamp(value, now)).toThrow(RangeError);
  });

  it('refuses a future calendar day but accepts today in Bangladesh', () => {
    expect(postingDateError('2026-11-06', now)).toContain('after today');
    expect(postingDateError('2026-11-05', now)).toBeNull();
    expect(() => postingDateTimestamp('2026-11-06', now)).toThrow(RangeError);
  });
});
