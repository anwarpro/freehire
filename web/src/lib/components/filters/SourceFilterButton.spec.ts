import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import SourceFilterButton from './SourceFilterButton.svelte';

describe('source filter shortcut', () => {
  it('opens the picker with no selected sources', async () => {
    const onOpen = vi.fn();
    render(SourceFilterButton, { selection: { include: [], exclude: [], matchAll: false }, onOpen });
    const button = screen.getByRole('button', { name: 'Filter by source' });
    expect(button.getAttribute('aria-haspopup')).toBe('dialog');
    expect(button.getAttribute('aria-expanded')).toBe('false');
    await fireEvent.click(button);
    expect(onOpen).toHaveBeenCalledOnce();
  });

  it('describes included and excluded sources and reflects changes after applying or clearing', async () => {
    const view = render(SourceFilterButton, {
      selection: { include: ['bdjobs'], exclude: ['linkedin'], matchAll: false },
      open: true, onOpen: vi.fn(),
    });
    const button = screen.getByRole('button', { name: 'Filter by source: BDJobs; excluding LinkedIn' });
    expect(button.getAttribute('aria-expanded')).toBe('true');
    expect(button.textContent).toContain('2');
    await view.rerender({ selection: { include: ['greenhouse'], exclude: [], matchAll: false }, open: false });
    expect(screen.getByRole('button', { name: 'Filter by source: Greenhouse' }).getAttribute('aria-expanded')).toBe('false');
    await view.rerender({ selection: { include: [], exclude: [], matchAll: false } });
    expect(screen.getByRole('button', { name: 'Filter by source' }).textContent).not.toContain('2');
  });
});
