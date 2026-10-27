<script lang="ts">
  import { page } from '$app/state';
  import AboutValues from '#lib/components/AboutValues.svelte';
  import HomeView from '#lib/components/HomeView.svelte';
  import Seo from '#lib/components/Seo.svelte';
  import { HOME_FAQ } from '#lib/homeFaq.js';
  import { faqPageJsonLd, jsonLdScript, siteOrganizationJsonLd, websiteJsonLd } from '#lib/seo.js';
  import type { PageData } from './$types';

  const { data }: { data: PageData } = $props();

  const origin = $derived(page.url.origin);
  const canonical = $derived(`${origin}/about`);
  // One script carrying the site-level entities: what the site is (WebSite +
  // its search action), who publishes it (Organization), and the FAQ.
  const jsonLd = $derived(
    jsonLdScript([websiteJsonLd(origin), siteOrganizationJsonLd(origin), faqPageJsonLd(HOME_FAQ)])
  );
</script>

<!-- The description was 215 characters and got cut mid-sentence in the SERP snippet,
     and where Google cuts is not where we would. Trimmed to ~155, the width it shows. -->
<Seo
  title="About PH Jobs — the open-source search engine for tech jobs"
  description="PH Jobs is the Programming Hero team’s workspace for worldwide tech jobs, skills research and job market insights."
  {canonical}
/>

<svelte:head>
  <!-- eslint-disable-next-line svelte/no-at-html-tags -- non-executable JSON-LD built by jsonLdScript, which escapes `<`; raw injection is the only way to emit a structured-data <script> -->
  {@html jsonLd}
</svelte:head>

<div class="mx-auto w-full max-w-6xl px-4 py-6">
  <HomeView stats={data.stats} />
  <AboutValues />
</div>
