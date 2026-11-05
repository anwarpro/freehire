import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import CurriculumWorkspace from './CurriculumWorkspace.svelte';
const mocks = vi.hoisted(() => ({ curriculumSettings: vi.fn(), curriculumJobs: vi.fn(), listSources: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
vi.mock('$app/paths', () => ({ resolve: (path: string, params?: { slug: string }) => params ? path.replace('[slug]', params.slug) : path }));
const settings = { version: 2, config: { tracks: [{ id: 'python', label: 'Python engineering', enabled: true, minimum: 1, skills: [] }] }, progress: { mapping_version: 2, cursor_id: 100, upper_id: 100, complete: true, processed: 100, last_run_at: '2026-11-06T01:00:00Z', last_error: '' } };
const row = { job_id: 1, public_slug: 'python-job', title: 'Python Engineer', company: 'Example', location: 'Dhaka', source: 'linkedin', closed_at: null, suggested_tracks: ['python'], reviewed_tracks: null, effective_tracks: ['python'], reviewed: false, review_stale: false, update_pending: false };
beforeEach(() => { vi.resetAllMocks(); mocks.curriculumSettings.mockResolvedValue(settings); mocks.curriculumJobs.mockResolvedValue({ data: [row], meta: { has_more: true, offset: 0, limit: 50 } }); mocks.listSources.mockResolvedValue([{ source: 'linkedin' }, { source: 'bdjobs' }]); });
describe('curriculum browsing', () => {
  it('filters projected jobs and resets pagination when filters change', async () => {
    render(CurriculumWorkspace, { role: 'admin' }); await screen.findByRole('link', { name: 'Python Engineer' });
    expect(screen.getByText('Suggested · unreviewed')).toBeTruthy();
    await fireEvent.click(screen.getByRole('button', { name: 'Next curriculum page' }));
    await waitFor(() => expect(mocks.curriculumJobs).toHaveBeenLastCalledWith({ track: '', source: '', state: 'active', review: '', offset: 50 }));
    await fireEvent.change(screen.getByLabelText('Curriculum track'), { target: { value: 'python' } });
    await fireEvent.change(screen.getByLabelText('Curriculum review'), { target: { value: 'needs_review' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Filter curriculum jobs' }));
    await waitFor(() => expect(mocks.curriculumJobs).toHaveBeenLastCalledWith({ track: 'python', source: '', state: 'active', review: 'needs_review', offset: 0 }));
  });
  it('keeps reviewers scoped to assignments without mapping controls', async () => {
    render(CurriculumWorkspace, { role: 'reviewer' }); await screen.findByRole('link', { name: 'Python Engineer' });
    expect(screen.getByText(/Curriculum matches for jobs assigned to you/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Edit curriculum mapping' })).toBeNull();
  });
  it('clears team content after access revocation', async () => {
    render(CurriculumWorkspace, { role: 'admin' }); await screen.findByRole('link', { name: 'Python Engineer' });
    mocks.curriculumJobs.mockRejectedValue(new ApiError(403, 'Team access is required'));
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh curriculum' })); await screen.findByRole('alert');
    expect(screen.queryByRole('link', { name: 'Python Engineer' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Edit curriculum mapping' })).toBeNull();
  });
  it('describes pending backfill instead of claiming no catalogue matches', async () => {
    mocks.curriculumSettings.mockResolvedValue({ ...settings, progress: { ...settings.progress, complete: false, processed: 0, last_run_at: null } });
    mocks.curriculumJobs.mockResolvedValue({ data: [], meta: { has_more: false, offset: 0, limit: 50 } });
    render(CurriculumWorkspace, { role: 'admin' }); await screen.findByText(/Matching is still in progress/);
    expect(screen.getByText(/Waiting for the matching worker/)).toBeTruthy();
  });
});
