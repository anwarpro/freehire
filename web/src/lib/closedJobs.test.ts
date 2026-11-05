import { describe, expect, it } from 'vitest';
import { jobStatusQuery, originalPostingUrl } from './closedJobs.js';

describe('job status navigation', () => {
  it('carries supported filters and resets page/cursor and active-only filters', () => {
    const result = new URLSearchParams(jobStatusQuery('source=bdjobs&source=greenhouse&skills=python&skills_exclude=go&skills_mode=and&countries=bd&page=4&before=old&q=engineer&sort=match'));
    expect(result.getAll('source')).toEqual(['bdjobs','greenhouse']);
    expect(result.get('skills')).toBe('python');
    expect(result.get('skills_exclude')).toBe('go');
    expect(result.get('skills_mode')).toBe('and');
    expect(result.get('countries')).toBe('bd');
    for (const key of ['page','before','q','sort']) expect(result.has(key)).toBe(false);
  });
  it('encodes the next cursor while preserving exclusions', () => {
    const result = new URLSearchParams(jobStatusQuery('source_exclude=linkedin&countries_exclude=us','cursor+value'));
    expect(result.get('before')).toBe('cursor+value');
    expect(result.get('source_exclude')).toBe('linkedin');
    expect(jobStatusQuery('')).toBe('');
  });
  it('opens only http(s) original-posting links', () => {
    expect(originalPostingUrl('https://example.test/job')).toBe('https://example.test/job');
    for (const value of ['javascript:alert(1)','data:text/html,test','/relative','']) expect(originalPostingUrl(value)).toBeNull();
  });
});
