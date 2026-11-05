import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import StudentFollowups from './StudentFollowups.svelte';
const mocks = vi.hoisted(() => ({ studentFollowups: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
vi.mock('$app/paths', () => ({ resolve: (path: string, params?: { slug: string }) => params ? path.replace('[slug]', params.slug) : path }));
const row = { id: 7, public_slug: 'react-job', title: 'React developer', company: 'Company', student_ref: 'PH-42', student_name: 'Example student', reason: 'React practice', status: 'informed', follow_up: '2026-11-01', overdue: true, contact_count: 1, assignee: 'Reviewer', closed_at: null };
beforeEach(() => { vi.resetAllMocks(); mocks.studentFollowups.mockResolvedValue({ data: [row], meta: { offset: 0, limit: 50, has_more: true } }); });
describe('student follow-up list', () => {
  it('filters the full queue and paginates', async () => {
    render(StudentFollowups); await screen.findByText('Example student'); await fireEvent.input(screen.getByLabelText('Student reference or name'), { target: { value: 'PH-42' } });
    await fireEvent.change(screen.getByLabelText('Outcome filter'), { target: { value: 'informed' } }); await fireEvent.click(screen.getByLabelText('Overdue student follow-ups only')); await fireEvent.click(screen.getByRole('button', { name: 'Filter students' }));
    await waitFor(() => expect(mocks.studentFollowups).toHaveBeenLastCalledWith({ q: 'PH-42', status: 'informed', overdue: true, offset: 0 }, undefined));
    await fireEvent.click(screen.getByRole('button', { name: 'Next' })); await waitFor(() => expect(mocks.studentFollowups).toHaveBeenLastCalledWith(expect.objectContaining({ offset: 50 }), undefined));
  });
  it('clears references on access revocation', async () => {
    render(StudentFollowups); await screen.findByText('Example student'); mocks.studentFollowups.mockRejectedValue(new ApiError(403, 'Team access required')); await fireEvent.click(screen.getByRole('button', { name: 'Refresh students' })); await screen.findByRole('alert'); expect(screen.queryByText('PH-42')).toBeNull(); expect(screen.queryByText('Example student')).toBeNull();
  });
  it('scopes to a job and prevents adding students to closed postings', async () => {
    render(StudentFollowups, { slug: 'react-job', closed: true }); await screen.findByText('Example student'); expect(mocks.studentFollowups).toHaveBeenCalledWith(expect.any(Object), 'react-job'); expect(screen.getByRole('button', { name: 'Link a student' }).hasAttribute('disabled')).toBe(true);
  });
});
