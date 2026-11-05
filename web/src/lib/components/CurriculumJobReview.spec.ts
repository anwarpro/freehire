import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import CurriculumJobReview from './CurriculumJobReview.svelte';
const mocks = vi.hoisted(() => ({ curriculumJob: vi.fn(), reviewCurriculum: vi.fn(), curriculumHistory: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
const tracks = [{ id: 'python', label: 'Python', enabled: true, minimum: 1, skills: [] }, { id: 'l1', label: 'L1 frontend', enabled: true, minimum: 1, skills: [] }];
const detail = { job_id: 1, mapping_version: 2, input_hash: 'body-v1', tracks, suggested_tracks: ['python'], reviewed_tracks: null, effective_tracks: ['python'], review_version: 0, review_notes: '', reviewed: false, review_stale: false, evidence: { truncated: false, matches: [{ track_id: 'python', evidence: [{ skill: 'Python', alias: 'python', kind: 'required', text: 'Python required for APIs.' }] }] } };
beforeEach(() => { vi.resetAllMocks(); mocks.curriculumJob.mockResolvedValue(detail); mocks.reviewCurriculum.mockResolvedValue({ ...detail, reviewed: true, reviewed_tracks: ['python'], review_version: 1 }); mocks.curriculumHistory.mockResolvedValue([]); });
describe('curriculum review', () => {
  it('shows quoted evidence and saves a reviewer correction with input and mapping versions', async () => {
    const onSaved = vi.fn(); render(CurriculumJobReview, { slug: 'engineer', onSaved });
    await screen.findByText('Python required for APIs.');
    expect(screen.getByText('Suggested · unreviewed')).toBeTruthy();
    await fireEvent.click(screen.getByLabelText('L1 frontend'));
    await fireEvent.input(screen.getByLabelText('Curriculum review notes'), { target: { value: 'Frontend skills also fit this course.' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save curriculum labels' }));
    await waitFor(() => expect(mocks.reviewCurriculum).toHaveBeenCalledWith('engineer', { tracks: ['l1', 'python'], notes: 'Frontend skills also fit this course.', expected_version: 0, mapping_version: 2, input_hash: 'body-v1' }));
    expect(onSaved).toHaveBeenCalledOnce();
  });
  it('keeps a conflicting draft until the user explicitly reloads current evidence', async () => {
    mocks.reviewCurriculum.mockRejectedValue(new ApiError(409, 'Changed'));
    render(CurriculumJobReview, { slug: 'engineer' }); await screen.findByText('Python required for APIs.');
    await fireEvent.input(screen.getByLabelText('Curriculum review notes'), { target: { value: 'My unsaved review' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save curriculum labels' }));
    expect((await screen.findByRole('alert')).textContent).toContain('draft is kept');
    expect((screen.getByLabelText('Curriculum review notes') as HTMLTextAreaElement).value).toBe('My unsaved review');
    expect(screen.getByRole('button', { name: 'Save curriculum labels' }).hasAttribute('disabled')).toBe(true);
    mocks.curriculumJob.mockResolvedValue({ ...detail, input_hash: 'body-v2', review_notes: 'Latest review' });
    await fireEvent.click(screen.getByRole('button', { name: 'Reload curriculum (discard draft)' }));
    await waitFor(() => expect((screen.getByLabelText('Curriculum review notes') as HTMLTextAreaElement).value).toBe('Latest review'));
  });
  it('distinguishes reviewed no-fit from restoring automatic suggestions', async () => {
    mocks.curriculumJob.mockResolvedValue({ ...detail, reviewed: true, reviewed_tracks: [], effective_tracks: [], review_version: 3 });
    render(CurriculumJobReview, { slug: 'engineer' }); await screen.findByText('No matching track');
    await fireEvent.click(screen.getByRole('button', { name: 'Restore automatic suggestions' }));
    await waitFor(() => expect(mocks.reviewCurriculum).toHaveBeenCalledWith('engineer', { tracks: null, notes: '', expected_version: 3, mapping_version: 2, input_hash: 'body-v1' }));
  });
  it('removes private review content when access is revoked', async () => {
    mocks.curriculumJob.mockResolvedValue({ ...detail, review_notes: 'Private curriculum note' });
    mocks.reviewCurriculum.mockRejectedValue(new ApiError(404, 'Unavailable'));
    render(CurriculumJobReview, { slug: 'engineer' }); await screen.findByText('Python required for APIs.');
    await fireEvent.click(screen.getByRole('button', { name: 'Save curriculum labels' }));
    await screen.findByRole('alert'); expect(screen.queryByLabelText('Curriculum review notes')).toBeNull(); expect(screen.queryByText('Python required for APIs.')).toBeNull();
  });
});
