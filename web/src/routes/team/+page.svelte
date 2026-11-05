<script lang="ts">
  import { currentUser } from '#lib/auth.svelte.js';
  import TeamWorkspace from '#lib/components/TeamWorkspace.svelte';
  import States from '#lib/components/States.svelte';
</script>
<svelte:head><title>Team workspace — PH Jobs</title><meta name="robots" content="noindex" /></svelte:head>
<div class="mx-auto w-full max-w-5xl px-4 py-8">
  {#if currentUser()}
    {#key currentUser()?.id}<TeamWorkspace />{/key}
  {:else}<States state="empty" message="Sign in to your team account" />{/if}
</div>
