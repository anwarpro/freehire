import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import TeamWorkspace from './TeamWorkspace.svelte';
const mocks = vi.hoisted(() => ({ teamAccess: vi.fn(), teamReviews: vi.fn(), searchJobs: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
vi.mock('$app/paths', () => ({ resolve: (path: string, params?: { slug: string }) => params ? path.replace('[slug]', params.slug) : path }));
const row = { job_id: 1, public_slug: 'python', title: 'Python Engineer', company: 'Example BD', assignee_id: 2, assignee: 'Reviewer', assignee_active: true, status: 'pending', notes: 'Private notes', follow_up: '2026-11-01', version: 3, closed_at: null, updated_at: null, overdue: true };
beforeEach(() => {
  vi.resetAllMocks();
  mocks.teamAccess.mockResolvedValue({ user_id: 2, name: 'Reviewer', role: 'reviewer' });
  mocks.teamReviews.mockResolvedValue({ data: [row], meta: { has_more: true, offset: 0, limit: 50 } });
  mocks.searchJobs.mockResolvedValue({ items: [] });
});
describe('team assignment workspace', () => {
  it('shows only the personal queue to reviewers and paginates it', async () => {
    render(TeamWorkspace); await screen.findByText('Private notes');
    expect(mocks.teamReviews).toHaveBeenCalledWith({ scope: 'mine', status: '', overdue: false, offset: 0 });
    expect(screen.queryByRole('button', { name: 'Team members' })).toBeNull();
    expect(screen.queryByRole('region', { name: 'Assign a job' })).toBeNull();
    await fireEvent.click(screen.getByRole('button', { name: 'Next' }));
    await waitFor(() => expect(mocks.teamReviews).toHaveBeenLastCalledWith(expect.objectContaining({ offset: 50 })));
  });
  it('lets managers search jobs and filter the unassigned queue', async () => {
    mocks.teamAccess.mockResolvedValue({ user_id: 1, name: 'Manager', role: 'admin' });
    mocks.searchJobs.mockResolvedValue({ items: [{ public_slug: 'new-job', title: 'New job', company: 'New BD' }] });
    render(TeamWorkspace); await screen.findByText('Private notes');
    await fireEvent.input(screen.getByLabelText('Search jobs to assign'), { target: { value: 'Python' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Find jobs' }));
    expect(await screen.findByRole('link', { name: 'New job' })).toBeTruthy();
    await fireEvent.change(screen.getByLabelText('Queue'), { target: { value: 'unassigned' } });
    await fireEvent.click(screen.getByLabelText('Overdue only'));
    await fireEvent.click(screen.getByRole('button', { name: 'Apply filters' }));
    await waitFor(() => expect(mocks.teamReviews).toHaveBeenLastCalledWith({ scope: 'unassigned', status: '', overdue: true, offset: 0 }));
  });
  it('clears private queue content when team access is revoked', async () => {
    render(TeamWorkspace); await screen.findByText('Private notes');
    mocks.teamReviews.mockRejectedValue(new ApiError(403, 'Team access is required'));
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh queue' }));
    await screen.findByRole('alert');
    expect(screen.queryByText('Private notes')).toBeNull();
    expect(screen.queryByRole('link', { name: 'Python Engineer' })).toBeNull();
  });
});
