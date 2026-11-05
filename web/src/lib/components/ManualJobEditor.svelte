<script lang="ts">
  import { SquarePen } from '@lucide/svelte';
  import { currentUser } from '#lib/auth.svelte.js';
  import type { Job } from '#lib/types.js';
  import { Button } from '#lib/ui/index.js';

  let { job, onSaved }: { job: Job; onSaved: (updated: Job) => void } = $props();
  const allowed = $derived(currentUser()?.role === 'moderator' && job.manually_added);
  type Editor = typeof import('./ManualJobEditDialog.svelte').default;
  let editor = $state<Editor | null>(null);
  let opening = $state(false);
  let open = $state(false);
  let error = $state<string | null>(null);
  let saved = $state(false);

  async function edit() {
    if (!allowed || opening) return;
    opening = true;
    error = null;
    saved = false;
    try {
      editor = (await import('./ManualJobEditDialog.svelte')).default;
      open = true;
    } catch {
      error = 'Could not open the job editor. Please try again.';
    } finally {
      opening = false;
    }
  }

  function finish(updated: Job) {
    onSaved(updated);
    open = false;
    saved = true;
  }
</script>

{#if allowed}
  <div class="flex flex-wrap items-center gap-2">
    <Button variant="outline" size="sm" onclick={edit} disabled={opening}>
      <SquarePen class="size-3.5" aria-hidden="true" />
      {opening ? 'Opening editor…' : 'Edit job'}
    </Button>
    {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
    {#if saved}<p role="status" class="text-sm text-muted-foreground">Job updated.</p>{/if}
  </div>
  {#if open && editor}
    {@const Editor = editor}
    {#key job.public_slug}
      <Editor slug={job.public_slug} onClose={() => (open = false)} onSaved={finish} />
    {/key}
  {/if}
{/if}
