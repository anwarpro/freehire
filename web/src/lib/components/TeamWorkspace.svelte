<script lang="ts">
  import { onMount } from 'svelte';
  import { resolve } from '$app/paths';
  import { ApiError, api } from '#lib/api.js';
  import { reviewStatuses, reviewStatusLabel } from '#lib/teamReview.js';
  import type { TeamAccess, TeamReview, TeamQueue, TeamQueueFilter } from '#lib/teamReview.js';
  import type { Job } from '#lib/types.js';
  import { Button } from '#lib/ui/index.js';
  import TeamMembersView from './TeamMembersView.svelte';
  import TeamJobDiscussion from './TeamJobDiscussion.svelte';
  import StudentFollowups from './StudentFollowups.svelte';
  import CurriculumWorkspace from './CurriculumWorkspace.svelte';
  import TeamReviewButton from './TeamReviewButton.svelte';
  let access = $state.raw<TeamAccess | null>(null);
  let view = $state<'assignments' | 'members' | 'curriculum' | 'students' | 'bot'>('assignments');
  let scope = $state<'all' | 'mine' | 'unassigned'>('mine');
  let status = $state('');
  let overdue = $state(false);
  let rows = $state.raw<TeamReview[]>([]);
  let meta = $state<TeamQueue['meta']>({ has_more: false, offset: 0, limit: 50 });
  let loading = $state(true);
  let error = $state('');
  let query = $state('');
  let jobs = $state.raw<Job[]>([]);
  let searching = $state(false);
  let searchError = $state('');
  let alive = false;
  let generation = 0;
  let searchGeneration = 0;
  const fieldClass = 'rounded-md border border-border bg-background px-3 py-2 text-sm';
  async function load(offset = 0) {
    const mine = ++generation; loading = true; error = '';
    try {
      const filter: TeamQueueFilter = { scope, status, overdue, offset };
      const result = await api.teamReviews(filter);
      if (!alive || mine !== generation) return;
      rows = result.data; meta = result.meta;
    } catch (e) {
      if (alive && mine === generation) {
        rows = [];
        if (e instanceof ApiError && (e.status === 401 || e.status === 403)) access = null;
        error = e instanceof Error ? e.message : 'Could not load assignments. Try again.';
      }
    } finally { if (alive && mine === generation) loading = false; }
  }
  async function start() {
    loading = true; error = '';
    try {
      const a = await api.teamAccess();
      if (!alive) return;
      access = a;
      if (a.role === 'none') { loading = false; return; }
      scope = a.role === 'admin' ? 'all' : 'mine';
      await load();
    } catch (e) { if (alive) { loading = false; error = e instanceof Error ? e.message : 'Could not check team access.'; } }
  }
  async function search(event: SubmitEvent) {
    event.preventDefault(); const mine = ++searchGeneration;
    searching = true; searchError = ''; jobs = [];
    try {
      const result = await api.searchJobs(new URLSearchParams({ q: query.trim() }), 10, 0);
      if (alive && mine === searchGeneration) jobs = result.items;
    } catch { if (alive && mine === searchGeneration) searchError = 'Could not search jobs. Try again.'; }
    finally { if (alive && mine === searchGeneration) searching = false; }
  }
  onMount(() => { alive = true; void start(); return () => { alive = false; generation++; searchGeneration++; }; });
</script>

