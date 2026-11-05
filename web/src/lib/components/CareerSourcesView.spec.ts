import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api.js';
import CareerSourcesView from './CareerSourcesView.svelte';
const mocks = vi.hoisted(() => ({ listCareerSources: vi.fn(), previewCareerSource: vi.fn(), enableCareerSource: vi.fn(), pauseCareerSource: vi.fn() }));
vi.mock('#lib/api.js', async (original) => ({ ...await original<typeof import('#lib/api.js')>(), api: mocks }));
vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));
const seed = { company: 'Example BD', url: 'https://example.test/careers' };
const preview = { provider: 'careerpage', board: seed.url, active: 2, closed: 1, samples: [{ title: 'Python Engineer', url: 'https://example.test/job/1', closed: false }] };
beforeEach(() => {
  vi.resetAllMocks();
  mocks.listCareerSources.mockResolvedValue({ data: [], shortlist: [seed, { company: 'Blocked BD', url: 'https://blocked.test', fallback: 'Access could not be verified.' }] });
  mocks.previewCareerSource.mockResolvedValue(preview);
  mocks.enableCareerSource.mockResolvedValue(undefined);
  mocks.pauseCareerSource.mockResolvedValue(undefined);
});
async function configure() { await fireEvent.click(await screen.findByRole('button', { name: 'Configure Example BD' })); }
async function verify() { await fireEvent.click(screen.getByRole('button', { name: 'Preview collection' })); await screen.findByRole('region', { name: 'Collection preview' }); }

describe('company career source management', () => {
  it('requires a fresh preview before enabling and queues collection', async () => {
    render(CareerSourcesView); await configure();
    const enable = screen.getByRole('button', { name: 'Enable collection' });
    expect(enable.hasAttribute('disabled')).toBe(true);
    await verify(); expect(screen.getByRole('link', { name: 'Python Engineer ↗' }).getAttribute('href')).toBe('https://example.test/job/1');
    await fireEvent.click(enable);
    await waitFor(() => expect(mocks.enableCareerSource).toHaveBeenCalledWith(seed));
    expect(await screen.findByRole('status')).toBeTruthy();
  });
  it('invalidates the preview when the URL changes', async () => {
    render(CareerSourcesView); await configure(); await verify();
    await fireEvent.input(screen.getByLabelText('Career page URL'), { target: { value: 'https://different.test/jobs' } });
    expect(screen.getByRole('button', { name: 'Enable collection' }).hasAttribute('disabled')).toBe(true);
  });
  it('preserves the draft on a refusal and labels manual fallbacks', async () => {
    mocks.previewCareerSource.mockRejectedValue(new ApiError(422, 'Could not verify this page'));
    render(CareerSourcesView); await configure();
    await fireEvent.click(screen.getByRole('button', { name: 'Preview collection' }));
    expect((await screen.findByRole('alert')).textContent).toContain('Could not verify this page');
    expect((screen.getByLabelText('Career page URL') as HTMLInputElement).value).toBe(seed.url);
    expect(screen.getByText('Access could not be verified.')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Configure Blocked BD' })).toBeNull();
  });
  it('shows failures and pauses a collection', async () => {
    mocks.listCareerSources.mockResolvedValue({ shortlist: [], data: [{ ...seed, id: 12, provider: 'careerpage', board: seed.url, status: 'pending', state: 'collection_failed', last_run_at: null, last_success_at: null, failures: 2, last_ingested_count: 0 }] });
    render(CareerSourcesView);
    expect(await screen.findByText(/Collection failed/)).toBeTruthy();
    await fireEvent.click(screen.getByRole('button', { name: 'Pause collection' }));
    await waitFor(() => expect(mocks.pauseCareerSource).toHaveBeenCalledWith(12));
  });
});
