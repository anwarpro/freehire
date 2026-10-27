<script lang="ts">
  import { asset } from '$app/paths';
  let { class: cls = 'size-7 shrink-0' }: { class?: string } = $props();
</script>

<img src={asset('ph-jobs-logo.svg')} class={cls} alt="" aria-hidden="true" width="28" height="28" />
