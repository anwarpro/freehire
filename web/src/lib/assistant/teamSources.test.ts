import { describe, expect, it } from 'vitest';
import { teamJobSources } from './teamSources';
describe('verified team job sources', () => {
  it('extracts canonical/original links from successful tools, not model prose', () => {
    const result = JSON.stringify({ job: { title: 'React engineer', closed_at: '2026-11-01' }, job_link: 'https://jobs.test/jobs/react', source_url: 'https://company.test/jobs/1' });
    expect(teamJobSources([{ name: 'get_job', input: {}, result }])).toEqual([{ title: 'React engineer', closed: true, jobLink: 'https://jobs.test/jobs/react', sourceURL: 'https://company.test/jobs/1' }]);
  });
  it('drops malformed, failed and executable source links', () => {
    const result = JSON.stringify({ job: { title: 'Fake' }, job_link: 'javascript:alert(1)' });
    expect(teamJobSources([{ name: 'get_job', input: {}, result }, { name: 'get_job', input: {}, result: '{broken' }, { name: 'get_job', input: {}, result: '{}', isError: true }])).toEqual([]);
  });
  it('deduplicates sources and accepts same-site job paths', () => {
    const item = { posting: { title: 'Python' }, job_link: '/jobs/python', source_url: 'data:text/html,hi' };
    const result = JSON.stringify({ jobs: [item, item] });
    expect(teamJobSources([{ name: 'course_jobs', input: {}, result }])).toEqual([{ title: 'Python', closed: false, jobLink: '/jobs/python', sourceURL: '' }]);
  });
});
