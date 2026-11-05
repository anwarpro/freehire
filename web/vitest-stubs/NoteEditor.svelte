<script lang="ts">
  let { value, label, onsave }: { value: string; label: string; onsave: (value: string) => unknown } = $props();
</script>

<!-- EasyMDE needs browser layout. Preserve the parent's live-sync contract in tests. -->
<textarea aria-label={label} {value} oninput={(event) => onsave(event.currentTarget.value)}></textarea>
