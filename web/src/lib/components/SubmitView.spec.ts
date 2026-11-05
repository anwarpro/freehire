import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import SubmitView from './SubmitView.svelte';

const { publishJob, submitJob, findJobByUrl, prefillSubmission, auth } = vi.hoisted(() => ({
  publishJob: vi.fn(),
  submitJob: vi.fn(),
  findJobByUrl: vi.fn(),
  prefillSubmission: vi.fn(),
  auth: { signedIn: true, role: 'moderator' },
}));

vi.mock('#lib/api.js', async (importOriginal) => {
  const original = await importOriginal<typeof import('#lib/api.js')>();
  return { ...original, api: { publishJob, submitJob, findJobByUrl, prefillSubmission } };
});
vi.mock('#lib/auth.svelte.js', () => ({
  isAuthenticated: () => auth.signedIn,
  currentUser: () => auth.signedIn ? { role: auth.role } : null,
}));
vi.mock('$app/paths', () => ({
  resolve: (path: string, params?: { slug: string }) => path.replace('[slug]', params?.slug ?? ''),
}));
// EasyMDE requires browser layout APIs. The form's API, permissions and lifecycle
// are under test here; the editor has its own tests.
vi.mock('./NoteEditor.svelte', async () => ({
  default: (await import('../../../vitest-stubs/EmptyComponent.svelte')).default,
}));

async function fillBasics() {
  await fireEvent.input(screen.getByRole('textbox', { name: 'Job URL' }), {
    target: { value: 'https://example.com/jobs/python' },
  });
  await fireEvent.input(screen.getByRole('textbox', { name: /Title/ }), {
    target: { value: 'Python Engineer' },
  });
  await fireEvent.input(screen.getByRole('textbox', { name: /Company/ }), {
    target: { value: 'Example BD' },
  });
}

beforeEach(() => {
  vi.resetAllMocks();
  auth.signedIn = true;
  auth.role = 'moderator';
  findJobByUrl.mockResolvedValue(null);
  publishJob.mockResolvedValue({ title: 'Python Engineer', company: 'Example BD', public_slug: 'python-engineer' });
  submitJob.mockResolvedValue({ title: 'Python Engineer', company: 'Example BD' });
});

