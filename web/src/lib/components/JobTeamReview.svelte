<script lang="ts">
  import { currentUser } from '#lib/auth.svelte.js';
  import { api } from '#lib/api.js';
  import { teamRole } from '#lib/teamReview.js';
  import TeamReviewButton from './TeamReviewButton.svelte';
  let { slug }: { slug: string } = $props();
  const user = $derived(currentUser());
  const role = $derived(teamRole(user));
  let assigned = $state(false);
  $effect(() => {
    const currentSlug = slug;
    const id = user?.id;
    assigned = false;
    let cancelled = false;
    if (role === 'reviewer' && id) {
      void api.teamReview(currentSlug).then(() => { if (!cancelled) assigned = true; }).catch(() => {});
    }
    return () => { cancelled = true; };
  });
</script>
{#if user && (role === 'admin' || assigned)}
  {#key `${user.id}-${slug}`}<TeamReviewButton {slug} />{/key}
{/if}
