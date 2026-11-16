import { describe, expect, it } from 'vitest';
import { emptyFilters, filtersFromParams, filtersToParams } from './facetModel';
import { fresherFilters, isFresherFilters, isFresherJob } from './fresher';

describe('Fresher jobs', () => {
  it('preserves other filters and round trips through shared URLs and alert queries', () => {
    const original = filtersFromParams(new URLSearchParams('countries=bd&skills=react&source=bdjobs&q=developer'));
    const enabled = fresherFilters(original, true);
    const params = filtersToParams(enabled);
    expect(params.get('countries')).toBe('bd');
    expect(params.get('skills')).toBe('react');
    expect(params.get('source')).toBe('bdjobs');
    expect(params.get('q')).toBe('developer');
    expect(isFresherFilters(filtersFromParams(params))).toBe(true);
    expect(original.experienceYearsMax).toBeNull();
    const disabled = fresherFilters(enabled, false);
    expect(isFresherFilters(disabled)).toBe(false);
    expect(filtersToParams(disabled).toString()).toBe(filtersToParams(original).toString());
  });

  it('does not claim a custom seniority or experience selection is the preset', () => {
    const enabled = fresherFilters(emptyFilters(), true);
    expect(isFresherFilters({ ...enabled, experienceYearsMax: 3 })).toBe(false);
    expect(isFresherFilters({ ...enabled, seniorityIncludeUnspecified: true })).toBe(false);
    expect(isFresherFilters(emptyFilters())).toBe(false);
  });

  it.each([
    ['intern', undefined, true], ['junior', 0, true], ['junior', 1, true],
    ['junior', undefined, true], ['junior', 2, false], ['intern', 3, false],
    ['senior', 0, false], [undefined, 0, false], [undefined, undefined, false],
  ])('labels %s with experience %s as fresher: %s', (seniority, years, expected) => {
    expect(isFresherJob({ seniority, experience_years_min: years })).toBe(expected);
  });
});
