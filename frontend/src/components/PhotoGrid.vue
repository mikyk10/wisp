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

    <!-- Load error -->
    <div
      v-if="error"
      class="grid-state d-flex flex-column align-center justify-center"
    >
      <v-alert
        type="error"
        variant="tonal"
        class="grid-error-alert"
        title="Failed to load photos"
        :text="error"
      />
      <v-btn
        color="primary"
        variant="outlined"
        class="mt-4"
        prepend-icon="mdi-refresh"
        @click="retry"
      >
        Retry
      </v-btn>
    </div>

    <!-- Empty catalog (streamCompleted guards against a flash before the first load starts) -->
    <div
      v-else-if="!loading && streamCompleted && photos.length === 0"
      class="grid-state d-flex flex-column align-center justify-center text-center"
    >
      <v-icon
        icon="mdi-image-off-outline"
        size="64"
        class="empty-icon"
      />
      <div class="mt-4 text-h6">
        No photos in this catalog
      </div>
      <div class="mt-1 empty-hint">
        Photos will appear here once they are uploaded.
      </div>
    </div>

    <!-- Photo grid -->
    <v-container
      v-else
      fluid
      class="photo-grid-content"
    >
      <RecycleScroller
        ref="scrollerRef"
        class="photo-grid"
        role="listbox"
        aria-label="Photos"
        aria-multiselectable="true"
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
import { useCatalogsStore } from '@/stores/catalogs'
import {
  GRID_HORIZONTAL_PADDING,
  GRID_ITEM_SIZE,
  MOBILE_BREAKPOINT,
  TIMELINE_WIDTH,
} from '@/constants'
import type { Photo } from '@/types'
import PhotoItem from './PhotoItem.vue'
import { RecycleScroller } from 'vue-virtual-scroller'

const photosStore = usePhotosStore()
const catalogsStore = useCatalogsStore()
const scrollTimeout = ref<number | null>(null)

const photos = computed((): Photo[] => {
  return photosStore.items
})

const scrollerRef = ref<InstanceType<typeof RecycleScroller> | null>(null)
const itemSize = ref<number>(GRID_ITEM_SIZE.desktop)
const buffer = 200
const columns = ref(1)

const mobileQuery = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT}px)`)

const updateColumns = () => {
  const isMobile = mobileQuery.matches
  itemSize.value = isMobile ? GRID_ITEM_SIZE.mobile : GRID_ITEM_SIZE.desktop
  const timelineWidth = isMobile ? TIMELINE_WIDTH.mobile : TIMELINE_WIDTH.desktop
  const available = window.innerWidth - timelineWidth - GRID_HORIZONTAL_PADDING
  columns.value = Math.max(1, Math.floor(available / itemSize.value))
}


const loading = computed(() => {
  return photosStore.loading
})

const error = computed(() => photosStore.error)
const streamCompleted = computed(() => photosStore.streamCompleted)

const retry = () => {
  if (catalogsStore.currentCatalog) {
    catalogsStore.setCurrentCatalog(catalogsStore.currentCatalog)
  }
}

// Report the index of the first visible item to the store, which owns the
// active-timeline-month state shared with TimelineScrollbar.
const reportViewport = () => {
  const el = scrollerRef.value?.$el
  if (!el || photos.value.length === 0) return

  const scrollTop = el.scrollTop
  const firstVisibleRow = Math.floor(scrollTop / itemSize.value)
  const firstVisibleIndex = Math.min(firstVisibleRow * columns.value, photos.value.length - 1)

  photosStore.reportViewport(firstVisibleIndex)
}

// Scroll event handler
const handleScroll = () => {
  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }

  scrollTimeout.value = window.setTimeout(() => {
    reportViewport()
  }, 150)
}

// Timeline clicks land in the store as a scroll request; execute it here.
watch(
  () => photosStore.scrollRequest,
  (req) => {
    if (req) {
      scrollerRef.value?.scrollToItem(req.index)
    }
  },
)

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
      // The scroller is v-if-mounted with the first batch: (re)attach the scroll listener.
      attachScrollListener()
      return
    }

    const el = scrollerRef.value?.$el
    if (!el) return
    const saved = el.scrollTop
    el.scrollTop = saved + 1
    el.scrollTop = saved
  },
)

let listeningEl: HTMLElement | null = null

// Register the scroll event on the RecycleScroller element itself
// (scroll occurs on the overflow-y: auto element, not on window)
const attachScrollListener = () => {
  nextTick(() => {
    const el = scrollerRef.value?.$el
    if (el && el !== listeningEl) {
      listeningEl?.removeEventListener('scroll', handleScroll)
      el.addEventListener('scroll', handleScroll)
      listeningEl = el
      reportViewport()
    }
  })
}

onMounted(() => {
  updateColumns()
  window.addEventListener('resize', updateColumns)
  attachScrollListener()
})

onUnmounted(() => {
  listeningEl?.removeEventListener('scroll', handleScroll)
  listeningEl = null
  window.removeEventListener('resize', updateColumns)

  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }
})
</script>

<style scoped>
.photo-grid-container {
  position: relative;
  height: calc(100vh - var(--v-layout-top, 0px));
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding-right: var(--wisp-timeline-width);
  background: rgb(var(--v-theme-background));
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

.grid-state {
  flex: 1 1 auto;
  min-height: 0;
  padding: 24px;
}

.grid-error-alert {
  max-width: 480px;
}

.empty-icon {
  opacity: 0.35;
}

.empty-hint {
  font-size: 0.85rem;
  opacity: 0.5;
}

.stream-loading {
  color: rgba(var(--v-theme-on-surface), 0.4);
  font-size: 0.8rem;
}

.stream-loading-text {
  letter-spacing: 0.5px;
  color: rgba(var(--v-theme-on-surface), 0.4);
}

/* Bottom margin in selection mode */
.photo-grid-container.selection-mode {
  padding-bottom: 100px;
}
</style>
