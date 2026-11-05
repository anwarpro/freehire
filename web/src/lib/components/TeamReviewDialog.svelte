<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError, api } from '#lib/api.js';
  import { reviewStatuses, reviewStatusLabel } from '#lib/teamReview.js';
  import type { TeamAccess, TeamMember, TeamReview, TeamReviewEvent, ReviewStatus } from '#lib/teamReview.js';
  import CurriculumJobReview from './CurriculumJobReview.svelte';
  import { Button, Dialog } from '#lib/ui/index.js';

  let { slug, onClose, onSaved }: { slug: string; onClose: () => void; onSaved: () => void } = $props();
  let open = $state(true);
  let current = $state.raw<TeamReview | null>(null);
  let access = $state.raw<TeamAccess | null>(null);
  let members = $state.raw<TeamMember[]>([]);
  let assignee = $state('');
  let status = $state<ReviewStatus>('pending');
  let notes = $state('');
  let followUp = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let error = $state('');
  let conflict = $state(false);
  let history = $state.raw<TeamReviewEvent[]>([]);
  let historyOpen = $state(false);
  let curriculumOpen = $state(false);
  let historyError = $state('');
  let alive = false;
  let generation = 0;
  const fieldClass = 'w-full rounded-md border border-border bg-background px-3 py-2 text-sm';

  async function load() {
    const mine = ++generation;
    loading = true;
    error = '';
    conflict = false;
    current = null;
    history = [];
    historyOpen = false;
    try {
      const a = await api.teamAccess();
      const [review, staff] = await Promise.all([api.teamReview(slug), a.role === 'admin' ? api.teamMembers() : Promise.resolve([])]);
      if (!alive || mine !== generation) return;
      access = a;
      current = review;
      members = staff;
      assignee = review.assignee_id === null ? '' : String(review.assignee_id);
      status = review.status;
      notes = review.notes;
      followUp = review.follow_up ?? '';
    } catch (e) {
      if (alive && mine === generation) error = e instanceof ApiError && e.status === 404
        ? 'This job is not assigned to you, or it is unavailable.'
        : e instanceof Error ? e.message : 'Could not load this review. Try again.';
    } finally { if (alive && mine === generation) loading = false; }
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (!current || saving || conflict) return;
    saving = true;
    error = '';
    try {
      await api.saveTeamReview(slug, { assignee_id: assignee ? Number(assignee) : null, status, notes, follow_up: followUp || null, expected_version: current.version });
      if (!alive) return;
      onSaved();
      open = false;
    } catch (e) {
      if (!alive) return;
      conflict = e instanceof ApiError && e.status === 409;
      if (e instanceof ApiError && (e.status === 401 || e.status === 403 || e.status === 404)) {
        current = null;
        notes = '';
        history = [];
      }
      error = conflict ? 'This review changed elsewhere. Your draft is kept here. Reload the latest review before saving again.'
        : e instanceof Error ? e.message : 'Could not save. Your draft is kept; try again.';
    } finally { if (alive) saving = false; }
  }

  async function showHistory() {
    historyOpen = !historyOpen;
    if (!historyOpen) return;
    historyError = '';
    try { const rows = await api.teamReviewHistory(slug); if (alive) history = rows; }
    catch { if (alive) { history = []; historyError = 'Could not load history. Close and reopen it to retry.'; } }
  }

  onMount(() => { alive = true; void load(); return () => { alive = false; generation++; }; });
  $effect(() => { if (!open) onClose(); });
</script>

<Dialog bind:open title="Team job review" dismissible={!saving} class="sm:max-w-2xl">
  <div class="space-y-5">
    <p class="text-sm text-muted-foreground">Private to team managers and the current assignee.</p>
    {#if loading}<p role="status">Loading review…</p>{/if}
    {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
    {#if !loading && !current}<Button variant="outline" onclick={load}>Retry loading</Button>{/if}
    {#if current && access}
      <div><h2 class="font-semibold">{current.title}</h2><p class="text-sm text-muted-foreground">{current.company}</p></div>
      {#if current.closed_at}<p class="text-sm text-muted-foreground">This posting is closed. Its team review remains available.</p>{/if}
      <form onsubmit={save} class="space-y-4">
        {#if access.role === 'admin'}
          <label class="block space-y-1 text-sm font-medium">Assignee
            <select bind:value={assignee} class={fieldClass} disabled={saving}>
              <option value="">Unassigned</option>
              {#each members as member (member.id)}
                <option value={String(member.id)} disabled={!member.active}>{member.name}{member.active ? '' : ' (inactive)'}</option>
              {/each}
            </select>
          </label>
          {#if current.assignee_id && !current.assignee_active}<p class="text-sm text-muted-foreground">This assignee is inactive. Choose an active member or unassign the job.</p>{/if}
        {:else}<p class="text-sm">Assigned to you: <strong>{current.assignee}</strong></p>{/if}
        <label class="block space-y-1 text-sm font-medium">Review status
          <select bind:value={status} class={fieldClass} disabled={saving}>
            {#each reviewStatuses as option (option.value)}<option value={option.value}>{option.label}</option>{/each}
          </select>
        </label>
        <label class="block space-y-1 text-sm font-medium">Review notes
          <textarea bind:value={notes} rows="5" maxlength="5000" class={fieldClass} disabled={saving}></textarea>
        </label>
        <label class="block space-y-1 text-sm font-medium">Follow-up date (optional)
          <input type="date" bind:value={followUp} class={fieldClass} disabled={saving} min="1900-01-01" max="9999-12-31" />
        </label>
        <p class="text-xs text-muted-foreground">Follow-up dates use the Bangladesh calendar. Pending and in-review jobs become overdue after that date.</p>
        <div class="flex flex-wrap gap-2">
          <Button type="submit" disabled={saving || conflict}>{saving ? 'Saving…' : 'Save review'}</Button>
          <Button variant="outline" onclick={() => (open = false)} disabled={saving}>Cancel</Button>
          {#if conflict}<Button variant="outline" onclick={load}>Reload latest (discard draft)</Button>{/if}
        </div>
      </form>
      <div class="border-t border-border pt-4">
        <Button variant="outline" onclick={() => (curriculumOpen = !curriculumOpen)}>{curriculumOpen ? 'Hide curriculum badges' : 'Curriculum badges'}</Button>
        {#if curriculumOpen}<div class="mt-4"><CurriculumJobReview {slug} {onSaved} /></div>{/if}
      </div>
      <div class="border-t border-border pt-4">
        <Button variant="ghost" onclick={showHistory} disabled={saving}>{historyOpen ? 'Hide history' : 'View change history'}</Button>
        {#if historyOpen}
          {#if historyError}<p role="alert" class="text-sm text-destructive">{historyError}</p>{/if}
          {#if history.length === 0 && !historyError}<p class="mt-2 text-sm text-muted-foreground">No changes recorded yet.</p>{/if}
          <ol class="mt-3 space-y-3">
            {#each history as event (event.id)}
              <li class="rounded-md border border-border p-3 text-sm">
                <p class="font-medium">{event.actor} · {event.action} · version {event.version}</p>
                <p class="text-muted-foreground">{event.created_at} · {event.assignee} · {reviewStatusLabel(event.status)}</p>
                {#if event.follow_up}<p>Follow-up: {event.follow_up}</p>{/if}
                {#if event.notes}<p class="mt-2 whitespace-pre-wrap break-words">{event.notes}</p>{/if}
              </li>
            {/each}
          </ol>
        {/if}
      </div>
    {/if}
  </div>
</Dialog>
