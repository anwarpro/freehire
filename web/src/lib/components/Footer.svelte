<script lang="ts">
  import { resolve } from '$app/paths';
  import { reopen } from '#lib/consent.svelte.js';
  import { POPULAR_COLLECTIONS } from '#lib/popularCollections.js';

  // Grouped navigation over existing routes only — kept deliberately small so the
  // footer stays uncluttered. Internal links go through resolve() (base-path safe),
  // mirroring the header.
  const groups = [
    {
      title: 'Product',
      links: [
        { label: 'Jobs', href: resolve('/jobs') },
        { label: 'Companies', href: resolve('/companies') },
        { label: 'Collections', href: resolve('/collections') },
        { label: 'Talent Network', href: resolve('/talent') },
        { label: 'Jobs by role', href: resolve('/roles') },
        // The glossary's only link from the app — the chip's reveal opens on
        // interaction and the sitemap is for crawlers, so this is the one path a
        // reader browsing the site can follow to it.
        { label: 'Skills glossary', href: resolve('/skills') },
        { label: 'Dictionary', href: resolve('/dictionary') },
        { label: 'Recruiters', href: resolve('/recruiters') },
      ],
    },
    {
      title: 'Features',
      links: [
        { label: 'Advanced search', href: resolve('/features/advanced-search') },
        { label: 'Inbox', href: resolve('/features/inbox') },
        { label: 'CV tailoring', href: resolve('/features/tailor') },
        { label: 'Application tracking', href: resolve('/features/tracking') },
        { label: 'Notifications', href: resolve('/features/notifications') },
        { label: 'Referrals', href: resolve('/features/referrals') },
        { label: 'Ghost jobs', href: resolve('/features/ghost-jobs') },
      ],
    },
    {
      title: 'Resources',
      links: [
        { label: 'How it works', href: resolve('/how-it-works') },
        { label: 'Discussions', href: resolve('/discussions') },
        { label: 'Insights', href: resolve('/insights') },
        { label: 'Hiring signal', href: resolve('/insights/companies') },
        { label: 'Trends', href: resolve('/trends') },
        { label: 'Semantic search', href: resolve('/semantic-search') },
        { label: 'API docs', href: resolve('/docs/api') },
        { label: 'Jobs API', href: resolve('/jobs-api') },
      ],
    },
    {
      title: 'Company',
      links: [
        { label: 'About', href: resolve('/about') },
        { label: 'For companies', href: resolve('/for-companies') },
        { label: 'Submit a job', href: resolve('/submit') },
        // Next to Status rather than under Resources: the two answer neighbouring
        // questions — where the jobs come from, and whether we are still reading them.
        { label: 'Sources', href: resolve('/sources') },
        { label: 'Status', href: resolve('/status') },
        { label: 'Privacy', href: resolve('/privacy') },
        { label: 'Terms', href: resolve('/terms') },
        { label: 'Job removal', href: resolve('/job-removal') },
      ],
    },
  ];

  // A strip below the four groups rather than a fifth column: the grid is
  // sm:grid-cols-4, and ten links would not fit one anyway. Kept out of `groups`
  // because these are collection landing pages, not site navigation — and because
  // it is the one place every page links them from (see #lib/popularCollections).
  const popular = POPULAR_COLLECTIONS;

  const year = new Date().getFullYear();

  // Compact: the account shell (/my/*) is an app-like surface with its own sidebar
  // nav, so the four link columns, the popular-collections strip and the Product
  // promotions are unnecessary there — only the
  // bottom bar (copyright, cookie settings, social links, open-source note) still
  // applies. Full-bleed account routes (/my/assistant/*, /tailor/*) render no
  // footer at all, compact or otherwise — see routes/+layout.svelte's hideFooter.
  let { compact = false }: { compact?: boolean } = $props();
</script>

<footer class="border-t border-border">
  {#if !compact}
    <div class="mx-auto max-w-6xl px-4 py-8 sm:py-12">
      <div class="grid grid-cols-2 gap-x-6 gap-y-7 sm:grid-cols-4 sm:gap-6">
        <!-- Navigation groups. Each is a named landmark (aria-label) so screen readers
             get a title without adding headings to the page outline. -->
        {#each groups as group (group.title)}
          <nav class="flex flex-col gap-3" aria-label={group.title}>
            <p class="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {group.title}
            </p>
            <ul class="flex flex-col gap-2">
              {#each group.links as link (link.href)}
                <li>
                  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal route already passed through resolve() when building `groups`; the linter can't trace it via the variable -->
                  <a href={link.href}
                    class="text-sm text-muted-foreground transition-colors hover:text-foreground"
                  >
                    {link.label}
                  </a>
                </li>
              {/each}
            </ul>
          </nav>
        {/each}
      </div>

      <!-- Popular collections. Real <a href> in the server-rendered HTML: crawlers
           discover links by parsing markup, and these landing pages had none from the
           homepage at all. -->
      <nav class="mt-8 border-t border-border pt-6" aria-label="Popular collections">
        <p class="text-xs font-medium uppercase tracking-wider text-muted-foreground">Popular</p>
        <ul class="mt-3 flex flex-wrap gap-x-4 gap-y-2">
          {#each popular as collection (collection.slug)}
            <li>
              <a href={resolve('/collections/[slug]', { slug: collection.slug })}
                class="text-sm text-muted-foreground transition-colors hover:text-foreground"
              >
                {collection.title}
              </a>
            </li>
          {/each}
        </ul>
      </nav>

    </div>
  {/if}

  <!-- Bottom bar: copyright and cookie settings on the left, team attribution on the
       right. Split off by its own top border only when it follows the link
       groups above — compact, the footer's own top border already does that job. -->
  <div class={compact ? undefined : 'border-t border-border'}>
    <div
      class="mx-auto flex max-w-6xl flex-col gap-3 px-4 py-4 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between sm:gap-1"
    >
      <div class="flex items-center gap-4">
        <p>© {year} PH Jobs</p>
        <!-- Re-opens the consent banner so a prior cookie choice can be changed —
             withdrawal as easy as granting (GDPR). -->
        <button
          type="button"
          onclick={reopen}
          class="text-muted-foreground transition-colors hover:text-foreground"
        >
          Cookie settings
        </button>
      </div>
      <p>Built for the Programming Hero team.</p>
    </div>
  </div>
</footer>
