<script lang="ts">
  import { onMount } from 'svelte';
  import { resolve } from '$app/paths';
  import { api, ApiError } from '#lib/api.js';
  import { studentStages } from '#lib/studentFollowups.js';
  import type { StudentFollowup, StudentFollowupQueue } from '#lib/studentFollowups.js';
  import { Button } from '#lib/ui/index.js';
  import StudentFollowupEditor from './StudentFollowupEditor.svelte';
  let { slug, closed = false }: { slug?: string; closed?: boolean } = $props();
  let rows = $state.raw<StudentFollowup[]>([]); let meta = $state<StudentFollowupQueue['meta']>({ has_more: false, limit: 50, offset: 0 });
  let query = $state(''); let status = $state(''); let overdue = $state(false); let loading = $state(true); let error = $state('');
  let selected = $state<{ slug: string; id: number } | null>(null); let unavailable = $state(false); let alive = false; let generation = 0;
  const field = 'w-full min-w-0 rounded-md border border-border bg-background px-3 py-2 text-sm';
  async function load(offset = 0) {
    const mine = ++generation; loading = true; error = '';
    try { const result = await api.studentFollowups({ q: query.trim(), status, overdue, offset }, slug); if (alive && mine === generation) { rows = result.data; meta = result.meta; unavailable = false; } }
    catch (e) { if (alive && mine === generation) { rows = []; if (e instanceof ApiError && [401, 403, 404].includes(e.status)) { unavailable = true; selected = null; } error = e instanceof Error ? e.message : 'Could not load student follow-ups.'; } }
    finally { if (alive && mine === generation) loading = false; }
  }
  onMount(() => { alive = true; void load(); return () => { alive = false; generation++; }; });
</script>
<section class="min-w-0 space-y-4" aria-label="Student follow-ups">
  <h2 class="text-xl font-semibold">Student follow-ups</h2>
  <p class="text-sm text-muted-foreground">References to students in your school system, linked to individual jobs. Private to managers and each job’s current assignee.</p>
  {#if error}<p role="alert" class="break-words text-sm text-destructive">{error}</p><Button variant="outline" onclick={() => void load(meta.offset)}>Retry student follow-ups</Button>{/if}
  {#if selected && !unavailable}
    {#key `${selected.slug}:${selected.id}`}<StudentFollowupEditor slug={selected.slug} id={selected.id} onClose={() => (selected = null)} onSaved={() => void load(meta.offset)} onAccessLost={() => { rows = []; selected = null; unavailable = true; error = 'Access to this student follow-up changed. Refresh to check current access.'; }} />{/key}
  {:else if !unavailable}
    {#if slug}<Button onclick={() => (selected = { slug, id: 0 })} disabled={closed || loading}>Link a student</Button>{#if closed}<p class="text-sm text-muted-foreground">New links and contacts are unavailable for this closed posting. Existing outcomes remain editable.</p>{/if}
    {:else}<p class="text-sm text-muted-foreground">To add a student, open Team review on a job and choose Student follow-ups.</p>{/if}
    <form onsubmit={(e) => { e.preventDefault(); void load(); }} class="flex min-w-0 flex-wrap items-end gap-3">
      <label class="min-w-0 flex-1 space-y-1 text-sm"><span class="block">Student reference or name</span><input bind:value={query} maxlength="300" class={field} /></label>
      <label class="space-y-1 text-sm"><span class="block">Outcome filter</span><select bind:value={status} class={field}><option value="">All outcomes</option>{#each studentStages as stage (stage)}<option value={stage}>{stage.charAt(0).toUpperCase() + stage.slice(1)}</option>{/each}</select></label>
      <label class="flex items-center gap-2 py-2 text-sm"><input type="checkbox" bind:checked={overdue} />Overdue student follow-ups only</label>
      <Button type="submit" variant="outline" disabled={loading}>Filter students</Button><Button variant="ghost" onclick={() => void load(meta.offset)} disabled={loading}>Refresh students</Button>
    </form>
    {#if loading}<p role="status">Loading student follow-ups…</p>{/if}
    {#if !loading && rows.length === 0}<p class="text-sm text-muted-foreground">No student follow-ups match these filters.</p>{/if}
    <ul class="space-y-3">{#each rows as row (row.id)}<li class="min-w-0 space-y-2 rounded-lg border border-border p-4"><div class="flex min-w-0 flex-wrap items-start justify-between gap-3"><div class="min-w-0"><p class="break-words font-semibold">{row.student_name || row.student_ref}</p>{#if row.student_name}<p class="break-words text-sm">{row.student_ref}</p>{/if}<a href={resolve('/jobs/[slug]', { slug: row.public_slug })} class="break-words text-sm underline">{row.title} · {row.company}</a></div><Button variant="outline" onclick={() => (selected = { slug: row.public_slug, id: row.id })} aria-label={`Follow up with ${row.student_ref}`}>Open follow-up</Button></div><p class="text-sm capitalize">{row.status} · {row.contact_count} recorded contacts{row.closed_at ? ' · Posting closed' : ''}</p><p class="break-words text-sm">Staff assignee: {row.assignee}</p><p class="whitespace-pre-wrap break-words text-sm text-muted-foreground">{row.reason}</p>{#if row.follow_up}<p class="text-sm" class:font-semibold={row.overdue}>{row.overdue ? 'Overdue' : 'Next follow-up'}: {row.follow_up}</p>{/if}</li>{/each}</ul>
    <div class="flex flex-wrap items-center gap-3"><Button variant="outline" disabled={loading || meta.offset === 0} onclick={() => void load(Math.max(0, meta.offset - meta.limit))}>Previous</Button><span class="text-sm">Page {Math.floor(meta.offset / meta.limit) + 1}</span><Button variant="outline" disabled={loading || !meta.has_more} onclick={() => void load(meta.offset + meta.limit)}>Next</Button></div>
  {/if}
</section>
