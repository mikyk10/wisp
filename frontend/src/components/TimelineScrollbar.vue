<template>
  <nav
    ref="scrollbarEl"
    class="timeline-scrollbar"
    aria-label="Photo timeline"
  >
    <div class="timeline-content">
      <button
        v-for="entry in timelineEntries"
        :key="entry.key"
        type="button"
        class="timeline-entry"
        :class="{ 'timeline-entry--active': entry.key === activeEntry }"
        :aria-current="entry.key === activeEntry ? 'true' : undefined"
        @click="scrollToEntry(entry)"
      >
        <div class="timeline-label">
          {{ entry.label }}
        </div>
        <div class="timeline-count">
          {{ entry.count }} photos
        </div>
      </button>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { usePhotosStore } from '@/stores/photos'
import type { TimelineEntry } from '@/types'

const photosStore = usePhotosStore()
const scrollbarEl = ref<HTMLElement | null>(null)

// The active month lives in the photos store; grid scroll reports and
// timeline clicks both funnel through it (no window events, no timer flags).
const activeEntry = computed(() => photosStore.activeTimelineKey)

watch(activeEntry, async () => {
  await nextTick()
  scrollbarEl.value?.querySelector('.timeline-entry--active')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
})

const timelineEntries = computed((): TimelineEntry[] => {
  return photosStore.timelineEntries
})

const scrollToEntry = (entry: TimelineEntry) => {
  photosStore.requestTimelineScroll(entry)
}
</script>

<style scoped>
.timeline-scrollbar {
  position: fixed;
  right: 0;
  top: var(--v-layout-top, 0px);
  bottom: 0;
  width: var(--wisp-timeline-width);
  background: rgb(var(--v-theme-surface));
  border-left: 1px solid rgba(var(--v-theme-primary), 0.12);
  z-index: 100;
  overflow-y: auto;
}

.timeline-content {
  padding: 12px 6px;
}

.timeline-entry {
  /* Native <button> reset: inherit the sidebar's typography and colors */
  display: block;
  width: 100%;
  text-align: left;
  background: none;
  border: none;
  font: inherit;
  color: inherit;
  padding: 8px 10px;
  margin-bottom: 2px;
  border-radius: 0;
  cursor: pointer;
  transition:
    background-color 0.15s ease,
    border-left-color 0.15s ease;
  border-left: 2px solid transparent;
}

@media (hover: hover) {
  .timeline-entry:hover {
    background-color: rgba(var(--v-theme-primary), 0.06);
    border-left-color: rgba(var(--v-theme-primary), 0.4);
  }
}

.timeline-entry:focus-visible {
  outline: 1px solid rgba(var(--v-theme-primary), 0.6);
  outline-offset: -1px;
}

.timeline-entry--active {
  background-color: rgba(var(--v-theme-primary), 0.1);
  border-left-color: rgb(var(--v-theme-primary));
  color: rgb(var(--v-theme-primary));
}

.timeline-label {
  font-size: 11px;
  font-weight: 600;
  line-height: 1.3;
  letter-spacing: 0.3px;
}

.timeline-count {
  font-size: 10px;
  opacity: 0.5;
  margin-top: 2px;
}

.timeline-entry--active .timeline-count {
  opacity: 0.75;
}

/* Mobile support (width follows --wisp-timeline-width automatically) */
@media (max-width: 768px) {
  .timeline-entry {
    padding: 6px 8px;
  }

  .timeline-label {
    font-size: 10px;
  }

  .timeline-count {
    font-size: 9px;
  }
}

/* Scrollbar */
.timeline-scrollbar::-webkit-scrollbar {
  width: 3px;
}

.timeline-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}

.timeline-scrollbar::-webkit-scrollbar-thumb {
  background: rgba(var(--v-theme-primary), 0.2);
  border-radius: 0;
}

.timeline-scrollbar::-webkit-scrollbar-thumb:hover {
  background: rgba(var(--v-theme-primary), 0.4);
}
</style>