<div class="space-y-6">
  <header><h1 class="text-3xl font-semibold tracking-tight">{access?.role === 'reviewer' ? 'My assignments' : 'Team workspace'}</h1><p class="mt-2 text-muted-foreground">Review jobs, record notes, and keep track of follow-ups.</p></header>
  {#if error}<div role="alert" class="text-sm text-destructive">{error}</div><Button variant="outline" onclick={start}>Retry</Button>{/if}
  {#if loading && !access}<p role="status">Checking team access…</p>{/if}
  {#if access?.role === 'none'}<p>Team access is required. Ask a team manager to add or reactivate your account.</p>{/if}
  {#if access && access.role !== 'none'}
      <nav class="flex flex-wrap gap-2" aria-label="Team workspace sections">
        <Button variant={view === 'assignments' ? 'primary' : 'outline'} onclick={() => (view = 'assignments')}>Assignments</Button>
        <Button variant={view === 'curriculum' ? 'primary' : 'outline'} onclick={() => (view = 'curriculum')}>Curriculum jobs</Button>
        <Button variant={view === 'students' ? 'primary' : 'outline'} onclick={() => (view = 'students')}>Student follow-ups</Button>
        <Button variant={view === 'bot' ? 'primary' : 'outline'} onclick={() => (view = 'bot')}>Discuss jobs</Button>
        {#if access.role === 'admin'}<Button variant={view === 'members' ? 'primary' : 'outline'} onclick={() => (view = 'members')}>Team members</Button>{/if}
      </nav>
    {#if view === 'curriculum'}<CurriculumWorkspace role={access.role} />
    {:else if view === 'bot'}<TeamJobDiscussion />
    {:else if view === 'students'}<StudentFollowups />
    {:else if view === 'members' && access.role === 'admin'}<TeamMembersView />
    {:else}
      {#if access.role === 'admin'}
        <section class="space-y-3 rounded-lg border border-border p-4" aria-label="Assign a job">
          <h2 class="font-semibold">Assign a job</h2><p class="text-sm text-muted-foreground">Find an active posting here, or use Team review on any job page, including the closed archive.</p>
          <form onsubmit={search} class="flex flex-wrap gap-2"><label class="flex-1 text-sm"><span class="sr-only">Search jobs to assign</span><input bind:value={query} class={`${fieldClass} w-full`} placeholder="Job title, company, or skill" required maxlength="200" /></label><Button type="submit" disabled={searching}>{searching ? 'Searching…' : 'Find jobs'}</Button></form>
          {#if searchError}<p role="alert" class="text-sm text-destructive">{searchError}</p>{/if}
          <ul class="space-y-2">{#each jobs as job (job.public_slug)}<li class="flex flex-wrap items-center justify-between gap-2 border-t border-border pt-2"><div><a href={resolve('/jobs/[slug]', { slug: job.public_slug })} class="font-medium underline underline-offset-4">{job.title}</a><p class="text-sm text-muted-foreground">{job.company}</p></div><TeamReviewButton slug={job.public_slug} onSaved={() => void load()} /></li>{/each}</ul>
        </section>
      {/if}
      <section class="space-y-4" aria-label="Review assignments">
        <h2 class="text-xl font-semibold">{scope === 'mine' ? 'My assignments' : scope === 'unassigned' ? 'Unassigned reviews' : 'All assignments'}</h2>
        <form class="flex flex-wrap items-end gap-3" onsubmit={(e) => { e.preventDefault(); void load(); }}>
          {#if access.role === 'admin'}<label class="space-y-1 text-sm"><span class="block">Queue</span><select bind:value={scope} class={fieldClass}><option value="all">All assignments</option><option value="mine">My assignments</option><option value="unassigned">Unassigned reviews</option></select></label>{/if}
          <label class="space-y-1 text-sm"><span class="block">Status</span><select bind:value={status} class={fieldClass}><option value="">All statuses</option>{#each reviewStatuses as option (option.value)}<option value={option.value}>{option.label}</option>{/each}</select></label>
          <label class="flex items-center gap-2 py-2 text-sm"><input type="checkbox" bind:checked={overdue} />Overdue only</label>
          <Button type="submit" variant="outline" disabled={loading}>Apply filters</Button>
          <Button variant="ghost" onclick={() => void load(meta.offset)} disabled={loading}>Refresh queue</Button>
        </form>
        {#if loading}<p role="status">Loading assignments…</p>{/if}
        {#if !loading && rows.length === 0}<p class="text-muted-foreground">No assignments match these filters.</p>{/if}
        <ul class="space-y-3">
          {#each rows as review (review.job_id)}
            <li class="rounded-lg border border-border p-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div><a href={resolve('/jobs/[slug]', { slug: review.public_slug })} class="font-semibold underline underline-offset-4">{review.title}</a><p class="text-sm text-muted-foreground">{review.company}</p></div>
                <TeamReviewButton slug={review.public_slug} onSaved={() => void load(meta.offset)} />
              </div>
              <p class="mt-2 text-sm">{reviewStatusLabel(review.status)} · {review.assignee}{review.assignee_id && !review.assignee_active ? ' (inactive)' : ''}{review.closed_at ? ' · Posting closed' : ''}</p>
              {#if review.follow_up}<p class:font-semibold={review.overdue} class="mt-1 text-sm">{review.overdue ? 'Overdue follow-up' : 'Follow-up'}: {review.follow_up}</p>{/if}
              {#if review.notes}<p class="mt-2 whitespace-pre-wrap break-words text-sm text-muted-foreground">{review.notes}</p>{/if}
            </li>
          {/each}
        </ul>
        <div class="flex items-center gap-3"><Button variant="outline" disabled={loading || meta.offset === 0} onclick={() => void load(Math.max(0, meta.offset - meta.limit))}>Previous</Button><span class="text-sm text-muted-foreground">Page {Math.floor(meta.offset / meta.limit) + 1}</span><Button variant="outline" disabled={loading || !meta.has_more} onclick={() => void load(meta.offset + meta.limit)}>Next</Button></div>
      </section>
    {/if}
  {/if}
</div>
