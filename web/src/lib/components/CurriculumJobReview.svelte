<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '#lib/api.js';
  import { curriculumLabel, curriculumTime } from '#lib/curriculum.js';
  import type { CurriculumDetail, CurriculumEvent } from '#lib/curriculum.js';
  import { Button } from '#lib/ui/index.js';
  import CurriculumBadges from './CurriculumBadges.svelte';
  let { slug, onSaved = () => {} }: { slug: string; onSaved?: () => void } = $props();
  let current = $state.raw<CurriculumDetail | null>(null);
  let selected = $state<string[]>([]);
  let notes = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let conflict = $state(false);
  let error = $state('');
  let notice = $state('');
  let history = $state.raw<CurriculumEvent[]>([]);
  let historyOpen = $state(false);
  let historyError = $state('');
  let alive = false;
  let generation = 0;
  function setCurrent(value: CurriculumDetail) { current = value; selected = [...(value.reviewed_tracks ?? value.suggested_tracks)]; notes = value.review_notes; }
  async function load() {
    const mine = ++generation; loading = true; error = ''; conflict = false; notice = ''; history = []; historyOpen = false;
    try { const value = await api.curriculumJob(slug); if (alive && mine === generation) setCurrent(value); }
    catch (e) { if (alive && mine === generation) { current = null; notes = ''; selected = []; error = e instanceof Error ? e.message : 'Could not load curriculum matches.'; } }
    finally { if (alive && mine === generation) loading = false; }
  }
  async function save(reset = false) {
    if (!current || saving || conflict) return;
    saving = true; error = ''; notice = '';
    try {
      const value = await api.reviewCurriculum(slug, { tracks: reset ? null : [...selected].sort(), notes: reset ? '' : notes, expected_version: current.review_version, mapping_version: current.mapping_version, input_hash: current.input_hash });
      if (!alive) return;
      setCurrent(value); historyOpen = false; history = []; notice = reset ? 'Automatic suggestions restored.' : 'Curriculum review saved.'; onSaved();
    } catch (e) {
      if (!alive) return;
      conflict = e instanceof ApiError && e.status === 409;
      if (e instanceof ApiError && [401, 403, 404].includes(e.status)) { current = null; selected = []; notes = ''; history = []; }
      error = conflict ? 'The job, mapping or review changed. Your draft is kept. Reload the latest evidence before saving again.' : e instanceof Error ? e.message : 'Could not save. Your draft is kept.';
    } finally { if (alive) saving = false; }
  }
  async function showHistory() {
    historyOpen = !historyOpen; historyError = '';
    if (!historyOpen) return;
    try { const events = await api.curriculumHistory(slug); if (alive) history = events; }
    catch (e) { if (alive) { history = []; if (e instanceof ApiError && [401, 403, 404].includes(e.status)) { current = null; notes = ''; selected = []; } historyError = 'Could not load curriculum history. Close and reopen it to retry.'; error = historyError; } }
  }
  onMount(() => { alive = true; void load(); return () => { alive = false; generation++; }; });
