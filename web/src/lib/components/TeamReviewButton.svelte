<script lang="ts">
  import { Button } from '#lib/ui/index.js';
  let { slug, onSaved = () => {} }: { slug: string; onSaved?: () => void } = $props();
  type Editor = typeof import('./TeamReviewDialog.svelte').default;
  let editor = $state<Editor | null>(null);
  let open = $state(false);
  let loading = $state(false);
  let error = $state('');
  async function edit() {
    loading = true; error = '';
    try { editor = (await import('./TeamReviewDialog.svelte')).default; open = true; }
    catch { error = 'Could not open the team review. Try again.'; }
    finally { loading = false; }
  }
</script>
<Button size="sm" variant="outline" onclick={edit} disabled={loading}>{loading ? 'Opening…' : 'Team review'}</Button>
{#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
{#if open && editor}
  {@const Editor = editor}
  <Editor {slug} onClose={() => (open = false)} {onSaved} />
{/if}
