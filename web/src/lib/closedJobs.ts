/** Only shared archive filters travel between active and closed browsing. */
const sharedFilters = ['source', 'source_exclude', 'skills', 'skills_exclude', 'skills_mode', 'countries', 'countries_exclude'];

export function jobStatusQuery(search: string, before?: string): '' | `?${string}` {
  const input = new URLSearchParams(search);
  const output = new URLSearchParams();
  for (const key of sharedFilters) {
    for (const value of input.getAll(key)) if (value.trim()) output.append(key, value);
  }
  if (before) output.set('before', before);
  const query = output.toString();
  return query ? `?${query}` : '';
}

export function originalPostingUrl(value: string): string | null {
  try {
    const url = new URL(value);
    return url.protocol === 'https:' || url.protocol === 'http:' ? url.href : null;
  } catch { return null; }
}
