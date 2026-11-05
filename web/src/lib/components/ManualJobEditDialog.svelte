<script lang="ts">
  import { onMount } from 'svelte';
  import { marked } from 'marked';
  import { ApiError, api } from '#lib/api.js';
  import { currentUser } from '#lib/auth.svelte.js';
  import { htmlToMarkdown } from '#lib/htmlToMarkdown.js';
  import { renderMarkdown } from '#lib/markdown.js';
  import { postingDateError, postingDateTimestamp } from '#lib/postingDate.js';
  import type { Job, ManualJobUpdate } from '#lib/types.js';
  import { Button, Dialog, Input } from '#lib/ui/index.js';
  import NoteEditor from './NoteEditor.svelte';
  import JobDescription from './JobDescription.svelte';
  import PostingDateField from './PostingDateField.svelte';

  let { slug, onClose, onSaved }: {
    slug: string;
    onClose: () => void;
    onSaved: (updated: Job) => void;
  } = $props();

  let open = $state(true);
  let current = $state.raw<Job | null>(null);
  let loadError = $state<string | null>(null);
  let error = $state<string | null>(null);
  let saving = $state(false);
  let title = $state('');
  let company = $state('');
  let location = $state('');
  let remote = $state(false);
  let description = $state('');
  let originalDescription = $state('');
  let preview = $state(false);
  let postingDate = $state('');
  let alive = false;
  const allowed = $derived(currentUser()?.role === 'moderator');
  const dateError = $derived(postingDateError(postingDate));
  const selectedPostedAt = $derived(dateError ? undefined : postingDateTimestamp(postingDate));
  const dateChanged = $derived(selectedPostedAt !== undefined &&
    (!current?.posted_at || Date.parse(selectedPostedAt) !== Date.parse(current.posted_at)));
  const changed = $derived(current !== null && (
    title.trim() !== current.title || company.trim() !== current.company ||
    location.trim() !== current.location || remote !== (current.work_mode === 'remote') ||
    description !== originalDescription || dateChanged
  ));
  const canSave = $derived(allowed && changed && !dateError && title.trim() !== '' && company.trim() !== '' && !saving);

  $effect(() => {
    if (!open) onClose();
  });

  onMount(() => {
    alive = true;
    if (allowed) {
      api.getJob(slug).then((job) => {
        if (!alive) return;
        if (!job.manually_added) {
          loadError = 'This job is collected automatically and cannot be edited here.';
          return;
        }
        current = job;
        title = job.title;
        company = job.company;
        location = job.location ?? '';
        remote = job.work_mode === 'remote';
        originalDescription = htmlToMarkdown(job.description ?? '');
        description = originalDescription;
      }).catch((err: unknown) => {
        if (alive) loadError = err instanceof ApiError ? err.message : 'Could not load this job. Close the editor and try again.';
      });
    }
    return () => { alive = false; };
  });

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (!current || !canSave) return;
    saving = true;
    error = null;
    let updated: Job;
    try {
      // PATCH only changed fields: an untouched HTML description must not be
      // rewritten through a Markdown round trip, or another field silently reset.
      const patch: ManualJobUpdate = {};
      if (title.trim() !== current.title) patch.title = title.trim();
      if (company.trim() !== current.company) patch.company = company.trim();
      if (location.trim() !== current.location) patch.location = location.trim();
      if (remote !== (current.work_mode === 'remote')) patch.remote = remote;
      if (dateChanged) patch.posted_at = selectedPostedAt;
      if (description !== originalDescription) patch.description = await marked.parse(description);
      updated = await api.updateManualJob(current.public_slug, patch);
    } catch (err) {
      if (!alive) return;
      error = err instanceof ApiError ? err.message : 'Could not save the job. Your edits are still here; please try again.';
      saving = false;
      return;
    }
    if (!alive) return;
    saving = false;
    onSaved(updated);
  }
</script>

<Dialog bind:open title="Edit job" dismissible={!saving} class="sm:max-w-2xl">
  {#if !allowed}
    <p class="text-sm text-muted-foreground">Only moderators can edit jobs.</p>
  {:else if loadError}
    <p role="alert" class="text-sm text-destructive">{loadError}</p>
    <Button variant="secondary" onclick={() => (open = false)}>Close</Button>
  {:else if !current}
    <p role="status" class="text-sm text-muted-foreground">Loading the latest job…</p>
  {:else}
    <form onsubmit={save} class="flex flex-col gap-4">
      <p class="text-sm text-muted-foreground">
        Update the job content. Search tags are recalculated from the saved text and location.
        Editing a closed job keeps it closed.
      </p>
      <fieldset disabled={saving} inert={saving} class="flex flex-col gap-4">
        <legend class="sr-only">Job content</legend>
        <label class="flex flex-col gap-1 text-sm font-medium">
          Original URL
          <Input value={current.url} readonly class="w-full" />
        </label>
        <p class="text-xs text-muted-foreground">Source: {current.source}. The original URL and source stay fixed.</p>
        <label class="flex flex-col gap-1 text-sm font-medium">
          Title
          <Input bind:value={title} required class="w-full" />
        </label>
        <label class="flex flex-col gap-1 text-sm font-medium">
          Company
          <Input bind:value={company} required class="w-full" />
        </label>
        <label class="flex flex-col gap-1 text-sm font-medium">
          Location
          <Input bind:value={location} class="w-full" />
        </label>
        {#if current.posted_at}
          <p class="text-xs text-muted-foreground">
            Current displayed posting date:
            {new Intl.DateTimeFormat('en-GB', { dateStyle: 'long', timeZone: 'Asia/Dhaka' }).format(new Date(current.posted_at))}.
          </p>
        {/if}
        <PostingDateField bind:value={postingDate} label="New posting date" help="Optional: select a replacement date, or leave blank to keep the current date." />
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={remote} /> Fully remote
        </label>
        <p class="text-xs text-muted-foreground">
          When changing remote eligibility, also update any conflicting wording in the location or description.
        </p>
        <div class="flex items-center justify-between gap-2">
          <span class="text-sm font-medium">Description</span>
          <Button type="button" variant="ghost" size="sm" onclick={() => (preview = !preview)}>
            {preview ? 'Edit description' : 'Preview description'}
          </Button>
        </div>
        {#if preview}
          <JobDescription html={renderMarkdown(description)} />
        {:else}
          <NoteEditor value={description} onsave={(value) => (description = value)} label="Description" />
        {/if}
      </fieldset>
      {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
      <div class="flex justify-end gap-2">
        <Button variant="ghost" disabled={saving} onclick={() => (open = false)}>Cancel</Button>
        <Button type="submit" variant="primary" disabled={!canSave}>{saving ? 'Saving…' : 'Save changes'}</Button>
      </div>
    </form>
  {/if}
</Dialog>
