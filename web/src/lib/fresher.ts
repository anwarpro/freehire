import { emptyFacet, type JobFilters } from './facetModel';
import type { Enrichment } from './types';

const levels = ['intern', 'junior'];
export const fresherDescription = 'Intern and entry-level jobs asking for up to 1 year of experience. Includes roles that do not state experience; check the original posting.';

/** A preset over existing search facets, so saved searches and alerts share its rules. */
export function fresherFilters(filters: JobFilters, enabled: boolean): JobFilters {
  return {
    ...filters,
    facets: { ...filters.facets, seniority: { ...emptyFacet(), include: enabled ? [...levels] : [] } },
    experienceYearsMax: enabled ? 1 : null,
    experienceIncludeUnspecified: enabled,
    experienceOnlySeniorityUnspecified: false,
    seniorityIncludeUnspecified: false,
  };
}

export function isFresherFilters(filters: JobFilters): boolean {
  const selected = filters.facets.seniority;
  return selected?.include.length === 2 && levels.every((level) => selected.include.includes(level))
    && selected.exclude.length === 0 && !selected.matchAll
    && filters.experienceYearsMax === 1 && filters.experienceIncludeUnspecified
    && !filters.experienceOnlySeniorityUnspecified && !filters.seniorityIncludeUnspecified;
}

export function isFresherJob(e: Pick<Enrichment, 'seniority' | 'experience_years_min'>): boolean {
  return levels.includes(e.seniority ?? '') && (e.experience_years_min == null || e.experience_years_min <= 1);
}
