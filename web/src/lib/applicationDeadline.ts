import { postingDateToday } from './postingDate.js';

/** Deadlines may be historical: staff also record jobs that have already expired. */
export function applicationDeadlineError(value: string): string | null {
  if (!value) return null;
  const parsed = new Date(`${value}T00:00:00Z`);
  return !/^\d{4}-\d{2}-\d{2}$/.test(value) || value.startsWith('0000-') ||
    !Number.isFinite(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== value
    ? 'Choose a valid application deadline.' : null;
}

/** The application window ends at the start of the following day in Bangladesh. */
export function applicationDeadlineTimestamp(value: string): string | undefined {
  const error = applicationDeadlineError(value);
  if (error) throw new RangeError(error);
  if (!value) return undefined;
  return new Date(Date.parse(`${value}T00:00:00+06:00`) + 86_400_000).toISOString();
}

/** Convert an exclusive timestamp back to its last application day. */
export function applicationDeadlineDate(value: string): string {
  return postingDateToday(new Date(Date.parse(value) - 1));
}
