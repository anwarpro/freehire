<script lang="ts">
  import { resolve } from '$app/paths';
  import { page } from '$app/state';
  import { SOURCE_VALUES } from '#lib/generated/contracts.js';
  import { COUNTRY_OPTIONS, sourceLabel } from '#lib/facets.js';
  import { jobStatusQuery, originalPostingUrl } from '#lib/closedJobs.js';
  import { Button, Input } from '#lib/ui/index.js';
  import JobRow from '#lib/components/JobRow.svelte';
  import JobStatusTabs from '#lib/components/JobStatusTabs.svelte';
  import Seo from '#lib/components/Seo.svelte';
  import type { PageData } from './$types';
  let { data }: { data: PageData } = $props();
  const params = $derived(new URLSearchParams(data.query));
  const values = (key: string) => params.getAll(key).join(',');
  const sources = [...new Set(['manual', 'bdjobs', 'linkedin', ...SOURCE_VALUES])].sort();
</script>

<Seo title="Closed and expired jobs · PH Jobs" description="Browse retained closed jobs by source, skills and country." canonical={`${page.url.origin}${resolve('/jobs/closed')}`} robots="noindex,follow" />

<div class="mx-auto w-full max-w-6xl px-4 py-6">
  <JobStatusTabs closed params={data.query} />
  <h1 class="text-2xl font-semibold">Closed and expired jobs</h1>
  <p class="mt-2 text-sm text-muted-foreground">
    Retained postings that no longer accept applications, newest closure first.
    This archive supports source, skill and country filters; other active-search filters are not carried over.
  </p>
  <form method="GET" action={resolve('/jobs/closed')} class="my-5 grid gap-4 rounded-xl border border-border bg-card p-4 sm:grid-cols-3">
    <label class="flex flex-col gap-1 text-sm font-medium">Sources
      <Input name="source" value={values('source')} list="archive-sources" placeholder="e.g. greenhouse, bdjobs" maxlength={1024} />
    </label>
    <label class="flex flex-col gap-1 text-sm font-medium">Skills
      <Input name="skills" value={values('skills')} placeholder="e.g. python, react" maxlength={1024} />
    </label>
    <label class="flex flex-col gap-1 text-sm font-medium">Country codes
      <Input name="countries" value={values('countries')} list="archive-countries" placeholder="e.g. bd, in" maxlength={1024} pattern="[ ]*[a-zA-Z]{2}[ ]*(,[ ]*[a-zA-Z]{2}[ ]*)*" title="Use two-letter country codes, separated by commas" />
    </label>
    <label class="flex flex-col gap-1 text-sm">Exclude sources
      <Input name="source_exclude" value={values('source_exclude')} list="archive-sources" maxlength={1024} />
    </label>
    <label class="flex flex-col gap-1 text-sm">Exclude skills
      <Input name="skills_exclude" value={values('skills_exclude')} maxlength={1024} />
    </label>
    <label class="flex flex-col gap-1 text-sm">Exclude country codes
      <Input name="countries_exclude" value={values('countries_exclude')} maxlength={1024} pattern="[ ]*[a-zA-Z]{2}[ ]*(,[ ]*[a-zA-Z]{2}[ ]*)*" title="Use two-letter country codes, separated by commas" />
    </label>
    <label class="flex items-center gap-2 text-sm">Match skills
      <select name="skills_mode" value={params.get('skills_mode') ?? 'or'} class="rounded-lg border border-input bg-background p-2">
        <option value="or">Any selected skill</option><option value="and">All selected skills</option>
      </select>
    </label>
    <p class="text-xs text-muted-foreground sm:col-span-2">Use comma-separated source names and skill tags. Country codes use two letters, such as bd for Bangladesh. Source suggestions do not imply that a collector is connected.</p>
    <div class="flex items-center gap-3 sm:col-span-3">
      <Button type="submit">Apply filters</Button>
      <a href={resolve('/jobs/closed')} class="text-sm underline underline-offset-4">Clear filters</a>
    </div>
  </form>
  <datalist id="archive-sources">{#each sources as source (source)}<option value={source}>{sourceLabel(source)}</option>{/each}</datalist>
  <datalist id="archive-countries">{#each COUNTRY_OPTIONS as country (country.value)}<option value={country.value}>{country.label}</option>{/each}</datalist>

  <p class="mb-3 text-sm text-muted-foreground">{data.items.length} closed jobs shown</p>
  {#if data.items.length === 0}
    <p class="rounded-xl border border-border p-6">No matching closed jobs. Try fewer filters or browse active jobs.</p>
  {:else}
    <div class="flex flex-col gap-3">
      {#each data.items as job (job.public_slug)}
        <JobRow {job} dimViewed={false}>
          {#snippet footer()}
            {@const original = originalPostingUrl(job.url)}
            <div class="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              <span>Source: {sourceLabel(job.source)}</span>
              {#if original}
                <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- validated external http(s) source URL, not an app route -->
                <a href={original} target="_blank" rel="noopener noreferrer" class="underline underline-offset-4">Original posting ↗</a>
              {/if}
            </div>
          {/snippet}
        </JobRow>
      {/each}
    </div>
  {/if}
  <nav aria-label="Closed jobs pages" class="mt-5 flex flex-wrap items-center gap-4 text-sm">
    {#if params.has('before')}<a href={resolve(`/jobs/closed${jobStatusQuery(data.query)}`)} class="underline underline-offset-4">Newest closed jobs</a>{/if}
    {#if data.nextCursor}<a href={resolve(`/jobs/closed${jobStatusQuery(data.query, data.nextCursor)}`)} class="rounded-lg border border-border px-4 py-2 font-medium hover:bg-accent">Older closed jobs →</a>{/if}
  </nav>
</div>
