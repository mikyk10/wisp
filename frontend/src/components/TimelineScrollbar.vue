<template>
  <div ref="scrollbarEl" class="timeline-scrollbar">
    <div class="timeline-content">
      <div
        v-for="entry in timelineEntries"
        :key="entry.key"
        class="timeline-entry"
        :class="{ 'timeline-entry--active': entry.key === activeEntry }"
        @click="scrollToEntry(entry)"
      >
        <div class="timeline-label">{{ entry.label }}</div>
        <div class="timeline-count">{{ entry.count }} photos</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch, onMounted, onUnmounted } from 'vue'
import { usePhotosStore } from '@/stores/photos'
import type PhotoGrid from './PhotoGrid.vue'

interface Props {
  gridRef: InstanceType<typeof PhotoGrid> | null
}

const props = defineProps<Props>()

interface TimelineEntry {
  key: string
  label: string
  year: number
  month: number
  startIndex: number
  count: number
}

const photosStore = usePhotosStore()
const activeEntry = ref<string>('')
const scrollbarEl = ref<HTMLElement | null>(null)
// Flag to prevent the scroll event from overwriting activeEntry after a programmatic scroll triggered by a click.
// Released after 300ms, longer than the 150ms debounce.
let ignoreNextScrollUpdate = false

watch(activeEntry, async () => {
  await nextTick()
  scrollbarEl.value?.querySelector('.timeline-entry--active')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
})

const timelineEntries = computed((): TimelineEntry[] => {
  return photosStore.timelineEntries
})

const scrollToEntry = (entry: TimelineEntry) => {
  activeEntry.value = entry.key
  // Use RecycleScroller's scrollToItem directly.
  // Double-scrolling via DOM queries or timeline-scroll events causes a "snap back to latest" bug.
  props.gridRef?.scrollToIndex(entry.startIndex)
  // Prevent scroll events triggered by the programmatic scroll from overwriting activeEntry with a different month.
  // scrollToItem snaps to column boundaries, so firstVisibleIndex can be off by one.
  ignoreNextScrollUpdate = true
  setTimeout(() => {
    ignoreNextScrollUpdate = false
  }, 300)
}

// Handler for viewport update events
const handleViewportUpdate = (event: CustomEvent) => {
  if (ignoreNextScrollUpdate) return
  const { key } = event.detail
  activeEntry.value = key
}

onMounted(() => {
  // Add listener for viewport update events
  window.addEventListener('viewport-timeline-update', handleViewportUpdate as EventListener)
})

onUnmounted(() => {
  // Remove event listener
  window.removeEventListener('viewport-timeline-update', handleViewportUpdate as EventListener)
})
</script>

<style scoped>
.timeline-scrollbar {
  position: fixed;
  right: 0;
  top: var(--v-layout-top, 0px);
  bottom: 0;
  width: 120px;
  background: #1a1d27;
  border-left: 1px solid rgba(0, 210, 168, 0.12);
  z-index: 100;
  overflow-y: auto;
}

.timeline-content {
  padding: 12px 6px;
}

.timeline-entry {
  padding: 8px 10px;
  margin-bottom: 2px;
  border-radius: 0;
  cursor: pointer;
  transition:
    background-color 0.15s ease,
    border-left-color 0.15s ease;
  border-left: 2px solid transparent;
}

.timeline-entry:hover {
  background-color: rgba(0, 210, 168, 0.06);
  border-left-color: rgba(0, 210, 168, 0.4);
}

.timeline-entry--active {
  background-color: rgba(0, 210, 168, 0.1);
  border-left-color: #00d2a8;
  color: #00d2a8;
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

/* Mobile support */
@media (max-width: 768px) {
  .timeline-scrollbar {
    width: 80px;
  }

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
  background: rgba(0, 210, 168, 0.2);
  border-radius: 0;
}

.timeline-scrollbar::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 210, 168, 0.4);
}
</style>
