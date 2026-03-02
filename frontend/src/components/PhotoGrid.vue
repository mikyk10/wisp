<template>
  <div class="photo-grid-container">
    <!-- Loading indicator -->
    <v-overlay
      v-if="loading && photos.length === 0"
      contained
      class="d-flex align-center justify-center"
    >
      <div class="text-center">
        <v-progress-circular
          indeterminate
          size="64"
          color="primary"
        />
        <div class="mt-4 text-h6">
          Loading photos…
        </div>
      </div>
    </v-overlay>

    <!-- Photo grid -->
    <v-container
      fluid
      class="photo-grid-content"
    >
      <RecycleScroller
        ref="scrollerRef"
        class="photo-grid"
        :items="photos"
        :item-height="itemSize"
        :item-size="itemSize"
        :grid-items="columns"
        :buffer="buffer"
      >
        <template #default="{ item }">
          <PhotoItem :photo="item" />
        </template>
      </RecycleScroller>
      
      <!-- Streaming loading indicator -->
      <div
        v-if="loading && photos.length > 0"
        class="stream-loading d-flex justify-center align-center pa-4"
      >
        <v-progress-circular
          indeterminate
          size="18"
          width="2"
          color="primary"
        />
        <span class="ml-3 stream-loading-text">Loading more…</span>
      </div>
    </v-container>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { usePhotosStore } from '@/stores/photos'
import PhotoItem from './PhotoItem.vue'
import { RecycleScroller } from 'vue-virtual-scroller'

interface Photo {
  id: number
  url: string
  enabled: boolean
  timestamp: string
}

const photosStore = usePhotosStore()
const scrollTimeout = ref<number | null>(null)

const photos = computed((): Photo[] => {
  return photosStore.items
})

const scrollerRef = ref<InstanceType<typeof RecycleScroller> | null>(null)
const itemSize = ref(256)
const buffer = 200
const columns = ref(1)

const updateColumns = () => {
  const isMobile = window.innerWidth <= 768
  itemSize.value = isMobile ? 130 : 256
  const timelineWidth = isMobile ? 80 : 120
  const available = window.innerWidth - timelineWidth - 32
  columns.value = Math.max(1, Math.floor(available / itemSize.value))
}


const loading = computed(() => {
  return photosStore.loading
})

// Calculate the index of the first visible item from scroll position and update the timeline
const updateActiveTimeline = () => {
  const el = scrollerRef.value?.$el
  if (!el || photos.value.length === 0) return

  const scrollTop = el.scrollTop
  const firstVisibleRow = Math.floor(scrollTop / itemSize.value)
  const firstVisibleIndex = Math.min(firstVisibleRow * columns.value, photos.value.length - 1)

  const photo = photos.value[firstVisibleIndex]
  if (!photo) return

  const date = new Date(photo.timestamp)
  const year = date.getFullYear()
  const month = date.getMonth() + 1
  const key = `${year}-${month.toString().padStart(2, '0')}`

  window.dispatchEvent(new CustomEvent('viewport-timeline-update', { detail: { key } }))
}

// Scroll event handler
const handleScroll = () => {
  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }

  scrollTimeout.value = window.setTimeout(() => {
    updateActiveTimeline()
  }, 150)
}

// When photos are added via the stream, RecycleScroller may not re-render visible items (black gap bug).
// Force an internal recalculation by nudging the scroll position by 1px.
watch(
  () => photos.value.length,
  async (newLen, oldLen) => {
    await nextTick()

    if (oldLen === 0 && newLen > 0) {
      // On first load: updateVisibleItems(true) called on items change
      // involves removeAndRecycleAllViews() which may not work correctly in beta.
      // Work around it by calling updateVisibleItems(false) after rAF, same as on resize.
      requestAnimationFrame(() => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        ;(scrollerRef.value as any)?.updateVisibleItems?.(false)
      })
      return
    }

    const el = scrollerRef.value?.$el
    if (!el) return
    const saved = el.scrollTop
    el.scrollTop = saved + 1
    el.scrollTop = saved
  },
)

onMounted(() => {
  updateColumns()
  window.addEventListener('resize', updateColumns)

  // Register the scroll event on the RecycleScroller element itself
  // (scroll occurs on the overflow-y: auto element, not on window)
  nextTick(() => {
    const el = scrollerRef.value?.$el
    if (el) {
      el.addEventListener('scroll', handleScroll)
      updateActiveTimeline()
    }
  })
})

onUnmounted(() => {
  const el = scrollerRef.value?.$el
  if (el) {
    el.removeEventListener('scroll', handleScroll)
  }
  window.removeEventListener('resize', updateColumns)

  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }
})

defineExpose({
  scrollToIndex: (index: number) => scrollerRef.value?.scrollToItem(index)
})
</script>

<style scoped>
.photo-grid-container {
  position: relative;
  height: calc(100vh - var(--v-layout-top, 0px));
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding-right: 120px;
  background: #0f1117;
}

.photo-grid-content {
  max-width: none;
  height: calc(100vh - var(--v-layout-top, 0px));
  display: flex;
  flex-direction: column;
  padding: 0;
}

.photo-grid {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
}

.photo-grid :deep(.vue-recycle-scroller__item-wrapper) {
  padding: 2px;
  box-sizing: border-box;
}

.stream-loading {
  color: rgba(255, 255, 255, 0.4);
  font-size: 0.8rem;
}

.stream-loading-text {
  letter-spacing: 0.5px;
  color: rgba(255, 255, 255, 0.4);
}

/* Mobile support */
@media (max-width: 768px) {
  .photo-grid-container {
    padding-right: 80px;
  }
}

/* Bottom margin in selection mode */
.photo-grid-container.selection-mode {
  padding-bottom: 100px;
}
</style>
