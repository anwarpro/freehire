<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '#lib/api.js';
  import { teamJobTurn, StreamInterrupted, type Turn } from '#lib/assistant/client.js';
  import { initChat, reduceTurnEvent } from '#lib/assistant/chat.js';
  import { eventsFromTranscript, type SessionSummary } from '#lib/assistant/wire.js';
  import { teamJobSources } from '#lib/assistant/teamSources.js';
  import { renderMarkdown } from '#lib/markdown.js';
  import { Button } from '#lib/ui/index.js';
  let status = $state<Awaited<ReturnType<typeof api.teamBotStatus>> | null>(null);
  let sessions = $state.raw<SessionSummary[]>([]); let session = $state<SessionSummary | null>(null);
  let meta = $state({ has_more: false, offset: 0, limit: 50 }); let chat = $state(initChat());
  let question = $state(''); let loading = $state(true); let running = $state(false); let error = $state('');
  let needsReload = $state(false); let lastError = $state(false); let unavailable = $state(false); let confirmingDelete = $state(false);
  let active: Turn | null = null; let alive = false; let generation = 0;
  const examples = ['Show recent React jobs from LinkedIn.', 'Which Python jobs overlap with our courses?', 'Compare the requirements of these two job links.'];
  function failed(e: unknown) {
    if (!alive) return;
    if (e instanceof ApiError && [401, 403, 404].includes(e.status)) {
      session = null; sessions = []; chat = initChat(); question = ''; status = null; unavailable = true; lastError = false;
    }
    error = e instanceof Error ? e.message : 'Could not load job discussion. Try again.';
  }
  async function load(offset = 0) {
    const mine = ++generation; loading = true; error = ''; unavailable = false;
    try {
      const [settings, list] = await Promise.all([api.teamBotStatus(), api.teamBotSessions(offset)]);
      if (!alive || mine !== generation) return; status = settings; sessions = list.data; meta = list.meta;
    } catch (e) { if (alive && mine === generation) failed(e); }
    finally { if (alive && mine === generation) loading = false; }
  }
  async function open(id: string) {
    if (running) return; const mine = ++generation; loading = true; error = ''; chat = initChat(); session = null; lastError = false; needsReload = false; confirmingDelete = false;
    try { const result = await api.teamBotSession(id); if (!alive || mine !== generation) return; session = result.session; let state = initChat(); for (const event of eventsFromTranscript(result.messages)) state = reduceTurnEvent(state, event); chat = state; }
    catch (e) { if (alive && mine === generation) failed(e); } finally { if (alive && mine === generation) loading = false; }
  }
  async function fresh() {
    if (running || !status?.can_chat) return; loading = true; error = ''; confirmingDelete = false;
    try { const created = await api.createTeamBotSession(); if (!alive) return; session = created; chat = initChat(); lastError = false; needsReload = false; question = ''; sessions = [created, ...sessions.filter((s) => s.id !== created.id)].slice(0, 50); }
    catch (e) { failed(e); } finally { if (alive) loading = false; }
  }
  async function send(retry = false) {
    if (!session || running || needsReload || !status?.can_chat || (!retry && !question.trim())) return;
    const text = question.trim(); running = true; error = ''; lastError = false;
    active = teamJobTurn(session.id, text, (event) => {
      if (!alive) return; chat = reduceTurnEvent(chat, event);
      if (event.type === 'user_prompt') question = '';
      if (event.type === 'result') lastError = event.is_error === true;
    }, retry);
    try { await active.done; }
    catch (e) {
      if (e instanceof StreamInterrupted) { if (alive) needsReload = true; if (alive) error = 'Connection interrupted. The turn may still be running. Reload the conversation before sending again.'; }
      else { if (!(e instanceof ApiError) && alive) needsReload = true; failed(e); }
    } finally { if (alive) { running = false; active = null; } }
  }
  async function remove() {
    if (!session || running) return; loading = true; error = '';
    try { await api.deleteTeamBotSession(session.id); if (!alive) return; sessions = sessions.filter((s) => s.id !== session?.id); session = null; chat = initChat(); confirmingDelete = false; await load(meta.offset); }
    catch (e) { failed(e); } finally { if (alive) loading = false; }
  }
  onMount(() => { alive = true; void load(); return () => { alive = false; generation++; active?.cancel(); }; });
