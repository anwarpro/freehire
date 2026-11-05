import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import CurriculumSettingsEditor from './CurriculumSettingsEditor.svelte';
const mocks = vi.hoisted(() => ({ saveCurriculumSettings: vi.fn(), curriculumConfigHistory: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
const settings = { version: 2, config: { tracks: [{ id: 'python', label: 'Python', enabled: true, minimum: 1, skills: [{ name: 'Python', aliases: ['python'] }] }] }, progress: { mapping_version: 2, cursor_id: 0, upper_id: 0, complete: false, processed: 0, last_run_at: null, last_error: '' } };
beforeEach(() => { vi.resetAllMocks(); mocks.saveCurriculumSettings.mockResolvedValue({ ...settings, version: 3 }); mocks.curriculumConfigHistory.mockResolvedValue([]); });
describe('editable curriculum mapping', () => {
  it('saves aliases against the loaded mapping version', async () => {
    const onSaved = vi.fn(); render(CurriculumSettingsEditor, { settings, onSaved, onReload: vi.fn() });
    const rules = await screen.findByLabelText(/Skill rules for Python/);
    await fireEvent.input(rules, { target: { value: 'Python | python, python3\nFastAPI | fastapi' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save curriculum mapping' }));
    await waitFor(() => expect(mocks.saveCurriculumSettings).toHaveBeenCalledWith({ expected_version: 2, config: { tracks: [{ ...settings.config.tracks[0], skills: [{ name: 'Python', aliases: ['python', 'python3'] }, { name: 'FastAPI', aliases: ['fastapi'] }] }] } }));
    expect(onSaved).toHaveBeenCalledOnce();
  });
  it('keeps conflicting drafts until explicit reload', async () => {
    mocks.saveCurriculumSettings.mockRejectedValue(new ApiError(409, 'Changed'));
    const onReload = vi.fn(); render(CurriculumSettingsEditor, { settings, onSaved: vi.fn(), onReload });
    await screen.findByLabelText(/Skill rules for Python/);
    await fireEvent.input(screen.getByLabelText('Badge label'), { target: { value: 'My Python label' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save curriculum mapping' }));
    expect((await screen.findByRole('alert')).textContent).toContain('draft is kept');
    expect((screen.getByLabelText('Badge label') as HTMLInputElement).value).toBe('My Python label');
    expect(screen.getByRole('button', { name: 'Save curriculum mapping' }).hasAttribute('disabled')).toBe(true);
    await fireEvent.click(screen.getByRole('button', { name: 'Reload mapping (discard draft)' })); expect(onReload).toHaveBeenCalledOnce();
  });
  it('rejects malformed rules and can remove an unsaved track', async () => {
    render(CurriculumSettingsEditor, { settings, onSaved: vi.fn(), onReload: vi.fn() });
    const rules = await screen.findByLabelText(/Skill rules for Python/);
    await fireEvent.input(rules, { target: { value: 'Missing separator' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Save curriculum mapping' }));
    expect((await screen.findByRole('alert')).textContent).toContain('Line 1'); expect(mocks.saveCurriculumSettings).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByRole('button', { name: 'Add track' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Remove unsaved track' }));
    expect(screen.getAllByLabelText('Track ID')).toHaveLength(1);
  });
});
