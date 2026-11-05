<script lang="ts">
  import { resolve } from '$app/paths';
  import { api, ApiError } from '#lib/api.js';
  import { originalPostingUrl } from '#lib/closedJobs.js';
  import type { CareerCatalog, CareerInput, CareerPageConfig, CareerPreview } from '#lib/api.js';
  import { Button, Input } from '#lib/ui/index.js';
  let catalog = $state<CareerCatalog>({ data: [], shortlist: [] });
  let loading = $state(true);
  let busy = $state(false);
  let error = $state('');
  let notice = $state('');
  let company = $state('');
  let url = $state('');
  let advanced = $state(false);
  let links = $state(''); let title = $state('h1'); let body = $state('');
  let location = $state(''); let posted = $state(''); let next = $state('');
  let emptyText = $state(''); let detailHosts = $state(''); let defaultLocation = $state('');
  let preview = $state<CareerPreview | null>(null);
  let previewKey = $state('');
  const input = $derived<CareerInput>({ company: company.trim(), url: url.trim(), ...(advanced ? { config: { links, title, body, location, posted, next, empty_text: emptyText, detail_hosts: detailHosts.split(',').map(v => v.trim()).filter(Boolean), default_location: defaultLocation } } : {}) });
  const canEnable = $derived(preview !== null && previewKey === JSON.stringify(input));
  const describeError = (e: unknown) => e instanceof ApiError ? e.message : 'Could not reach the server. Your configuration is preserved.';
  async function refresh() {
    loading = true;
    try { catalog = await api.listCareerSources(); } catch (e) { error = describeError(e); }
    finally { loading = false; }
  }
  $effect(() => { void refresh(); });
  function configure(seed: CareerInput) {
    company = seed.company; url = seed.url;
    const c: CareerPageConfig = seed.config ?? {};
    advanced = Boolean(seed.config);
    links = c.links ?? ''; title = c.title ?? 'h1'; body = c.body ?? ''; location = c.location ?? '';
    posted = c.posted ?? ''; next = c.next ?? ''; emptyText = c.empty_text ?? '';
    detailHosts = c.detail_hosts?.join(', ') ?? ''; defaultLocation = c.default_location ?? '';
    preview = null; notice = ''; error = '';
  }
  async function verify(e: SubmitEvent) {
    e.preventDefault(); if (busy) return;
    busy = true; error = ''; notice = ''; preview = null;
    const draft = input;
    try { preview = await api.previewCareerSource(draft); previewKey = JSON.stringify(draft); }
    catch (e) { error = describeError(e); }
    finally { busy = false; }
  }
  async function enable() {
    if (!canEnable || busy) return;
    busy = true; error = '';
    try { await api.enableCareerSource(input); notice = 'Career page enabled. Collection is queued for the scheduled rotation.'; preview = null; await refresh(); }
    catch (e) { error = describeError(e); }
    finally { busy = false; }
  }
  async function pause(id: number) {
    if (busy) return;
    busy = true; error = '';
    try { await api.pauseCareerSource(id); notice = 'Collection paused. Existing job records are retained.'; await refresh(); }
    catch (e) { error = describeError(e); }
    finally { busy = false; }
  }
  const labels: Record<string, string> = { awaiting_first_collection: 'Awaiting first collection', healthy: 'Healthy', collection_failed: 'Collection failed — inspect worker logs', paused: 'Paused' };
</script>

