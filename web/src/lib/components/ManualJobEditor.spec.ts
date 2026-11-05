import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { tick } from 'svelte';
import { ApiError } from '#lib/api.js';
import type { Job } from '#lib/types.js';
import { must } from '#lib/utils.js';
import ManualJobEditor from './ManualJobEditor.svelte';

const { getJob, updateManualJob, auth } = vi.hoisted(() => ({
  getJob: vi.fn(), updateManualJob: vi.fn(), auth: { role: 'moderator' as string | null },
}));
vi.mock('#lib/api.js', async (importOriginal) => ({
  ...await importOriginal<typeof import('#lib/api.js')>(), api: { getJob, updateManualJob },
}));
vi.mock('#lib/auth.svelte.js', () => ({
  currentUser: () => auth.role ? { role: auth.role } : null,
}));
vi.mock('./NoteEditor.svelte', async () => ({
  default: (await import('../../../vitest-stubs/NoteEditor.svelte')).default,
}));

const job = {
  public_slug: 'python-engineer', manually_added: true, title: 'Python Engineer',
  company: 'Example BD', location: 'Dhaka', description: '<p>Build <strong>Python</strong> services.</p>',
  source: 'bdjobs', url: 'https://example.com/jobs/python', work_mode: 'remote',
} as Job;

beforeEach(() => {
  vi.resetAllMocks();
  auth.role = 'moderator';
  getJob.mockResolvedValue({ ...job });
  updateManualJob.mockResolvedValue({ ...job, title: 'Senior Python Engineer' });
});

async function openEditor() {
  await fireEvent.click(screen.getByRole('button', { name: 'Edit job' }));
  await screen.findByRole('textbox', { name: 'Title' });
}

