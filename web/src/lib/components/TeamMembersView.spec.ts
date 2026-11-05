import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import TeamMembersView from './TeamMembersView.svelte';
const mocks = vi.hoisted(() => ({ teamMembers: vi.fn(), createTeamMember: vi.fn(), updateTeamMember: vi.fn(), teamMemberHistory: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
beforeEach(() => {
  vi.resetAllMocks();
  mocks.teamMembers.mockResolvedValue([{ id: 2, name: 'Reviewer One', email: 'one@team.test', team_role: 'reviewer', active: true, version: 4, assignment_count: 2 }]);
  mocks.createTeamMember.mockResolvedValue({ id: 3 });
  mocks.updateTeamMember.mockResolvedValue(undefined);
  mocks.teamMemberHistory.mockResolvedValue([]);
});
describe('team member management', () => {
  it('creates a reviewer with an initial password and clears it after success', async () => {
    render(TeamMembersView); await screen.findByRole('button', { name: 'Manage Reviewer One' });
    await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'New Reviewer' } });
    await fireEvent.input(screen.getByLabelText('Email'), { target: { value: 'new@team.test' } });
    await fireEvent.input(screen.getByLabelText('Initial password'), { target: { value: 'private-password' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Add reviewer' }));
    await waitFor(() => expect(mocks.createTeamMember).toHaveBeenCalledWith({ name: 'New Reviewer', email: 'new@team.test', password: 'private-password', use_existing: false }));
    await waitFor(() => expect((screen.getByLabelText('Initial password') as HTMLInputElement).value).toBe(''));
  });
  it('adds a verified existing account without sending a replacement password', async () => {
    render(TeamMembersView);
    await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Existing Reviewer' } });
    await fireEvent.input(screen.getByLabelText('Email'), { target: { value: 'existing@team.test' } });
    await fireEvent.click(screen.getByLabelText('Use an existing verified account'));
    expect(screen.queryByLabelText('Initial password')).toBeNull();
    await fireEvent.click(screen.getByRole('button', { name: 'Add reviewer' }));
    await waitFor(() => expect(mocks.createTeamMember).toHaveBeenCalledWith({ name: 'Existing Reviewer', email: 'existing@team.test', use_existing: true }));
  });
  it('deactivates team access using the current member version', async () => {
    render(TeamMembersView);
    await fireEvent.click(await screen.findByRole('button', { name: 'Manage Reviewer One' }));
    await fireEvent.click(screen.getByLabelText('Team access active'));
    expect(screen.getByText(/Their 2 assignments and history remain/)).toBeTruthy();
    await fireEvent.click(screen.getByRole('button', { name: 'Save member' }));
    await waitFor(() => expect(mocks.updateTeamMember).toHaveBeenCalledWith(2, { name: 'Reviewer One', active: false, expected_version: 4 }));
  });
});
