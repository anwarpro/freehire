/** Date-only controls use Bangladesh time consistently across browsers and SSR. */
export function postingDateToday(now = new Date()): string {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Dhaka', year: 'numeric', month: '2-digit', day: '2-digit',
  }).formatToParts(now);
  const part = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? '';
  return `${part('year').padStart(4, '0')}-${part('month')}-${part('day')}`;
}

/** Empty means no replacement date; impossible or future dates must not be sent. */
export function postingDateError(value: string, now = new Date()): string | null {
  if (value === '') return null;
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || value.startsWith('0000-')) return 'Choose a valid posting date.';
  const parsed = new Date(`${value}T00:00:00Z`);
  if (!Number.isFinite(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== value) {
    return 'Choose a valid posting date.';
  }
  return value > postingDateToday(now) ? 'Posting date cannot be after today in Bangladesh.' : null;
}

/** A selected calendar day is stored at midnight UTC+06:00. Blank leaves the API field out. */
export function postingDateTimestamp(value: string, now = new Date()): string | undefined {
  const error = postingDateError(value, now);
  if (error) throw new RangeError(error);
  return value ? new Date(`${value}T00:00:00+06:00`).toISOString() : undefined;
}
