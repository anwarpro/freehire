<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '#lib/api.js';
  import type { TeamMember, TeamMemberEvent } from '#lib/teamReview.js';
  import { Button, Dialog } from '#lib/ui/index.js';
  let members = $state.raw<TeamMember[]>([]);
  let loading = $state(true);
  let error = $state('');
  let success = $state('');
  let name = $state('');
  let email = $state('');
  let password = $state('');
  let useExisting = $state(false);
  let busy = $state(false);
  let selected = $state.raw<TeamMember | null>(null);
  let editName = $state('');
  let editActive = $state(true);
  let editOpen = $state(false);
  let editError = $state('');
  let events = $state.raw<TeamMemberEvent[]>([]);
  let historyOpen = $state(false);
  let alive = false;
  let generation = 0;
  const fieldClass = 'w-full rounded-md border border-border bg-background px-3 py-2 text-sm';
  async function load() {
    const mine = ++generation; loading = true; error = '';
    try { const rows = await api.teamMembers(); if (alive && mine === generation) members = rows; }
    catch (e) { if (alive && mine === generation) { members = []; error = e instanceof Error ? e.message : 'Could not load team members.'; } }
    finally { if (alive && mine === generation) loading = false; }
  }
  async function create(event: SubmitEvent) {
    event.preventDefault(); if (busy) return;
    busy = true; error = ''; success = '';
    try {
      await api.createTeamMember({ name: name.trim(), email: email.trim(), use_existing: useExisting, ...(useExisting ? {} : { password }) });
      if (!alive) return;
      name = ''; email = ''; password = '';
      success = useExisting ? 'Reviewer added to the team.' : 'Reviewer created. Share their email and initial password privately; they can sign in and change it under Security.';
      await load();
    } catch (e) { if (alive) error = e instanceof Error ? e.message : 'Could not create the reviewer. Check the details and retry.'; }
    finally { if (alive) busy = false; }
  }
  function edit(member: TeamMember) {
    selected = member; editName = member.name; editActive = member.active;
    editError = ''; events = []; historyOpen = false; editOpen = true;
  }
  async function save(event: SubmitEvent) {
    event.preventDefault(); if (!selected || busy) return;
    busy = true; editError = '';
    try {
      await api.updateTeamMember(selected.id, { name: editName.trim(), active: editActive, expected_version: selected.version });
      if (!alive) return;
      editOpen = false; success = 'Team access updated.'; await load();
    } catch (e) { if (alive) editError = e instanceof Error ? e.message : 'Could not update this member.'; }
    finally { if (alive) busy = false; }
  }
  async function showHistory() {
    if (!selected) return;
    const id = selected.id;
    historyOpen = true; editError = '';
    try { const rows = await api.teamMemberHistory(id); if (alive && selected?.id === id) events = rows; }
    catch { if (alive && selected?.id === id) editError = 'Could not load member history. Try again.'; }
  }
  onMount(() => { alive = true; void load(); return () => { alive = false; generation++; }; });
</script>

<section class="space-y-6" aria-label="Team members">
  <div><h2 class="text-xl font-semibold">Team members</h2><p class="mt-1 text-sm text-muted-foreground">Existing moderators and administrators manage the team. Reviewers can work only on their own assignments.</p></div>
  {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
  {#if success}<p role="status" class="text-sm">{success}</p>{/if}
  <form onsubmit={create} class="space-y-4 rounded-lg border border-border p-4">
    <h3 class="font-semibold">Add a reviewer</h3>
    <div class="grid gap-4 sm:grid-cols-2">
      <label class="block space-y-1 text-sm font-medium">Name<input bind:value={name} required maxlength="120" class={fieldClass} disabled={busy} autocomplete="off" /></label>
      <label class="block space-y-1 text-sm font-medium">Email<input type="email" bind:value={email} required class={fieldClass} disabled={busy} autocomplete="off" /></label>
    </div>
    <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={useExisting} disabled={busy} onchange={() => (password = '')} />Use an existing verified account</label>
    {#if useExisting}<p class="text-sm text-muted-foreground">The existing account keeps its password. Its email must already be verified.</p>
    {:else}
      <label class="block space-y-1 text-sm font-medium">Initial password<input type="password" bind:value={password} required minlength="8" maxlength="72" class={fieldClass} disabled={busy} autocomplete="new-password" /></label>
      <p class="text-sm text-muted-foreground">8–72 bytes. Share it privately with the reviewer. No email invitation is sent.</p>
    {/if}
    <Button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Add reviewer'}</Button>
  </form>
  <div class="flex items-center justify-between"><h3 class="font-semibold">Current team</h3><Button variant="outline" onclick={load} disabled={loading || busy}>Refresh members</Button></div>
  {#if loading}<p role="status">Loading team…</p>{/if}
  <ul class="space-y-2">
    {#each members as member (member.id)}
      <li class="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border p-3">
        <div><p class="font-medium">{member.name} <span class="text-xs text-muted-foreground">{member.team_role === 'admin' ? 'Team manager' : member.active ? 'Reviewer' : 'Inactive reviewer'}</span></p>
          <p class="text-sm text-muted-foreground">{member.email} · {member.assignment_count} assignment{member.assignment_count === 1 ? '' : 's'}</p></div>
        {#if member.team_role !== 'admin'}<Button size="sm" variant="outline" onclick={() => edit(member)} disabled={busy}>Manage {member.name}</Button>{/if}
      </li>
    {/each}
  </ul>
  <Dialog bind:open={editOpen} title="Manage reviewer" dismissible={!busy} class="sm:max-w-xl">
    {#if selected}
      <form onsubmit={save} class="space-y-4">
        <p class="text-sm text-muted-foreground">{selected.email}</p>
        {#if editError}<p role="alert" class="text-sm text-destructive">{editError}</p>{/if}
        <label class="block space-y-1 text-sm font-medium">Reviewer name<input bind:value={editName} required maxlength="120" class={fieldClass} disabled={busy} /></label>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={editActive} disabled={busy} />Team access active</label>
        {#if !editActive}<p class="text-sm text-muted-foreground">Deactivation immediately removes team access. Their {selected.assignment_count} assignment{selected.assignment_count === 1 ? '' : 's'} and history remain available to managers for reassignment.</p>{/if}
        <div class="flex flex-wrap gap-2"><Button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save member'}</Button><Button variant="outline" onclick={() => (editOpen = false)} disabled={busy}>Cancel</Button><Button variant="ghost" onclick={showHistory} disabled={busy}>Member history</Button></div>
      </form>
      {#if historyOpen}<ol class="mt-4 space-y-2 text-sm">{#each events as event (event.id)}<li>{event.created_at} · {event.actor} · {event.action}</li>{/each}</ol>{/if}
    {/if}
  </Dialog>
</section>
