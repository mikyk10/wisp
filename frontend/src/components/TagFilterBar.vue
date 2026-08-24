<template>
  <!-- The row is always here, filtered or not. It carries the photo count,
       which used to sit in the app bar and cost width the bar did not have on
       a narrow screen; keeping the row present regardless means adding a tag
       does not push the grid down a line as you use it. -->
  <div class="tag-filter-bar">
    <div class="tag-filter-count">
      <template v-if="filterTags.length > 0">
        <strong>{{ shown.toLocaleString() }}</strong>
        <span class="tag-filter-of">of {{ total.toLocaleString() }}</span>
      </template>
      <template v-else>
        <strong>{{ shown.toLocaleString() }}</strong>
        <span class="tag-filter-of">photos</span>
      </template>
    </div>

    <!-- Every active tag, wrapping onto as many lines as it takes, each one
         removable where it stands. The previous UI showed the first and a
         "+2", which meant the second and third filters could not be read or
         undone without reopening the picker. -->
    <div
      v-if="filterTags.length > 0"
      class="tag-filter-chips"
    >
      <v-chip
        v-for="tag in filterTags"
        :key="tag"
        class="tag-filter-chip"
        color="primary"
        variant="flat"
        size="x-small"
        closable
        :aria-label="`Remove filter ${tag}`"
        @click:close="emit('remove', tag)"
      >
        {{ tag }}
      </v-chip>

      <v-btn
        class="tag-filter-clear"
        variant="text"
        size="x-small"
        @click="emit('clear')"
      >
        Clear all
      </v-btn>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  /** Photos currently in the grid — the filtered count when a filter is on. */
  shown: number
  /** Photos the catalogue holds without the filter, for the "of N" reading. */
  total: number
  filterTags: string[]
}

defineProps<Props>()
const emit = defineEmits<{ remove: [tag: string]; clear: [] }>()
</script>

<style scoped>
.tag-filter-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 10px;
  padding: 6px 12px;
  /* Leaves room for the timeline, which is fixed to the right edge and would
     otherwise sit on top of the chips at the end of a long line. */
  padding-right: calc(var(--wisp-timeline-width) + 12px);
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.08);
  background: rgb(var(--v-theme-background));
  font-size: 0.8rem;
  color: rgba(var(--v-theme-on-surface), 0.75);
}

.tag-filter-count {
  display: flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.tag-filter-of {
  color: rgba(var(--v-theme-on-surface), 0.55);
}

.tag-filter-chips {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  min-width: 0;
}

.tag-filter-chip {
  max-width: 100%;
}
</style>
