<script lang="ts">
  import { page } from '$app/state';
  import Seo from '#lib/components/Seo.svelte';
  import HowItWorksLandingView from '#lib/components/HowItWorksLandingView.svelte';
  import { breadcrumbJsonLd, faqPageJsonLd, jsonLdScript } from '#lib/seo.js';
  import { PIPELINE_FAQ } from '#lib/pipelineFaq.js';
  import type { PageData } from './$types';

  const { data }: { data: PageData } = $props();

  const origin = $derived(page.url.origin);
  const canonical = $derived(`${origin}/how-it-works`);
  const jsonLd = $derived(
    jsonLdScript([
      faqPageJsonLd(PIPELINE_FAQ),
      breadcrumbJsonLd([
        { name: 'PH Jobs', url: `${origin}/` },
        { name: 'How it works', url: canonical },
      ]),
    ])
  );
</script>

<Seo
  title="How PH Jobs works — from a job board to your search results | PH Jobs"
  description="How a posting becomes searchable on PH Jobs: crawled straight from the source, tagged, checked for duplicates across every board, and rebuilt into the live search index — five steps, no person touching a listing."
  {canonical}
/>

<svelte:head>
  <!-- eslint-disable-next-line svelte/no-at-html-tags -- non-executable JSON-LD built by jsonLdScript, which escapes `<`; raw injection is the only way to emit a structured-data <script> -->
  {@html jsonLd}
</svelte:head>

<div class="mx-auto w-full max-w-6xl px-4 py-6">
  <HowItWorksLandingView scale={data.scale} />
</div>
