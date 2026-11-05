import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import StudentFollowupEditor from './StudentFollowupEditor.svelte';
const mocks = vi.hoisted(() => ({ studentFollowup: vi.fn(), createStudentFollowup: vi.fn(), updateStudentFollowup: vi.fn(), recordStudentContact: vi.fn(), studentFollowupHistory: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
const row = { id: 7, job_id: 1, public_slug: 'react-job', title: 'React developer', company: 'Company', job_url: 'https://company.test/job', student_ref: 'PH-42', student_name: 'Example student', profile_url: 'https://school.test/42', reason: 'React practice', status: 'suggested', notes: 'Private follow-up', follow_up: null, version: 1, closed_at: null, contact_count: 0 };
beforeEach(() => { vi.resetAllMocks(); mocks.studentFollowup.mockResolvedValue(row); mocks.createStudentFollowup.mockResolvedValue({ id: 7, version: 1 }); mocks.updateStudentFollowup.mockResolvedValue({ id: 7, version: 2 }); mocks.recordStudentContact.mockResolvedValue({ id: 7, version: 2 }); mocks.studentFollowupHistory.mockResolvedValue([]); });
const mount = () => render(StudentFollowupEditor, { slug: 'react-job', id: 7, onSaved: vi.fn(), onClose: vi.fn() });
describe('student follow-up editor', () => {
  it('adds a reference without requiring a directory', async () => {
    const onSaved = vi.fn(), onClose = vi.fn(); render(StudentFollowupEditor, { slug: 'react-job', id: 0, onSaved, onClose });
    await fireEvent.input(screen.getByLabelText('Student ID or profile reference', { exact: false }), { target: { value: 'PH-42' } });
    await fireEvent.input(screen.getByLabelText('Why this job may fit'), { target: { value: 'React practice' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Link student' }));
    await waitFor(() => expect(mocks.createStudentFollowup).toHaveBeenCalledWith('react-job', { student_ref: 'PH-42', student_name: '', profile_url: '', reason: 'React practice' }));
    expect(onSaved).toHaveBeenCalledOnce(); expect(onClose).toHaveBeenCalledOnce();
  });
  it('copying a draft never records contact or changes outcomes', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined); Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    mount(); await screen.findByLabelText('Message draft'); await fireEvent.click(screen.getByRole('button', { name: 'Copy message' }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(expect.stringContaining('https://company.test/job')));
    expect(mocks.recordStudentContact).not.toHaveBeenCalled(); expect(mocks.updateStudentFollowup).not.toHaveBeenCalled();
  });
  it('records explicit manual contact and reuses its ID on network retry', async () => {
    mocks.recordStudentContact.mockRejectedValueOnce(new Error('Network failure')).mockResolvedValueOnce({ id: 7, version: 2 });
    mount(); await screen.findByLabelText('Contact channel');
    await fireEvent.input(screen.getByLabelText('Contacted at (Bangladesh time)'), { target: { value: '2026-11-06T08:30' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Record manual contact' })); await screen.findByRole('alert');
    await fireEvent.click(screen.getByRole('button', { name: 'Record manual contact' }));
    await waitFor(() => expect(mocks.recordStudentContact).toHaveBeenCalledTimes(2));
    const first = mocks.recordStudentContact.mock.calls[0]?.[2]; expect(first.contacted_at).toBe('2026-11-06T08:30:00+06:00'); expect(first.expected_version).toBe(1);
    expect(mocks.recordStudentContact.mock.calls[1]?.[2]).toEqual(first);
  });
  it('requires saving outcome drafts before recording contact', async () => {
    mount(); await screen.findByLabelText('Follow-up notes');
    await fireEvent.input(screen.getByLabelText('Follow-up notes'), { target: { value: 'New outcome context' } });
    expect(screen.getByRole('button', { name: 'Record manual contact' }).hasAttribute('disabled')).toBe(true);
    expect(screen.getByText('Save outcome changes before recording contact.')).toBeTruthy();
    expect(mocks.recordStudentContact).not.toHaveBeenCalled();
  });
  it('keeps conflicting drafts until explicit reload', async () => {
    mocks.updateStudentFollowup.mockRejectedValue(new ApiError(409, 'Changed')); mount(); await screen.findByLabelText('Follow-up notes');
    await fireEvent.input(screen.getByLabelText('Follow-up notes'), { target: { value: 'My unsaved note' } }); await fireEvent.click(screen.getByRole('button', { name: 'Save outcome' }));
    expect((await screen.findByRole('alert')).textContent).toContain('draft is kept'); expect((screen.getByLabelText('Follow-up notes') as HTMLTextAreaElement).value).toBe('My unsaved note');
    expect(screen.getByRole('button', { name: 'Save outcome' }).hasAttribute('disabled')).toBe(true);
    mocks.studentFollowup.mockResolvedValue({ ...row, notes: 'Latest saved note', version: 2 }); await fireEvent.click(screen.getByRole('button', { name: 'Reload follow-up (discard draft)' }));
    await waitFor(() => expect((screen.getByLabelText('Follow-up notes') as HTMLTextAreaElement).value).toBe('Latest saved note'));
  });
  it('removes private content when access is revoked', async () => {
    mocks.updateStudentFollowup.mockRejectedValue(new ApiError(404, 'Unavailable')); mount(); await screen.findByLabelText('Follow-up notes');
    await fireEvent.click(screen.getByRole('button', { name: 'Save outcome' })); await screen.findByRole('alert'); expect(screen.queryByLabelText('Follow-up notes')).toBeNull(); expect(screen.queryByText('PH-42')).toBeNull(); expect(screen.queryByLabelText('Message draft')).toBeNull();
  });
  it('keeps outcome editing for closed postings but hides contact', async () => {
    mocks.studentFollowup.mockResolvedValue({ ...row, closed_at: '2026-11-01T00:00:00Z' }); mount(); await screen.findByLabelText('Outcome');
    expect(screen.queryByRole('button', { name: 'Record manual contact' })).toBeNull(); expect(screen.getByRole('button', { name: 'Save outcome' })).toBeTruthy();
  });
});
