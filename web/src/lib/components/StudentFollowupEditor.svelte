<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '#lib/api.js';
  import { Button } from '#lib/ui/index.js';
  import { studentStages, contactChannels, studentMessage, safeStudentLink, bangladeshDateTime, bangladeshTime } from '#lib/studentFollowups.js';
  import type { StudentFollowup, StudentStage, ContactChannel, StudentContactInput, StudentFollowupEvent } from '#lib/studentFollowups.js';
  let { slug, id, onClose, onSaved, onAccessLost }: { slug: string; id: number; onClose: () => void; onSaved: () => void; onAccessLost?: () => void } = $props();
  let row = $state.raw<StudentFollowup | null>(null);
  let studentRef = $state(''); let studentName = $state(''); let profileURL = $state(''); let reason = $state('');
  let status = $state<StudentStage>('suggested'); let notes = $state(''); let followUp = $state('');
  let channel = $state<ContactChannel>('whatsapp'); let contactedAt = $state(''); let contactNote = $state('');
  const outcomeDirty = $derived(Boolean(row && (studentName.trim() !== row.student_name || profileURL.trim() !== row.profile_url || reason.trim() !== row.reason || status !== row.status || notes !== row.notes || followUp !== (row.follow_up ?? ''))));
  let pendingContact: StudentContactInput | null = null;
  let loading = $state(false); let saving = $state(false); let conflict = $state(false); let error = $state(''); let unavailable = $state(false);
  let message = $state(''); let copyStatus = $state(''); let history = $state.raw<StudentFollowupEvent[]>([]); let historyOpen = $state(false);
  let historyError = $state(''); let alive = false; let generation = 0;
  const field = 'w-full min-w-0 rounded-md border border-border bg-background px-3 py-2 text-sm';
  function failed(e: unknown) {
    if (!alive) return;
    conflict = e instanceof ApiError && e.status === 409;
    if (e instanceof ApiError && [401, 403, 404].includes(e.status)) {
      unavailable = true; row = null; studentRef = ''; studentName = ''; profileURL = ''; reason = ''; notes = ''; contactNote = ''; message = ''; history = []; pendingContact = null; onAccessLost?.();
    }
    error = conflict ? 'This follow-up changed elsewhere. Your draft is kept. Reload before saving again.' : e instanceof Error ? e.message : 'Could not save. Your draft is kept; try again.';
  }
  async function load() {
    const mine = ++generation; loading = true; error = ''; conflict = false; unavailable = false; history = []; historyOpen = false;
    try {
      const result = await api.studentFollowup(slug, id);
      if (!alive || mine !== generation) return;
      row = result; studentRef = result.student_ref; studentName = result.student_name; profileURL = result.profile_url; reason = result.reason;
      status = result.status; notes = result.notes; followUp = result.follow_up ?? ''; message = studentMessage(result); contactNote = ''; contactedAt = bangladeshDateTime(); pendingContact = null;
    } catch (e) { if (alive && mine === generation) failed(e); }
    finally { if (alive && mine === generation) loading = false; }
  }
  async function save(e: SubmitEvent) {
    e.preventDefault(); if (saving || conflict) return; saving = true; error = '';
    try {
      if (id === 0) await api.createStudentFollowup(slug, { student_ref: studentRef.trim(), student_name: studentName.trim(), profile_url: profileURL.trim(), reason: reason.trim() });
      else if (row) await api.updateStudentFollowup(slug, id, { student_name: studentName.trim(), profile_url: profileURL.trim(), reason: reason.trim(), status, notes, follow_up: followUp || null, expected_version: row.version });
      if (alive) { onSaved(); onClose(); }
    } catch (e) { failed(e); } finally { if (alive) saving = false; }
  }
  async function recordContact(e: SubmitEvent) {
    e.preventDefault(); if (!row || saving || conflict || outcomeDirty) return; saving = true; error = '';
    const instant = `${contactedAt}:00+06:00`;
    if (!pendingContact || pendingContact.channel !== channel || pendingContact.contacted_at !== instant || pendingContact.note !== contactNote) {
      pendingContact = { channel, contacted_at: instant, note: contactNote, request_id: crypto.randomUUID(), expected_version: row.version };
    }
    try { await api.recordStudentContact(slug, id, pendingContact); if (alive) { onSaved(); onClose(); } }
    catch (e) { failed(e); } finally { if (alive) saving = false; }
  }
  async function copyMessage() {
    try { await navigator.clipboard.writeText(message); if (alive) copyStatus = 'Copied. Contact the student through your usual channel, then record it below.'; }
    catch { if (alive) copyStatus = 'Clipboard unavailable. Select and copy the message draft manually.'; }
  }
  async function showHistory() {
    historyOpen = !historyOpen; if (!historyOpen) return; historyError = '';
    try { const events = await api.studentFollowupHistory(slug, id); if (alive) history = events; }
    catch (e) { if (alive) { history = []; if (e instanceof ApiError && [401, 403, 404].includes(e.status)) failed(e); else historyError = 'Could not load history. Close and reopen it to retry.'; } }
  }
  onMount(() => { alive = true; contactedAt = bangladeshDateTime(); if (id) void load(); return () => { alive = false; generation++; }; });
