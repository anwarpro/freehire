<script lang="ts">
  let { value, onChange }: { value: string; onChange: (value: string) => void } = $props();
</script>

<div class="mb-5 space-y-2 rounded-lg border border-border p-3">
  <label class="block text-sm font-semibold">
    Search job text for a skill
    <input type="search" {value} oninput={(event) => onChange(event.currentTarget.value)} maxlength="200"
      placeholder="Any skill, e.g. Mongoose" class="mt-2 w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-normal" />
  </label>
  <p class="text-xs text-muted-foreground">Search any skill in job text, including skills outside the curriculum or the list below. This edits the main search query. Use the skill selections below for tagged skills and Any/All matching.</p>
</div>
