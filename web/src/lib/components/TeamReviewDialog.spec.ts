import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import TeamReviewDialog from './TeamReviewDialog.svelte';
const mocks = vi.hoisted(() => ({ teamAccess: vi.fn(), teamReview: vi.fn(), teamMembers: vi.fn(), saveTeamReview: vi.fn(), teamReviewHistory: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
const review = { job_id: 1, public_slug: 'python', title: 'Python Engineer', company: 'Example BD', assignee_id: 2, assignee: 'Reviewer', assignee_active: true, status: 'pending', notes: 'Original notes', follow_up: null, version: 3, closed_at: null, updated_at: null, overdue: false };
beforeEach(() => {
  vi.resetAllMocks();
  mocks.teamAccess.mockResolvedValue({ user_id: 1, name: 'Manager', role: 'admin' });
  mocks.teamReview.mockResolvedValue(review);
  mocks.teamMembers.mockResolvedValue([{ id: 2, name: 'Reviewer', active: true }]);
  mocks.saveTeamReview.mockResolvedValue({ ...review, version: 4 });
  mocks.teamReviewHistory.mockResolvedValue([]);
});
function mount() { const onSaved = vi.fn(); render(TeamReviewDialog, { slug: 'python', onClose: vi.fn(), onSaved }); return onSaved; }
describe('private job review editor', () => {
  it('saves the selected reviewer, notes and calendar date with the loaded version', async () => {
    const saved = mount();
    await screen.findByLabelText('Review notes');
    await fireEvent.input(screen.getByLabelText('Review notes'), { target: { value: 'Contact the student' } });
    await fireEvent.change(screen.getByLabelText('Review status'), { target: { value: 'in_review' } });
    await fireEvent.input(screen.getByLabelText('Follow-up date (optional)'), { target: { value: '2026-12-01' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    await waitFor(() => expect(mocks.saveTeamReview).toHaveBeenCalledWith('python', { assignee_id: 2, status: 'in_review', notes: 'Contact the student', follow_up: '2026-12-01', expected_version: 3 }));
    expect(saved).toHaveBeenCalledOnce();
  });
  it('keeps a conflicting draft and requires a fresh load before saving', async () => {
    mocks.saveTeamReview.mockRejectedValue(new ApiError(409, 'Changed elsewhere'));
    mount(); await screen.findByLabelText('Review notes');
    await fireEvent.input(screen.getByLabelText('Review notes'), { target: { value: 'My draft' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    expect((await screen.findByRole('alert')).textContent).toContain('draft is kept');
    expect((screen.getByLabelText('Review notes') as HTMLTextAreaElement).value).toBe('My draft');
    expect(screen.getByRole('button', { name: 'Save review' }).hasAttribute('disabled')).toBe(true);
    mocks.teamReview.mockResolvedValue({ ...review, version: 4, notes: 'Latest notes' });
    await fireEvent.click(screen.getByRole('button', { name: 'Reload latest (discard draft)' }));
    await waitFor(() => expect((screen.getByLabelText('Review notes') as HTMLTextAreaElement).value).toBe('Latest notes'));
    expect(screen.getByRole('button', { name: 'Save review' }).hasAttribute('disabled')).toBe(false);
  });
  it('does not load the roster or show reassignment controls to a reviewer', async () => {
    mocks.teamAccess.mockResolvedValue({ user_id: 2, name: 'Reviewer', role: 'reviewer' });
    mount(); await screen.findByLabelText('Review notes');
    expect(screen.queryByLabelText('Assignee')).toBeNull();
    expect(mocks.teamMembers).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    await waitFor(() => expect(mocks.saveTeamReview).toHaveBeenCalledWith('python', expect.objectContaining({ assignee_id: 2 })));
  });
  it('removes private fields after access is revoked while saving', async () => {
    mocks.saveTeamReview.mockRejectedValue(new ApiError(403, 'Team access is required'));
    mount(); await screen.findByLabelText('Review notes');
    await fireEvent.click(screen.getByRole('button', { name: 'Save review' }));
    await screen.findByRole('alert');
    expect(screen.queryByLabelText('Review notes')).toBeNull();
    expect(screen.queryByText('Original notes')).toBeNull();
  });
  it('shows saved audit snapshots as text', async () => {
    mocks.teamReviewHistory.mockResolvedValue([{ id: 1, actor: 'Manager', action: 'assigned', assignee: 'Reviewer', status: 'pending', notes: '<script>not markup</script>', follow_up: '2026-12-01', version: 1, created_at: '2026-11-05' }]);
    mount(); await screen.findByLabelText('Review notes');
    await fireEvent.click(screen.getByRole('button', { name: 'View change history' }));
    expect(await screen.findByText('<script>not markup</script>')).toBeTruthy();
  });
});