</script>
<section class="min-w-0 space-y-4" aria-label="Team job discussion">
  <h2 class="text-xl font-semibold">Discuss collected jobs</h2>
  <p class="text-sm text-muted-foreground">Ask about requirements, compare postings, and explore automatic curriculum overlap. Replies are in English and conversations are private to your account while you have team access.</p>
  {#if loading}<p role="status">Loading job discussion…</p>{/if}
  {#if error}<p role="alert" class="break-words text-sm text-destructive">{error}</p>{/if}
  {#if unavailable}<Button variant="outline" onclick={() => void load()}>Check access again</Button>{/if}
  {#if status}
    {#if !status.model_configured}<p role="status" class="rounded border border-border p-3 text-sm">Job discussion is awaiting model configuration. A manager must configure the gateway credentials and exact model ID. Existing conversations remain readable.</p>{/if}
    {#if !status.paid_access}<p class="rounded border border-border p-3 text-sm">Your account needs Pro or Ultra access to send messages under the current policy. Team membership alone does not enable paid model calls.</p>{/if}
    <p class="text-xs text-muted-foreground">{status.data_scope} Automatic overlap does not establish student eligibility. Check the original posting before acting.</p>
    <div class="flex flex-wrap gap-2"><Button onclick={fresh} disabled={loading || running || !status.can_chat}>New job conversation</Button><Button variant="outline" onclick={() => void load(meta.offset)} disabled={loading || running}>Refresh availability</Button></div>
    <label class="block space-y-1 text-sm"><span class="block">Your job conversations</span><select class="w-full min-w-0 rounded border border-border bg-background px-3 py-2" value={session?.id ?? ''} onchange={(e) => { if (e.currentTarget.value) void open(e.currentTarget.value); }} disabled={loading || running}><option value="">Choose a conversation</option>{#each sessions as item (item.id)}<option value={item.id}>{item.label || 'New job conversation'}</option>{/each}</select></label>
    <div class="flex flex-wrap gap-2"><Button variant="outline" disabled={loading || running || meta.offset === 0} onclick={() => void load(Math.max(0, meta.offset - 50))}>Previous conversations</Button><Button variant="outline" disabled={loading || running || !meta.has_more} onclick={() => void load(meta.offset + 50)}>Next conversations</Button></div>
    {#if session}
      <div class="flex flex-wrap gap-2"><Button variant="outline" disabled={running || loading} onclick={() => { if (session) void open(session.id); }}>Reload conversation</Button><Button variant="ghost" disabled={running || loading} onclick={() => (confirmingDelete = !confirmingDelete)}>Delete conversation</Button></div>
      {#if confirmingDelete}<div class="flex flex-wrap items-center gap-2 rounded border border-border p-3"><p class="text-sm">Delete this conversation and its history?</p><Button variant="outline" onclick={remove} disabled={loading}>Confirm delete</Button><Button variant="ghost" onclick={() => (confirmingDelete = false)}>Keep conversation</Button></div>{/if}
      <div class="space-y-4" aria-label="Job conversation transcript" aria-busy={running}>
        {#each chat.messages as message, i (i)}<article class="min-w-0 rounded-lg border border-border p-4"><p class="mb-2 text-xs font-semibold">{message.role === 'user' ? 'You' : 'Job assistant'}</p>{#if message.role === 'user'}<p class="whitespace-pre-wrap break-words">{message.text}</p>{:else}<!-- eslint-disable-next-line svelte/no-at-html-tags -- Shared renderMarkdown sanitizes model text with DOMPurify before rendering. -->
<div class="prose prose-sm min-w-0 max-w-none break-words dark:prose-invert [&_a]:break-all [&_pre]:overflow-auto">{@html renderMarkdown(message.text)}</div>{@const sources = teamJobSources(message.tools)}
        {#if sources.length}<ul class="mt-3 space-y-2 text-sm" aria-label="Verified job sources">{#each sources as source (source.jobLink)}<li class="break-words">
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- Validated canonical HTTP/HTTPS or job-relative links returned by the job tools. -->
          <a href={source.jobLink} class="underline">{source.title}</a>{#if source.closed} · Closed posting{/if}
          {#if source.sourceURL}
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- Validated external original posting URL; never executed or fetched by this component. -->
            <a href={source.sourceURL} target="_blank" rel="noopener noreferrer" class="ml-2 underline">Original posting</a>
          {/if}
        </li>{/each}</ul>{/if}
        {#if message.tools.length}<details class="mt-3 text-xs text-muted-foreground"><summary>Sources and tools used ({message.tools.length})</summary><ul>{#each message.tools as tool, j (j)}<li class="mt-2 break-words">{tool.name}{tool.isError ? ' — unavailable' : ''}</li>{/each}</ul></details>{/if}{#if message.errored}<p class="mt-2 text-sm text-destructive">The answer did not finish. Retry to continue from the saved conversation.</p>{/if}{/if}</article>{/each}
      </div>
      {#if running}<p role="status">{chat.queued ? 'Waiting for the previous turn…' : 'Reading job evidence…'}</p><Button variant="outline" onclick={() => active?.cancel()}>Stop reply</Button>{/if}
      {#if lastError && !running}<Button variant="outline" onclick={() => void send(true)} disabled={needsReload || !status.can_chat}>Retry reply</Button>{/if}
      {#if chat.messages.length === 0}<div class="flex flex-wrap gap-2">{#each examples as example (example)}<Button variant="outline" onclick={() => (question = example)} disabled={running}>{example}</Button>{/each}</div>{/if}
      <form class="space-y-2" onsubmit={(e) => { e.preventDefault(); void send(); }}><label class="block text-sm">Ask about collected jobs<textarea bind:value={question} rows="3" maxlength="8000" required class="mt-1 w-full min-w-0 rounded border border-border bg-background px-3 py-2" disabled={running || needsReload || !status.can_chat}></textarea></label><Button type="submit" disabled={loading || running || needsReload || !status.can_chat || !question.trim()}>Send question</Button></form>
    {:else}<p class="text-sm text-muted-foreground">Choose an existing conversation or start a new one.</p>{/if}
  {/if}
</section>
