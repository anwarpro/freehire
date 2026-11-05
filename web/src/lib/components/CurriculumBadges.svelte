<script lang="ts">
  import { curriculumLabel } from '#lib/curriculum.js';
  import type { CurriculumTrack } from '#lib/curriculum.js';
  let { ids, tracks }: { ids: string[]; tracks: CurriculumTrack[] } = $props();
</script>
<div class="flex flex-wrap gap-2" aria-label="Curriculum tracks">
  {#each ids as id (id)}<span class="max-w-full break-words rounded-full border border-border bg-muted px-2.5 py-1 text-xs font-medium">{curriculumLabel(id, tracks)}</span>{/each}
  {#if ids.length === 0}<span class="text-sm text-muted-foreground">No matching track</span>{/if}
</div>
