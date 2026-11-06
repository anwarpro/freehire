import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import { StreamInterrupted } from '#lib/assistant/client.js';
import type { TurnEvent } from '#lib/assistant/wire.js';
import TeamJobDiscussion from './TeamJobDiscussion.svelte';
const mocks = vi.hoisted(() => ({ teamBotStatus: vi.fn(), teamBotSessions: vi.fn(), createTeamBotSession: vi.fn(), teamBotSession: vi.fn(), deleteTeamBotSession: vi.fn(), teamJobTurn: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
vi.mock('#lib/assistant/client.js', async (original) => ({ ...await original<typeof import('#lib/assistant/client.js')>(), teamJobTurn: mocks.teamJobTurn }));
const session = { id: 'session-1', label: 'React roles', preset: 'team_jobs' };
const settings = { model_configured: true, paid_access: true, can_chat: true, data_scope: 'Public jobs and automatic course evidence.', access_policy: 'Paid staff access', language: 'English' };
beforeEach(() => { vi.resetAllMocks(); mocks.teamBotStatus.mockResolvedValue(settings); mocks.teamBotSessions.mockResolvedValue({ data: [session], meta: { has_more: false, offset: 0, limit: 50 } }); mocks.createTeamBotSession.mockResolvedValue(session); mocks.teamBotSession.mockResolvedValue({ session, messages: [] }); mocks.deleteTeamBotSession.mockResolvedValue(undefined); mocks.teamJobTurn.mockImplementation((_id: string, text: string, event: (e: TurnEvent) => void) => { event({ type: 'user_prompt', text }); event({ type: 'assistant_text', text: 'React is explicitly required.' }); event({ type: 'result' }); return { done: Promise.resolve(), cancel: vi.fn() }; }); });
describe('team job discussion', () => {
  it('shows honest setup/entitlement state and does not start a conversation automatically', async () => {
    mocks.teamBotStatus.mockResolvedValue({ ...settings, model_configured: false, paid_access: false, can_chat: false }); render(TeamJobDiscussion);
    await screen.findByText(/awaiting model configuration/); expect(screen.getByRole('button', { name: 'New job conversation' }).hasAttribute('disabled')).toBe(true); expect(mocks.createTeamBotSession).not.toHaveBeenCalled();
    expect(screen.getByText(/Pro or Ultra access/)).toBeTruthy();
  });
  it('starts and streams an explicit question through the shared client', async () => {
    render(TeamJobDiscussion); await screen.findByLabelText('Your job conversations'); await fireEvent.click(screen.getByRole('button', { name: 'New job conversation' }));
    await screen.findByLabelText('Ask about collected jobs'); await fireEvent.input(screen.getByLabelText('Ask about collected jobs'), { target: { value: 'Recent React roles?' } }); await fireEvent.click(screen.getByRole('button', { name: 'Send question' }));
    expect(await screen.findByText('React is explicitly required.')).toBeTruthy(); expect(mocks.teamJobTurn).toHaveBeenCalledWith('session-1', 'Recent React roles?', expect.any(Function), false);
  });
  it('clears conversations and transcript after revoked membership', async () => {
    render(TeamJobDiscussion); await screen.findByLabelText('Your job conversations'); await fireEvent.click(screen.getByRole('button', { name: 'New job conversation' })); await screen.findByLabelText('Ask about collected jobs');
    mocks.teamBotStatus.mockRejectedValue(new ApiError(403, 'Access revoked')); await fireEvent.click(screen.getByRole('button', { name: 'Refresh availability' })); await screen.findByRole('alert'); expect(screen.queryByLabelText('Your job conversations')).toBeNull(); expect(screen.queryByLabelText('Ask about collected jobs')).toBeNull();
  });
  it('requires reloading after an interrupted stream before sending again', async () => {
    // Construct the rejection only when the click starts the request.
    mocks.teamJobTurn.mockImplementation(() => ({ done: Promise.reject(new StreamInterrupted(new Error('offline'))), cancel: vi.fn() }));
    render(TeamJobDiscussion); await screen.findByLabelText('Your job conversations'); await fireEvent.click(screen.getByRole('button', { name: 'New job conversation' })); await screen.findByLabelText('Ask about collected jobs');
    await fireEvent.input(screen.getByLabelText('Ask about collected jobs'), { target: { value: 'Python roles?' } }); await fireEvent.click(screen.getByRole('button', { name: 'Send question' })); await screen.findByRole('alert'); expect(screen.getByRole('button', { name: 'Send question' }).hasAttribute('disabled')).toBe(true);
    await fireEvent.click(screen.getByRole('button', { name: 'Reload conversation' })); await waitFor(() => expect(mocks.teamBotSession).toHaveBeenCalledWith('session-1'));
  });
  it('requires explicit confirmation before deleting a conversation', async () => {
    render(TeamJobDiscussion); await screen.findByLabelText('Your job conversations'); await fireEvent.change(screen.getByLabelText('Your job conversations'), { target: { value: 'session-1' } }); await screen.findByLabelText('Ask about collected jobs');
    await fireEvent.click(screen.getByRole('button', { name: 'Delete conversation' })); expect(mocks.deleteTeamBotSession).not.toHaveBeenCalled(); await fireEvent.click(screen.getByRole('button', { name: 'Confirm delete' })); await waitFor(() => expect(mocks.deleteTeamBotSession).toHaveBeenCalledWith('session-1'));
  });
});
