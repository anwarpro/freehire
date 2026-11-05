<script lang="ts">
  import { resolve } from '$app/paths';
  import { jobStatusQuery } from '#lib/closedJobs.js';
  let { closed = false, params = '' }: { closed?: boolean; params?: string } = $props();
</script>

<nav aria-label="Job status" class="mb-4 flex flex-wrap gap-2 text-sm">
  <a href={resolve(`/jobs${jobStatusQuery(params)}`)} aria-current={!closed ? 'page' : undefined}
    class="rounded-lg border border-border px-3 py-2 font-medium hover:bg-accent aria-[current=page]:bg-secondary">Active jobs</a>
  <a href={resolve(`/jobs/closed${jobStatusQuery(params)}`)} aria-current={closed ? 'page' : undefined}
    title="Browse closed jobs; source, skill and country filters are carried over"
    class="rounded-lg border border-border px-3 py-2 font-medium hover:bg-accent aria-[current=page]:bg-secondary">Closed / expired jobs</a>
</nav>
