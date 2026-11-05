<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '#lib/api.js';
  import { formatSkillRules, parseSkillRules, curriculumTime } from '#lib/curriculum.js';
  import type { CurriculumSettings, CurriculumConfigEvent } from '#lib/curriculum.js';
  import { Button } from '#lib/ui/index.js';
  let { settings, onSaved, onReload }: { settings: CurriculumSettings; onSaved: (value: CurriculumSettings) => void; onReload: () => void } = $props();
  type Draft = { id: string; label: string; enabled: boolean; minimum: number; rules: string };
  let drafts = $state<Draft[]>([]);
  let saving = $state(false);
  let error = $state('');
  let conflict = $state(false);
  let history = $state.raw<CurriculumConfigEvent[]>([]);
  let historyOpen = $state(false);
  let historyError = $state('');
  let alive = false;
  const field = 'w-full rounded-md border border-border bg-background px-3 py-2 text-sm';
  onMount(() => { alive = true; drafts = settings.config.tracks.map((t) => ({ ...t, rules: formatSkillRules(t.skills) })); return () => { alive = false; }; });
  async function save(event: SubmitEvent) {
    event.preventDefault(); if (saving || conflict) return;
    saving = true; error = '';
    try {
      const config = { tracks: drafts.map((d) => ({ id: d.id, label: d.label, enabled: d.enabled, minimum: d.minimum, skills: parseSkillRules(d.rules) })) };
      const value = await api.saveCurriculumSettings({ expected_version: settings.version, config });
      if (alive) onSaved(value);
    } catch (e) {
      if (alive) { conflict = e instanceof ApiError && e.status === 409; error = conflict ? 'The mapping changed elsewhere. Your draft is kept. Reload the latest mapping before saving again.' : e instanceof Error ? e.message : 'Could not save the curriculum mapping.'; }
    } finally { if (alive) saving = false; }
  }
  async function showHistory() {
    historyOpen = !historyOpen; historyError = '';
    if (!historyOpen) return;
    try { const rows = await api.curriculumConfigHistory(); if (alive) history = rows; }
    catch { if (alive) { history = []; historyError = 'Could not load mapping history. Close and reopen it to retry.'; } }
  }
</script>
<section class="space-y-4 rounded-lg border border-border p-4" aria-label="Curriculum mapping settings">
  <div><h2 class="text-lg font-semibold">Curriculum mapping</h2><p class="mt-1 text-sm text-muted-foreground">Managers can edit the taught-skill rules. Saving a changed mapping queues a catalogue refresh and marks earlier reviews for reconsideration. Stable track IDs preserve history; disable a track to retire it.</p></div>
  {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
  <form onsubmit={save} class="space-y-5">
    <fieldset disabled={saving || conflict} class="space-y-5">
      <legend class="sr-only">Track rules</legend>
      {#each drafts as draft, i (i)}
        <div class="space-y-3 border-t border-border pt-4">
          <div class="grid gap-3 sm:grid-cols-2"><label class="text-sm font-medium">Track ID<input bind:value={draft.id} readonly={i < settings.config.tracks.length} required pattern={'[a-z][a-z0-9_-]{1,39}'} class={field} /></label><label class="text-sm font-medium">Badge label<input bind:value={draft.label} required maxlength="100" class={field} /></label></div>
          <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={draft.enabled} />Enable {draft.label || 'this track'}</label>
          <label class="block text-sm font-medium">Skill rules for {draft.label || draft.id}<textarea bind:value={draft.rules} rows="6" required spellcheck="false" class={field}></textarea><span class="mt-1 block text-xs font-normal text-muted-foreground">One skill per line: Skill name | alias, alias. Up to 30 skills and eight aliases per skill. Aliases match literal words or phrases.</span></label>
          <label class="block max-w-sm text-sm font-medium">Distinct skills needed for a suggestion<input type="number" min="1" max="30" required bind:value={draft.minimum} class={field} /></label>
          {#if i >= settings.config.tracks.length}<Button variant="ghost" onclick={() => (drafts = drafts.filter((_, index) => index !== i))}>Remove unsaved track</Button>{/if}
        </div>
      {/each}
      <Button variant="outline" disabled={drafts.length >= 12} onclick={() => { drafts = [...drafts, { id: `track-${drafts.length + 1}`, label: '', enabled: true, minimum: 1, rules: '' }]; }}>Add track</Button>
    </fieldset>
    <div class="flex flex-wrap gap-2"><Button type="submit" disabled={saving || conflict}>{saving ? 'Saving mapping…' : 'Save curriculum mapping'}</Button>{#if conflict}<Button variant="outline" onclick={onReload}>Reload mapping (discard draft)</Button>{/if}</div>
  </form>
  <Button variant="ghost" onclick={showHistory} disabled={saving}>{historyOpen ? 'Hide mapping history' : 'View mapping history'}</Button>
  {#if historyOpen}
    {#if historyError}<p role="alert" class="text-sm text-destructive">{historyError}</p>{/if}
    {#if history.length === 0 && !historyError}<p class="text-sm text-muted-foreground">No mapping edits yet. The initial defaults were approved for this workspace.</p>{/if}
    <ol class="space-y-2">{#each history as event (event.version)}<li class="rounded-md border border-border p-3 text-sm"><p>{event.actor} · mapping {event.version} · {curriculumTime(event.created_at)} (Bangladesh)</p><details class="mt-2"><summary class="cursor-pointer">Saved track rules</summary><pre class="mt-2 overflow-auto whitespace-pre-wrap break-words text-xs">{event.payload.tracks.map((t) => `${t.label} (${t.enabled ? 'enabled' : 'disabled'}; minimum ${t.minimum})\n${formatSkillRules(t.skills)}`).join('\n\n')}</pre></details></li>{/each}</ol>
  {/if}
</section>
