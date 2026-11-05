<script lang="ts">
  import { applicationDeadlineError } from '#lib/applicationDeadline.js';
  import { Input } from '#lib/ui/index.js';
  let { value = $bindable('') }: { value?: string } = $props();
  const error = $derived(applicationDeadlineError(value));
  const uid = $props.id();
</script>

<div class="flex flex-col gap-1">
  <label class="flex flex-col gap-1 text-sm font-medium">
    Application deadline
    <Input type="date" bind:value aria-invalid={error ? true : undefined} aria-describedby={`${uid}-help`} class="w-full" />
  </label>
  <p id={`${uid}-help`} class="text-xs text-muted-foreground">
    Optional. Applications close at the end of this day in Bangladesh (UTC+06:00).
    Leave blank when unknown; clear the date to remove a saved deadline. An already closed job stays closed.
  </p>
  {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
</div>