</script>
<section class="space-y-4 rounded-lg border border-border p-4" aria-label="Curriculum matching">
  <div><h3 class="font-semibold">Curriculum matching</h3><p class="mt-1 text-sm text-muted-foreground">Skill overlap helps reviewers find relevant jobs. It does not establish student eligibility or experience level.</p></div>
  {#if loading}<p role="status">Loading curriculum evidence…</p>{/if}
  {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
  {#if notice}<p role="status" class="text-sm">{notice}</p>{/if}
  {#if current && !loading}
    <p class="text-sm font-medium">{current.reviewed ? current.review_stale ? 'Reviewed · evidence changed; review again' : 'Reviewer labels' : 'Suggested · unreviewed'}</p>
    <CurriculumBadges ids={current.effective_tracks} tracks={current.tracks} />
    {#if current.review_stale}<p class="text-sm text-muted-foreground">The posting or curriculum mapping changed after this review. Your saved labels remain until a reviewer updates them.</p>{/if}
    <details class="rounded-md bg-muted/40 p-3" open>
      <summary class="cursor-pointer text-sm font-medium">Automatic suggestions and evidence</summary>
      {#if current.evidence.truncated}<p class="mt-2 text-sm">Only the first 50,000 description characters were analyzed. Check the complete posting before approving.</p>{/if}
      {#if current.evidence.matches.length === 0}<p class="mt-3 text-sm text-muted-foreground">No configured skill rules matched. You can still apply a reviewer label.</p>{/if}
      {#each current.evidence.matches as match (match.track_id)}
        <div class="mt-3"><h4 class="text-sm font-semibold">{curriculumLabel(match.track_id, current.tracks)}</h4>
          <ul class="mt-2 space-y-2">{#each match.evidence as evidence (evidence.skill)}<li class="text-sm"><p><strong>{evidence.skill}</strong> · {evidence.kind === 'title' ? 'Job title' : evidence.kind === 'mentioned' ? 'Mentioned; requirement strength unspecified' : evidence.kind === 'preferred' ? 'Preferred' : 'Required'} · alias “{evidence.alias}”</p><blockquote class="mt-1 break-words border-l-2 border-border pl-3 text-muted-foreground">{evidence.text}</blockquote></li>{/each}</ul>
        </div>
      {/each}
    </details>
    <form onsubmit={(e) => { e.preventDefault(); void save(); }} class="space-y-3">
      <fieldset disabled={saving || conflict} class="space-y-2"><legend class="mb-2 text-sm font-medium">Reviewer curriculum labels</legend>
        {#each current.tracks as track (track.id)}<label class="flex items-center gap-2 text-sm"><input type="checkbox" value={track.id} bind:group={selected} disabled={!track.enabled && !selected.includes(track.id)} />{track.label}{track.enabled ? '' : ' (disabled; remove to save)'}</label>{/each}
      </fieldset>
      <p class="text-xs text-muted-foreground">An empty selection records a reviewed job with no relevant track.</p>
      <label class="block text-sm font-medium">Curriculum review notes<textarea bind:value={notes} maxlength="2000" rows="3" disabled={saving || conflict} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-2 text-sm"></textarea></label>
      <div class="flex flex-wrap gap-2"><Button type="submit" disabled={saving || conflict}>{saving ? 'Saving…' : 'Save curriculum labels'}</Button><Button variant="outline" onclick={() => { if (current) selected = [...current.suggested_tracks]; }} disabled={saving || conflict}>Select suggestions</Button>{#if current.reviewed}<Button variant="ghost" onclick={() => void save(true)} disabled={saving || conflict}>Restore automatic suggestions</Button>{/if}</div>
    </form>
    <Button variant="ghost" onclick={showHistory} disabled={saving}>{historyOpen ? 'Hide curriculum history' : 'View curriculum history'}</Button>
    {#if historyOpen}
      {#if historyError}<p role="alert" class="text-sm text-destructive">{historyError}</p>{/if}
      {#if history.length === 0 && !historyError}<p class="text-sm text-muted-foreground">No curriculum reviews recorded yet.</p>{/if}
      <ol class="space-y-2">{#each history as event (event.id)}<li class="rounded-md border border-border p-3 text-sm"><p class="font-medium">{event.actor} · version {event.version}</p><p class="text-xs text-muted-foreground">{curriculumTime(event.created_at)} (Bangladesh) · mapping {event.mapping_version}</p><p class="mt-1">{event.tracks === null ? 'Restored automatic suggestions' : event.tracks.length ? event.tracks.map((id) => curriculumLabel(id, current?.tracks ?? [])).join(', ') : 'Reviewed — no matching track'}</p>{#if event.notes}<p class="mt-1 whitespace-pre-wrap break-words">{event.notes}</p>{/if}</li>{/each}</ol>
    {/if}
  {/if}
  {#if conflict || (!loading && !current)}<Button variant="outline" onclick={load} disabled={saving}>{conflict ? 'Reload curriculum (discard draft)' : 'Retry curriculum'}</Button>{/if}
</section>
