<script lang="ts">
  import { onMount } from 'svelte';
  import { resolve } from '$app/paths';
  import { api, ApiError } from '#lib/api.js';
  import type { CurriculumSettings, CurriculumQueue, CurriculumRow } from '#lib/curriculum.js';
  import { curriculumTime } from '#lib/curriculum.js';
  import { sourceLabel } from '#lib/facets.js';
  import type { TeamRole } from '#lib/teamReview.js';
  import { Button } from '#lib/ui/index.js';
  import CurriculumBadges from './CurriculumBadges.svelte';
  import CurriculumSettingsEditor from './CurriculumSettingsEditor.svelte';
  import TeamReviewButton from './TeamReviewButton.svelte';
  let { role }: { role: TeamRole } = $props();
  let settings = $state.raw<CurriculumSettings | null>(null);
  let rows = $state.raw<CurriculumRow[]>([]);
  let meta = $state<CurriculumQueue['meta']>({ has_more: false, offset: 0, limit: 50 });
  let track = $state('');
  let source = $state('');
  let sourceOptions = $state<string[]>(['linkedin', 'bdjobs', 'careerpage']);
  let postingState = $state('active');
  let review = $state('');
  let error = $state('');
  let notice = $state('');
  let loading = $state(true);
  let editing = $state(false);
  let denied = $state(false);
  let alive = false;
  let generation = 0;
  const field = 'max-w-full rounded-md border border-border bg-background px-3 py-2 text-sm';
  async function load(offset = 0) {
    const mine = ++generation; loading = true; error = '';
    try {
      const [config, result] = await Promise.all([api.curriculumSettings(), api.curriculumJobs({ track, source, state: postingState, review, offset })]);
      if (!alive || mine !== generation) return;
      settings = config; rows = result.data; meta = result.meta; denied = false;
    } catch (e) {
      if (alive && mine === generation) {
        rows = []; denied = e instanceof ApiError && [401, 403].includes(e.status);
        if (denied) { settings = null; editing = false; }
        error = e instanceof Error ? e.message : 'Could not load curriculum jobs. Try again.';
      }
    } finally { if (alive && mine === generation) loading = false; }
  }
  function saved(value: CurriculumSettings) { settings = value; editing = false; notice = 'Mapping saved. Background matching will refresh the catalogue in batches.'; void load(); }
  onMount(() => { alive = true; void load(); void api.listSources().then((values) => { if (alive) sourceOptions = values.map((s) => s.source).sort((a, b) => sourceLabel(a).localeCompare(sourceLabel(b))); }).catch(() => {}); return () => { alive = false; generation++; }; });