</script>
<!-- eslint-disable svelte/no-navigation-without-resolve -- Both anchors use validated external school URLs, not application routes. -->
<div class="min-w-0 space-y-4 rounded-lg border border-border p-4">
  <div class="flex flex-wrap items-center justify-between gap-3"><h3 class="font-semibold">{id ? 'Student follow-up' : 'Link a student to this job'}</h3><Button variant="outline" onclick={onClose} disabled={saving}>Back to student follow-ups</Button></div>
  {#if loading}<p role="status">Loading student follow-up…</p>{/if}
  {#if error}<p role="alert" class="break-words text-sm text-destructive">{error}</p>{/if}
  {#if conflict || (id && !loading && !row)}<Button variant="outline" onclick={load}>Reload follow-up (discard draft)</Button>{/if}
  {#if !loading && !unavailable && (!id || row)}
    {#if row}<p class="break-words text-sm">{row.title} · {row.company}<br /><strong>{row.student_ref}</strong> · {row.contact_count} recorded contacts</p>{/if}
    {#if row?.closed_at}<p class="text-sm text-muted-foreground">This posting is closed. You can update outcomes and view history.</p>{/if}
    <form onsubmit={save} class="space-y-3">
      {#if !id}<label class="block space-y-1 text-sm">Student ID or profile reference<input bind:value={studentRef} class={field} maxlength="300" required disabled={saving} /><span class="block text-xs text-muted-foreground">Use a stable school ID or profile link. This reference stays fixed; close a mistaken link and add the correct one.</span></label>{/if}
      <label class="block space-y-1 text-sm">Student name (optional)<input bind:value={studentName} class={field} maxlength="120" disabled={saving} /></label>
      <label class="block space-y-1 text-sm">School profile URL (optional)<input type="url" bind:value={profileURL} class={field} maxlength="1000" disabled={saving} /></label>
      {#if row && safeStudentLink(row.profile_url)}<a href={safeStudentLink(row.profile_url)} target="_blank" rel="noopener noreferrer" class="break-words text-sm underline">Open school profile</a>{/if}
      <label class="block space-y-1 text-sm">Why this job may fit<textarea bind:value={reason} class={field} rows="3" maxlength="2000" required disabled={saving}></textarea></label>
      {#if row}
        <label class="block space-y-1 text-sm">Outcome<select bind:value={status} class={field} disabled={saving}>{#each studentStages as stage (stage)}<option value={stage} disabled={stage === 'informed' && row.contact_count === 0}>{stage.charAt(0).toUpperCase() + stage.slice(1)}</option>{/each}</select></label>
        <label class="block space-y-1 text-sm">Follow-up notes<textarea bind:value={notes} class={field} rows="3" maxlength="2000" disabled={saving}></textarea></label>
        <label class="block space-y-1 text-sm">Next follow-up date (Bangladesh)<input type="date" bind:value={followUp} class={field} min="1900-01-01" max="9999-12-31" disabled={saving} /></label>
      {/if}
      <Button type="submit" disabled={saving || conflict}>{saving ? 'Saving…' : id ? 'Save outcome' : 'Link student'}</Button>
    </form>
    {#if row}
      {#if !row.closed_at && row.status !== 'closed'}
        <section class="space-y-3 border-t border-border pt-4" aria-label="Manual student contact">
          <h4 class="font-semibold">Inform this student manually</h4><p class="text-sm text-muted-foreground">Copying a draft does not send a message or mark the student informed. Use your usual channel and respect the student’s contact preferences.</p>
          <label class="block space-y-1 text-sm">Message draft<textarea bind:value={message} rows="7" class={field} disabled={saving}></textarea></label>
          <Button variant="outline" onclick={copyMessage} disabled={saving || !safeStudentLink(row.job_url)}>Copy message</Button>
          {#if copyStatus}<p role="status" class="text-sm">{copyStatus}</p>{/if}
          <form onsubmit={recordContact} class="space-y-3">
            <label class="block space-y-1 text-sm">Contact channel<select bind:value={channel} class={field} disabled={saving}>{#each contactChannels as option (option)}<option value={option}>{option === 'whatsapp' ? 'WhatsApp' : option.charAt(0).toUpperCase() + option.slice(1)}</option>{/each}</select></label>
            <label class="block space-y-1 text-sm">Contacted at (Bangladesh time)<input type="datetime-local" bind:value={contactedAt} class={field} required min="1900-01-01T00:00" disabled={saving} /></label>
            <label class="block space-y-1 text-sm">Contact note (optional)<textarea bind:value={contactNote} rows="2" maxlength="2000" class={field} disabled={saving}></textarea></label>
            <p class="text-xs text-muted-foreground">Record only contact you have already made. Existing application outcomes are preserved.</p>
            <Button type="submit" disabled={saving || conflict || outcomeDirty}>Record manual contact</Button>
            {#if outcomeDirty}<p class="text-sm">Save outcome changes before recording contact.</p>{/if}
          </form>
        </section>
      {/if}
      <div class="space-y-3 border-t border-border pt-4"><Button variant="ghost" onclick={showHistory} disabled={saving}>{historyOpen ? 'Hide student history' : 'View student history'}</Button>
        {#if historyError}<p role="alert" class="text-sm text-destructive">{historyError}</p>{/if}
        {#if historyOpen}<p class="text-xs text-muted-foreground">Latest 100 changes; times shown in Bangladesh time.</p><ol class="space-y-3">{#each history as event (event.id)}<li class="space-y-1 rounded border border-border p-3 text-sm"><p class="break-words font-medium">{event.actor} · {event.action} · {event.status} · version {event.version}</p><p>{bangladeshTime(event.created_at)}</p>{#if event.contacted_at}<p>Contact: {event.channel} · {bangladeshTime(event.contacted_at)}</p>{/if}{#if event.contact_note}<p class="whitespace-pre-wrap break-words">{event.contact_note}</p>{/if}<p class="break-words">{event.student_name}</p><p class="whitespace-pre-wrap break-words">Reason: {event.reason}</p>{#if event.notes}<p class="whitespace-pre-wrap break-words">Notes: {event.notes}</p>{/if}{#if event.follow_up}<p>Next follow-up: {event.follow_up}</p>{/if}{#if safeStudentLink(event.profile_url)}<a class="underline" href={safeStudentLink(event.profile_url)} target="_blank" rel="noopener noreferrer">School profile at this revision</a>{/if}</li>{/each}</ol>{/if}
      </div>
    {/if}
  {/if}
</div>
