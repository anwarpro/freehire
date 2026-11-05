<script lang="ts">
  import { postingDateError, postingDateToday } from '#lib/postingDate.js';
  import { Input } from '#lib/ui/index.js';

  let { value = $bindable(''), label = 'Posting date', help }: {
    value?: string;
    label?: string;
    help: string;
  } = $props();
  const error = $derived(postingDateError(value));
  const uid = $props.id();
</script>

<div class="flex flex-col gap-1">
  <label class="flex flex-col gap-1 text-sm font-medium">
    {label}
    <Input type="date" bind:value max={postingDateToday()} aria-invalid={error ? true : undefined} aria-describedby={`${uid}-help`} class="w-full" />
  </label>
  <p id={`${uid}-help`} class="text-xs text-muted-foreground">{help} Dates use Bangladesh time (UTC+06:00).</p>
  {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
</div>