</script>
<section class="space-y-5" aria-label="Curriculum jobs">
  <header><h2 class="text-xl font-semibold">Jobs by curriculum</h2><p class="mt-1 text-sm text-muted-foreground">{role === 'reviewer' ? 'Curriculum matches for jobs assigned to you.' : 'Browse collected jobs by taught-skill overlap and reviewer labels.'} Suggestions are unreviewed until a staff member confirms them.</p></header>
  {#if error}<p role="alert" class="text-sm text-destructive">{error}</p><Button variant="outline" onclick={() => void load()}>Retry curriculum jobs</Button>{/if}
  {#if notice}<p role="status" class="text-sm">{notice}</p>{/if}
  {#if settings}
    <div class="rounded-lg bg-muted/50 p-3 text-sm">
      <p>Mapping {settings.version} · {settings.progress.complete ? 'Initial scan complete' : 'Matching catalogue in batches'} · {settings.progress.processed} matching checks recorded</p>
      <p class="mt-1 text-muted-foreground">{settings.progress.last_run_at ? `Last worker run: ${curriculumTime(settings.progress.last_run_at)} (Bangladesh)` : 'Waiting for the matching worker. Jobs become filterable as processing finishes.'} Changed job descriptions are queued automatically.</p>
      {#if settings.progress.last_error}<p role="alert" class="mt-1 text-destructive">{settings.progress.last_error}</p>{/if}
    </div>
    {#if role === 'admin'}<Button variant="outline" onclick={() => (editing = !editing)}>{editing ? 'Close mapping settings' : 'Edit curriculum mapping'}</Button>{/if}
    {#if editing && role === 'admin'}{#key settings.version}<CurriculumSettingsEditor {settings} onSaved={saved} onReload={() => void load(meta.offset)} />{/key}{/if}
    <form class="flex flex-wrap items-end gap-3" onsubmit={(e) => { e.preventDefault(); void load(); }}>
      <label class="space-y-1 text-sm"><span class="block">Curriculum track</span><select bind:value={track} class={field}><option value="">All tracks and unmatched jobs</option>{#each settings.config.tracks as t (t.id)}<option value={t.id}>{t.label}{t.enabled ? '' : ' (disabled)'}</option>{/each}</select></label>
      <label class="space-y-1 text-sm"><span class="block">Curriculum review</span><select bind:value={review} class={field}><option value="">All review states</option><option value="unreviewed">Unreviewed suggestions</option><option value="reviewed">Reviewed labels</option><option value="needs_review">Needs another review</option></select></label>
      <label class="space-y-1 text-sm"><span class="block">Job source</span><select bind:value={source} class={field}><option value="">All sources</option>{#each sourceOptions as option (option)}<option value={option}>{option === 'careerpage' ? 'Custom company career pages' : sourceLabel(option)}</option>{/each}</select></label>
      <label class="space-y-1 text-sm"><span class="block">Posting state</span><select bind:value={postingState} class={field}><option value="active">Active</option><option value="closed">Closed</option><option value="all">Active and closed</option></select></label>
      <Button type="submit" variant="outline" disabled={loading}>Filter curriculum jobs</Button><Button variant="ghost" onclick={() => void load(meta.offset)} disabled={loading}>Refresh curriculum</Button>
    </form>
    <p class="text-xs text-muted-foreground">Track filters use reviewer labels when present and automatic suggestions otherwise. Opening Curriculum badges in Team review refreshes that job immediately.</p>
  {/if}
  {#if loading}<p role="status">Loading curriculum jobs…</p>{/if}
  {#if !loading && !denied && rows.length === 0}<p class="text-muted-foreground">No processed jobs match these filters.{settings && !settings.progress.complete ? ' Matching is still in progress; refresh to see newly processed jobs.' : ''}</p>{/if}
  <ul class="space-y-3">{#each rows as row (row.job_id)}<li class="space-y-3 rounded-lg border border-border p-4">
    <div class="flex flex-wrap items-start justify-between gap-3"><div class="min-w-0 flex-1 break-words"><a href={resolve('/jobs/[slug]', { slug: row.public_slug })} class="font-semibold underline underline-offset-4">{row.title}</a><p class="text-sm text-muted-foreground">{row.company} · {row.location} · {row.source}{row.closed_at ? ' · Posting closed' : ''}</p></div><TeamReviewButton slug={row.public_slug} onSaved={() => void load(meta.offset)} /></div>
    {#if settings}<CurriculumBadges ids={row.effective_tracks} tracks={settings.config.tracks} />{/if}
    <p class="text-xs text-muted-foreground">{row.reviewed ? row.review_stale ? 'Reviewed · evidence changed; review again' : 'Reviewer labels' : 'Suggested · unreviewed'}{row.update_pending ? ' · Posting update queued; open the review for current evidence' : ''}</p>
  </li>{/each}</ul>
  {#if settings}<div class="flex items-center gap-3"><Button variant="outline" disabled={loading || meta.offset === 0} onclick={() => void load(Math.max(0, meta.offset - meta.limit))} aria-label="Previous curriculum page">Previous</Button><span class="text-sm text-muted-foreground">Page {Math.floor(meta.offset / meta.limit) + 1}</span><Button variant="outline" disabled={loading || !meta.has_more} onclick={() => void load(meta.offset + meta.limit)} aria-label="Next curriculum page">Next</Button></div>{/if}
</section>