<div class="flex flex-col gap-6">
  <div><h2 class="text-xl font-semibold">Company career pages</h2>
    <p class="mt-2 text-sm text-muted-foreground">Add a company career page, verify its jobs, then enable automatic collection. Company sources receive priority in the five-minute rotation and are attempted at most once per hour. Cooldowns and unchanged-listing backoff may delay a fetch.</p>
  </div>
  {#if error}<p role="alert" class="rounded-lg border border-destructive p-3 text-sm">{error}</p>{/if}
  {#if notice}<p role="status" class="rounded-lg border border-border p-3 text-sm">{notice}</p>{/if}
  <p class="text-xs text-muted-foreground">To replace an enabled source’s settings, pause its collection first, then configure it again.</p>
  <form onsubmit={verify} class="rounded-xl border border-border p-4">
    <fieldset disabled={busy} class="grid gap-4 sm:grid-cols-2">
      <label class="flex flex-col gap-1 text-sm">Company name<Input bind:value={company} required maxlength={200} placeholder="Company name" /></label>
      <label class="flex flex-col gap-1 text-sm">Career page URL<Input bind:value={url} type="url" required maxlength={2048} placeholder="https://company.com/careers" /></label>
      <label class="flex items-center gap-2 text-sm sm:col-span-2"><input type="checkbox" bind:checked={advanced} />Configure custom HTML selectors</label>
      {#if advanced}
        <p class="text-xs text-muted-foreground sm:col-span-2">Use selectors for the public job links and full job description. Blank selectors use structured JobPosting data. Include a next-page selector for paginated lists. JavaScript-only pages need a supported adapter.</p>
        <label class="flex flex-col gap-1 text-sm">Job links CSS selector<Input bind:value={links} maxlength={512} placeholder=".jobs a.job-link" /></label>
        <label class="flex flex-col gap-1 text-sm">Job title CSS selector<Input bind:value={title} maxlength={512} /></label>
        <label class="flex flex-col gap-1 text-sm">Full description CSS selector<Input bind:value={body} maxlength={512} placeholder=".job-description" /></label>
        <label class="flex flex-col gap-1 text-sm">Location CSS selector<Input bind:value={location} maxlength={512} /></label>
        <label class="flex flex-col gap-1 text-sm">Posting date CSS selector<Input bind:value={posted} maxlength={512} placeholder="time[datetime]" /></label>
        <label class="flex flex-col gap-1 text-sm">Next page CSS selector<Input bind:value={next} maxlength={512} placeholder="a[rel=next]" /></label>
        <label class="flex flex-col gap-1 text-sm">Exact empty-list notice<Input bind:value={emptyText} maxlength={200} placeholder="There are currently no openings" /></label>
        <label class="flex flex-col gap-1 text-sm">Additional job-detail hosts<Input bind:value={detailHosts} maxlength={400} placeholder="jobs.company.com" /></label>
        <label class="flex flex-col gap-1 text-sm sm:col-span-2">Fallback location (only when the source confirms it)<Input bind:value={defaultLocation} maxlength={200} /></label>
      {/if}
      <div class="flex gap-3 sm:col-span-2"><Button type="submit" disabled={busy}>{busy ? 'Checking…' : 'Preview collection'}</Button>
        <Button type="button" onclick={enable} disabled={busy || !canEnable}>Enable collection</Button></div>
    </fieldset>
  </form>
  {#if preview && canEnable}
    <section aria-label="Collection preview" class="rounded-xl border border-border p-4">
      <p class="font-medium">{preview.active} active · {preview.closed} closed/expired · {preview.provider}</p>
      <p class="mt-1 text-sm text-muted-foreground">Verified now. Jobs enter the catalogue when the scheduled collector runs. No active jobs means the verified feed is currently empty or expired.</p>
      <ul class="mt-3 list-disc pl-5 text-sm">{#each preview.samples as sample (sample.url)}
        {@const original = originalPostingUrl(sample.url)}<li>
        {#if original}
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- validated external HTTP(S) original job URL -->
          <a href={original} target="_blank" rel="noopener noreferrer" class="underline">{sample.title} ↗</a>
        {:else}{sample.title}{/if}{sample.closed ? ' — closed/expired' : ''}
      </li>{/each}</ul>
    </section>
  {/if}
  <section aria-label="Enabled career sources">
    <div class="mb-3 flex items-center justify-between"><h3 class="font-semibold">Collection status</h3><Button size="sm" variant="ghost" onclick={refresh} disabled={busy || loading}>Refresh status</Button></div>
    {#if loading}<p class="text-sm">Loading career sources…</p>
    {:else if catalog.data.length === 0}<p class="text-sm text-muted-foreground">No company career pages enabled yet. Configure a source below or add your own.</p>
    {:else}<div class="flex flex-col gap-3">{#each catalog.data as source (source.id)}
      <article class="rounded-xl border border-border p-4">
        <p class="font-medium">{source.company}</p><p class="text-sm">{labels[source.state] ?? source.state} · {source.provider}</p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- server-validated external HTTPS career URL -->
        <a href={source.url} target="_blank" rel="noopener noreferrer" class="break-all text-sm underline">Career page ↗</a>
        <p class="mt-2 text-xs text-muted-foreground">Last attempt: {source.last_run_at ? new Date(source.last_run_at).toLocaleString() : 'Never'} · Last success: {source.last_success_at ? new Date(source.last_success_at).toLocaleString() : 'Never'} · Last run saved: {source.last_ingested_count ?? 0} · Consecutive failures: {source.failures ?? 0}</p>
        {#if source.cooldown_until}<p class="text-xs text-muted-foreground">Failure cooldown until {new Date(source.cooldown_until).toLocaleString()}</p>{/if}
        {#if source.quiet_until}<p class="text-xs text-muted-foreground">Unchanged-listing pause until {new Date(source.quiet_until).toLocaleString()}</p>{/if}
        <div class="mt-3 flex gap-3"><Button size="sm" variant="ghost" onclick={() => configure(source)} disabled={busy}>{source.status === 'retired' ? 'Configure again' : 'Preview again'}</Button>
        {#if source.status !== 'retired'}<Button size="sm" variant="ghost" onclick={() => pause(source.id)} disabled={busy}>Pause collection</Button>{/if}</div>
      </article>
    {/each}</div>{/if}
  </section>
  <section aria-label="Bangladesh company shortlist">
    <h3 class="mb-3 font-semibold">Bangladesh company shortlist</h3>
    <p class="mb-3 text-sm text-muted-foreground">Supported sources can be previewed and enabled. Entries with an access limitation use manual intake until their collection method is verified.</p>
    <div class="grid gap-3 sm:grid-cols-2">{#each catalog.shortlist as seed (seed.url)}
      <article class="rounded-xl border border-border p-4"><p class="font-medium">{seed.company}</p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- curated external HTTPS company career URL -->
        <a href={seed.url} target="_blank" rel="noopener noreferrer" class="break-all text-sm underline" aria-label={`Career page for ${seed.company}`}>Career page ↗</a>
        {#if seed.fallback}<p class="my-2 text-sm text-muted-foreground">{seed.fallback}</p><a href={resolve('/moderation?tab=publish')} class="text-sm underline">Add a job manually</a>
        {:else}<p class="my-2 text-sm text-muted-foreground">Collector available · verification required</p><Button size="sm" variant="ghost" onclick={() => configure(seed)} disabled={busy}>Configure {seed.company}</Button>{/if}
      </article>
    {/each}</div>
  </section>
</div>