describe('manual job editing', () => {
  it('corrects the posting date independently of the job text', async () => {
    const onSaved = vi.fn();
    render(ManualJobEditor, { job, onSaved });
    await openEditor();
    await fireEvent.input(screen.getByLabelText('New posting date'), { target: { value: '2026-01-15' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(updateManualJob).toHaveBeenCalledWith(job.public_slug, {
      posted_at: '2026-01-14T18:00:00.000Z',
    }));
    expect(await screen.findByText('Job updated.')).toBeTruthy();
  });

  it('preserves an existing posting date when correcting another field', async () => {
    getJob.mockResolvedValue({ ...job, posted_at: '2026-01-14T18:00:00Z' });
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await openEditor();
    expect(screen.getByText(/Current displayed posting date/).textContent).toContain('15 January 2026');
    expect((screen.getByLabelText('New posting date') as HTMLInputElement).value).toBe('');
    await fireEvent.input(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'Corrected title' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(updateManualJob).toHaveBeenCalledWith(job.public_slug, { title: 'Corrected title' }));
  });

  it('treats an equivalent timestamp as unchanged and clearing the selection as undo', async () => {
    getJob.mockResolvedValue({ ...job, posted_at: '2026-01-14T18:00:00Z' });
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await openEditor();
    await fireEvent.input(screen.getByLabelText('New posting date'), { target: { value: '2026-01-15' } });
    expect((screen.getByRole('button', { name: 'Save changes' }) as HTMLButtonElement).disabled).toBe(true);
    await fireEvent.input(screen.getByLabelText('New posting date'), { target: { value: '2026-01-16' } });
    expect((screen.getByRole('button', { name: 'Save changes' }) as HTMLButtonElement).disabled).toBe(false);
    await fireEvent.input(screen.getByLabelText('New posting date'), { target: { value: '' } });
    expect((screen.getByRole('button', { name: 'Save changes' }) as HTMLButtonElement).disabled).toBe(true);
    expect(updateManualJob).not.toHaveBeenCalled();
  });

  it('retains the selected date after a failed save and refuses future dates', async () => {
    updateManualJob.mockRejectedValueOnce(new ApiError(500, 'Try again'));
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await openEditor();
    await fireEvent.input(screen.getByLabelText('New posting date'), { target: { value: '2026-01-15' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    expect((await screen.findByRole('alert')).textContent).toBe('Try again');
    expect((screen.getByLabelText('New posting date') as HTMLInputElement).value).toBe('2026-01-15');
    await fireEvent.input(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'Corrected title' } });
    await fireEvent.input(screen.getByLabelText('New posting date'), { target: { value: '2099-01-01' } });
    expect((screen.getByRole('button', { name: 'Save changes' }) as HTMLButtonElement).disabled).toBe(true);
    expect(updateManualJob).toHaveBeenCalledTimes(1);
  });

  it.each([null, 'user', 'admin'])('hides editing from the %s role', (role) => {
    auth.role = role;
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    expect(screen.queryByRole('button', { name: 'Edit job' })).toBeNull();
    expect(getJob).not.toHaveBeenCalled();
  });

  it('hides editing from automatically collected postings regardless of source', () => {
    render(ManualJobEditor, { job: { ...job, manually_added: false, source: 'manual' }, onSaved: vi.fn() });
    expect(screen.queryByRole('button', { name: 'Edit job' })).toBeNull();
  });

  it('loads the latest job only when asked and keeps identity read-only', async () => {
    getJob.mockResolvedValue({ ...job, title: 'Latest title' });
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    expect(getJob).not.toHaveBeenCalled();
    await openEditor();
    expect(getJob).toHaveBeenCalledWith('python-engineer');
    expect((screen.getByRole('textbox', { name: 'Title' }) as HTMLInputElement).value).toBe('Latest title');
    expect((screen.getByRole('textbox', { name: 'Original URL' }) as HTMLInputElement).readOnly).toBe(true);
    expect((screen.getByRole('button', { name: 'Save changes' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('patches only changed fields, retaining untouched HTML, and reports the updated job', async () => {
    const onSaved = vi.fn();
    render(ManualJobEditor, { job, onSaved });
    await openEditor();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Title' }), {
      target: { value: ' Senior Python Engineer ' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith({ ...job, title: 'Senior Python Engineer' }));
    expect(updateManualJob).toHaveBeenCalledWith('python-engineer', { title: 'Senior Python Engineer' });
    expect(await screen.findByRole('status')).toHaveProperty('textContent', 'Job updated.');
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('supports clearing optional text and turning off the remote flag', async () => {
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await openEditor();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Location' }), { target: { value: '' } });
    await fireEvent.input(screen.getByRole('textbox', { name: 'Description' }), { target: { value: '' } });
    await fireEvent.click(screen.getByRole('checkbox', { name: 'Fully remote' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(updateManualJob).toHaveBeenCalledWith('python-engineer', {
      location: '', description: '', remote: false,
    }));
  });

  it('previews sanitized Markdown and saves it as HTML', async () => {
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await openEditor();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Description' }), {
      target: { value: '**Data science** <img src="x" onerror="alert(1)">' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Preview description' }));
    expect(screen.getByText('Data science').tagName).toBe('STRONG');
    expect(document.querySelector('[onerror]')).toBeNull();
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(updateManualJob).toHaveBeenCalledWith('python-engineer', {
      description: expect.stringContaining('<strong>Data science</strong>'),
    }));
  });

  it('keeps a failed draft for retry and surfaces the backend refusal', async () => {
    updateManualJob.mockRejectedValueOnce(new ApiError(403, 'Moderator access required'));
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await openEditor();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Company' }), { target: { value: 'Correct company' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    expect((await screen.findByRole('alert')).textContent).toBe('Moderator access required');
    expect((screen.getByRole('textbox', { name: 'Company' }) as HTMLInputElement).value).toBe('Correct company');
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    expect(await screen.findByText('Job updated.')).toBeTruthy();
    expect(updateManualJob).toHaveBeenCalledTimes(2);
  });

  it('cannot save blank required content', async () => {
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await openEditor();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Title' }), { target: { value: '   ' } });
    expect((screen.getByRole('button', { name: 'Save changes' }) as HTMLButtonElement).disabled).toBe(true);
    expect(updateManualJob).not.toHaveBeenCalled();
  });

  it('cancels without saving and reloads a fresh draft on reopen', async () => {
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await openEditor();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'Discard me' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(updateManualJob).not.toHaveBeenCalled();
    await openEditor();
    expect((screen.getByRole('textbox', { name: 'Title' }) as HTMLInputElement).value).toBe(job.title);
    expect(getJob).toHaveBeenCalledTimes(2);
  });

  it('handles a missing or private job without offering a write', async () => {
    getJob.mockRejectedValue(new ApiError(404, 'job not found'));
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await fireEvent.click(screen.getByRole('button', { name: 'Edit job' }));
    expect((await screen.findByRole('alert')).textContent).toBe('job not found');
    expect(screen.queryByRole('button', { name: 'Save changes' })).toBeNull();
  });

  it('refuses a job that is no longer manually authored', async () => {
    getJob.mockResolvedValue({ ...job, manually_added: false });
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await fireEvent.click(screen.getByRole('button', { name: 'Edit job' }));
    expect((await screen.findByRole('alert')).textContent).toContain('cannot be edited here');
    expect(updateManualJob).not.toHaveBeenCalled();
  });

  it('holds the dialog and prevents duplicate writes while saving', async () => {
    let finish!: (job: Job) => void;
    updateManualJob.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    render(ManualJobEditor, { job, onSaved: vi.fn() });
    await openEditor();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'Updated title' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(updateManualJob).toHaveBeenCalledTimes(1));
    expect((screen.getByRole('button', { name: 'Saving…' }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole('button', { name: 'Cancel' }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.queryByRole('button', { name: 'Close' })).toBeNull();
    await fireEvent.submit(must(screen.getByRole('textbox', { name: 'Title' }).closest('form')));
    expect(updateManualJob).toHaveBeenCalledTimes(1);
    finish({ ...job, title: 'Updated title' });
    expect(await screen.findByText('Job updated.')).toBeTruthy();
  });

  it('keeps closed-job state from the saved response', async () => {
    const closed = { ...job, closed_at: '2026-11-01T00:00:00Z' };
    getJob.mockResolvedValue(closed);
    updateManualJob.mockResolvedValue({ ...closed, title: 'Corrected title' });
    const onSaved = vi.fn();
    render(ManualJobEditor, { job: closed, onSaved });
    await openEditor();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'Corrected title' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ closed_at: closed.closed_at })));
    expect(updateManualJob).toHaveBeenCalledWith(job.public_slug, { title: 'Corrected title' });
  });

  it('ignores a completed save after navigation unmounts the editor', async () => {
    let finish!: (job: Job) => void;
    const pending = new Promise<Job>((resolve) => { finish = resolve; });
    updateManualJob.mockReturnValue(pending);
    const onSaved = vi.fn();
    const view = render(ManualJobEditor, { job, onSaved });
    await openEditor();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'Updated title' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(updateManualJob).toHaveBeenCalledTimes(1));
    view.unmount();
    finish({ ...job, title: 'Updated title' });
    await pending;
    await tick();
    expect(onSaved).not.toHaveBeenCalled();
  });
});
