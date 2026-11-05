import { render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import JobStatusTabs from './JobStatusTabs.svelte';
vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

describe('job status tabs', () => {
  it('links active browsing to closed jobs with shared filters', () => {
    render(JobStatusTabs, { params:'source=bdjobs&skills=python&page=3' });
    expect(screen.getByRole('link',{name:'Active jobs'}).getAttribute('aria-current')).toBe('page');
    expect(screen.getByRole('link',{name:'Closed / expired jobs'}).getAttribute('href')).toBe('/jobs/closed?source=bdjobs&skills=python');
  });
  it('marks the archive current and returns to active jobs without a cursor', () => {
    render(JobStatusTabs,{closed:true,params:'countries=bd&before=cursor'});
    expect(screen.getByRole('link',{name:'Closed / expired jobs'}).getAttribute('aria-current')).toBe('page');
    expect(screen.getByRole('link',{name:'Active jobs'}).getAttribute('href')).toBe('/jobs?countries=bd');
  });
});
