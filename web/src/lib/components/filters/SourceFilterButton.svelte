<script lang="ts">
  import type { FacetSelection } from '#lib/facets.js';
  import { sourceLabel } from '#lib/facets.js';
  import { Button } from '#lib/ui/index.js';

  let { selection, open = false, onOpen }: {
    selection: FacetSelection;
    open?: boolean;
    onOpen: () => void;
  } = $props();
  const count = $derived(selection.include.length + selection.exclude.length);
  const summary = $derived([
    selection.include.map(sourceLabel).join(', '),
    selection.exclude.length ? `excluding ${selection.exclude.map(sourceLabel).join(', ')}` : '',
  ].filter(Boolean).join('; '));
</script>

<Button type="button" variant="outline" size="sm" class="shrink-0"
  aria-haspopup="dialog" aria-expanded={open}
  aria-label={`Filter by source${summary ? `: ${summary}` : ''}`}
  title={summary || 'Filter jobs by source'} onclick={onOpen}>
  Source
  {#if count > 0}
    <span class="rounded-full bg-secondary px-1.5 text-xs tabular-nums" aria-hidden="true">{count}</span>
  {/if}
</Button>