describe('moderator job publishing', () => {
  it('publishes a selected posting date and clears the selection on success', async () => {
    render(SubmitView, { mode: 'publish' });
    await fillBasics();
    await fireEvent.input(screen.getByLabelText('Posting date'), { target: { value: '2026-01-15' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Publish job' }));
    expect(await screen.findByRole('link', { name: 'Open published job' })).toBeTruthy();
    expect(publishJob).toHaveBeenCalledWith(expect.objectContaining({ posted_at: '2026-01-14T18:00:00.000Z' }));
    expect((screen.getByLabelText('Posting date') as HTMLInputElement).value).toBe('');
  });

  it('blocks a future posting date', async () => {
    render(SubmitView, { mode: 'publish' });
    await fillBasics();
    await fireEvent.input(screen.getByLabelText('Posting date'), { target: { value: '2099-01-01' } });
    expect((await screen.findByRole('alert')).textContent).toContain('after today');
    expect((screen.getByRole('button', { name: 'Publish job' }) as HTMLButtonElement).disabled).toBe(true);
    expect(publishJob).not.toHaveBeenCalled();
  });

  it('passes the optional posting date through the ordinary review flow', async () => {
    auth.role = 'user';
    render(SubmitView);
    await fillBasics();
    await fireEvent.input(screen.getByLabelText('Posting date'), { target: { value: '2026-01-15' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Submit for review' }));
    expect(await screen.findByRole('link', { name: 'My submissions' })).toBeTruthy();
    expect(submitJob).toHaveBeenCalledWith(expect.objectContaining({ posted_at: '2026-01-14T18:00:00.000Z' }));
    expect(publishJob).not.toHaveBeenCalled();
  });

  it('publishes the draft directly and links to the resulting job', async () => {
    render(SubmitView, { mode: 'publish' });
    await fillBasics();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Location' }), {
      target: { value: 'Dhaka, Bangladesh' },
    });
    await fireEvent.input(screen.getByRole('textbox', { name: 'Source' }), {
      target: { value: 'bdjobs' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Remote' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Publish job' }));

    expect(publishJob).toHaveBeenCalledWith(expect.objectContaining({
      url: 'https://example.com/jobs/python', title: 'Python Engineer', company: 'Example BD',
      location: 'Dhaka, Bangladesh', source: 'bdjobs', work_mode: 'remote', remote: true,
    }));
    expect(submitJob).not.toHaveBeenCalled();
    expect((await screen.findByRole('link', { name: 'Open published job' })).getAttribute('href'))
      .toBe('/jobs/python-engineer');
    expect(screen.queryByText(/awaiting review/)).toBeNull();
    expect((screen.getByRole('textbox', { name: /Title/ }) as HTMLInputElement).value).toBe('');
  });

  it.each(['user', 'admin'])('does not offer publishing to the %s role', (role) => {
    auth.role = role;
    render(SubmitView, { mode: 'publish' });
    expect(screen.queryByRole('form')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Publish job' })).toBeNull();
    expect(screen.getByText('Only moderators can publish jobs directly.')).toBeTruthy();
    expect(publishJob).not.toHaveBeenCalled();
  });

  it('requires sign-in', () => {
    auth.signedIn = false;
    render(SubmitView, { mode: 'publish' });
    expect(screen.queryByRole('button', { name: 'Publish job' })).toBeNull();
    expect(publishJob).not.toHaveBeenCalled();
  });

  it('preserves the draft and exposes a server refusal for retry', async () => {
    publishJob.mockRejectedValueOnce(new ApiError(403, 'Moderator access required'));
    render(SubmitView, { mode: 'publish' });
    await fillBasics();
    await fireEvent.click(screen.getByRole('button', { name: 'Publish job' }));
    expect((await screen.findByRole('alert')).textContent).toBe('Moderator access required');
    expect((screen.getByRole('textbox', { name: /Title/ }) as HTMLInputElement).value).toBe('Python Engineer');
    await fireEvent.click(screen.getByRole('button', { name: 'Publish job' }));
    expect(await screen.findByRole('link', { name: 'Open published job' })).toBeTruthy();
    expect(publishJob).toHaveBeenCalledTimes(2);
  });

  it('blocks a duplicate discovered by the URL lookup', async () => {
    findJobByUrl.mockResolvedValue({ public_slug: 'already-here' });
    render(SubmitView, { mode: 'publish' });
    await fillBasics();
    expect((await screen.findByRole('link', { name: 'open it' })).getAttribute('href')).toBe('/jobs/already-here');
    expect((screen.getByRole('button', { name: 'Publish job' }) as HTMLButtonElement).disabled).toBe(true);
    expect(publishJob).not.toHaveBeenCalled();
  });

  it('prevents a second publish while a request is pending', async () => {
    let finish!: (job: unknown) => void;
    publishJob.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    render(SubmitView, { mode: 'publish' });
    await fillBasics();
    await fireEvent.click(screen.getByRole('button', { name: 'Publish job' }));
    const pending = screen.getByRole('button', { name: 'Publishing…' }) as HTMLButtonElement;
    expect(pending.disabled).toBe(true);
    await fireEvent.click(pending);
    await waitFor(() => expect(publishJob).toHaveBeenCalledTimes(1));
    finish({ title: 'Python Engineer', company: 'Example BD', public_slug: 'python-engineer' });
    expect(await screen.findByRole('link', { name: 'Open published job' })).toBeTruthy();
  });

  it('keeps the ordinary submission path awaiting review', async () => {
    auth.role = 'user';
    render(SubmitView);
    await fillBasics();
    await fireEvent.click(screen.getByRole('button', { name: 'Submit for review' }));
    expect(await screen.findByRole('link', { name: 'My submissions' })).toBeTruthy();
    expect(submitJob).toHaveBeenCalledTimes(1);
    expect(publishJob).not.toHaveBeenCalled();
  });
});
