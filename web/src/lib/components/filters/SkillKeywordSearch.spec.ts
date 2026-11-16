import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import { StagedFilters } from '#lib/stagedFilters.svelte.js';
import { filtersFromParams } from '#lib/facetModel.js';
import SkillKeywordSearch from './SkillKeywordSearch.svelte';

describe('manual skill keyword search', () => {
  it('accepts a skill outside the dictionary without changing other staged filters', async () => {
    const staged = new StagedFilters();
    staged.seed(filtersFromParams(new URLSearchParams('countries=bd&skills=react&q=developer&q_fields=title')));
    render(SkillKeywordSearch, { value: staged.value.q, onChange: (q) => staged.setQuery(q) });
    const input = screen.getByRole('searchbox', { name: 'Search job text for a skill' });
    expect((input as HTMLInputElement).value).toBe('developer');
    await fireEvent.input(input, { target: { value: 'Mongoose' } });
    expect(staged.params().get('q')).toBe('Mongoose');
    expect(staged.params().get('q_fields')).toBe('title,description');
    expect(staged.params().get('countries')).toBe('bd');
    expect(staged.params().get('skills')).toBe('react');
    const restored = filtersFromParams(staged.params());
    expect(restored.q).toBe('Mongoose');
  });

  it('can clear a keyword and reflects a changed applied query', async () => {
    const onChange = vi.fn();
    const view = render(SkillKeywordSearch, { value: 'Mongoose', onChange });
    const input = screen.getByRole('searchbox');
    await fireEvent.input(input, { target: { value: '' } });
    expect(onChange).toHaveBeenCalledWith('');
    await view.rerender({ value: 'Node.js' });
    expect((input as HTMLInputElement).value).toBe('Node.js');
  });
});
