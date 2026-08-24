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
      <!-- tabindex: this div is the only thing in the app that scrolls. The
           page itself is pinned — html/body are overflow:hidden and v-main
           hands out the viewport as a flex column — so the browser's own
           scroller has nothing to move; without a tabindex this element could
           not take focus either, which left Arrow / PageUp / PageDown / Space
           / Home / End doing nothing at all until a photo happened to be
           clicked. With the native scrollbar hidden in favour of the scrubber
           rail there is no drag target here either, so the keyboard is one of
           only three ways to move: wheel, scrubber, keys. -->
      <div
        ref="scrollParentRef"
        class="photo-grid"
        role="listbox"
        tabindex="0"
        aria-label="Photos"
        aria-multiselectable="true"
        @scroll="handleScroll"
        @focusout="handleFocusOut"
      >
        <div
          class="photo-grid-spacer"
          :style="{ height: `${totalSize}px` }"
        >
          <div
            v-for="row in virtualRows"
            :key="row.index"
            class="photo-grid-row"
            :style="{ transform: `translateY(${row.start}px)`, height: `${row.size}px` }"
          >
            <div
              v-for="photo in rowPhotos(row.index)"
              :key="photo.id"
              class="photo-cell"
              :style="{ width: `${itemSize}px`, height: `${itemSize}px` }"
            >
              <PhotoItem :photo="photo" />
            </div>
          </div>
        </div>
      </div>

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
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { usePhotosStore } from '@/stores/photos'
import { useCatalogsStore } from '@/stores/catalogs'
import {
  GRID_HORIZONTAL_PADDING,
  GRID_ITEM_SIZE,
  MOBILE_BREAKPOINT,
  TIMELINE_WIDTH,
} from '@/constants'
import { computeGridLayout } from '@/utils/gridLayout'
import type { Photo } from '@/types'
import PhotoItem from './PhotoItem.vue'

const photosStore = usePhotosStore()
const catalogsStore = useCatalogsStore()
const scrollTimeout = ref<number | null>(null)

const photos = computed((): Photo[] => {
  return photosStore.items
})

const scrollParentRef = ref<HTMLElement | null>(null)
const itemSize = ref<number>(GRID_ITEM_SIZE.desktop)
const columns = ref(1)

const mobileQuery = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT}px)`)

const updateColumns = () => {
  const isMobile = mobileQuery.matches
  const timelineWidth = isMobile ? TIMELINE_WIDTH.mobile : TIMELINE_WIDTH.desktop
  // The layout viewport, deliberately not window.innerWidth: on iOS,
  // innerWidth follows the pinch zoom, and a pinch is a magnifying glass,
  // not a resize — reflowing on it collapsed the grid to one column and
  // pinching back out did not reliably deliver the resize to undo it.
  const layout = computeGridLayout(
    document.documentElement.clientWidth,
    isMobile ? GRID_ITEM_SIZE.mobile : GRID_ITEM_SIZE.desktop,
    timelineWidth + GRID_HORIZONTAL_PADDING,
  )
  columns.value = layout.columns
  itemSize.value = layout.itemSize
}

// The grid virtualizes whole rows; each row holds `columns` photos.
const rowCount = computed(() => Math.ceil(photos.value.length / columns.value))

const virtualizer = useVirtualizer(
  computed(() => ({
    count: rowCount.value,
    getScrollElement: () => scrollParentRef.value,
    estimateSize: () => itemSize.value,
    overscan: 2,
  })),
)

const virtualRows = computed(() => virtualizer.value.getVirtualItems())
const totalSize = computed(() => virtualizer.value.getTotalSize())

const rowPhotos = (rowIndex: number): Photo[] => {
  const start = rowIndex * columns.value
  return photos.value.slice(start, start + columns.value)
}

// Row height is uniform; re-measure when it changes (mobile ⇄ desktop).
watch(itemSize, () => virtualizer.value.measure())

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
// active-timeline-month state shared with the timeline scrubber.
const reportViewport = () => {
  const el = scrollParentRef.value
  if (!el || photos.value.length === 0) return

  const firstVisibleRow = Math.floor(el.scrollTop / itemSize.value)
  const firstVisibleIndex = Math.min(firstVisibleRow * columns.value, photos.value.length - 1)

  photosStore.reportViewport(firstVisibleIndex)
}

// The thumb on the scrubber rail tracks this. Throttled to animation frames
// rather than debounced: the month highlight can afford to settle after the
// scroll stops, a position marker that does the same looks broken.
let fractionFrame: number | null = null
const reportScrollFraction = () => {
  if (fractionFrame !== null) return
  fractionFrame = requestAnimationFrame(() => {
    fractionFrame = null
    const el = scrollParentRef.value
    if (!el) return
    const range = el.scrollHeight - el.clientHeight
    photosStore.reportScrollFraction(range > 0 ? el.scrollTop / range : 0)
  })
}

// Debounced scroll handler
const handleScroll = () => {
  reportScrollFraction()

  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }

  scrollTimeout.value = window.setTimeout(() => {
    reportViewport()
  }, 150)
}

/**
 * Keep the keyboard alive when the virtualizer unmounts the focused card.
 *
 * Scrolling recycles rows, so the PhotoItem holding focus is routinely removed
 * from the DOM under the reader. Focus then falls to <body>, which scrolls
 * nothing, and every key goes dead again until something is clicked — the
 * "it worked a second ago" half of the problem. relatedTarget === null is
 * exactly that case: focus went nowhere rather than somewhere else.
 *
 * Taking it back means the grid holds focus whenever nothing else claims it,
 * which for a single-view gallery is the behaviour you want.
 */
const handleFocusOut = (event: FocusEvent) => {
  if (event.relatedTarget !== null) return

  // A frame later, because the element focus is moving *to* is not always
  // known at focusout time (Vuetify moves focus into an overlay on open).
  requestAnimationFrame(() => {
    // An open menu / sheet / dialog owns focus while it is up; Vuetify hands
    // it back to the activator when it closes.
    if (document.querySelector('.v-overlay--active')) return
    if (document.activeElement !== document.body) return
    scrollParentRef.value?.focus({ preventScroll: true })
  })
}

// Scrubber drags land here as a fraction of the whole list. Written straight
// to scrollTop rather than through scrollToIndex: a drag names a position,
// not a row, and snapping each step to a row boundary would make the grid
// move in visible jumps under a smoothly moving finger.
watch(
  () => photosStore.scrubRequest,
  (req) => {
    const el = scrollParentRef.value
    if (!req || !el) return
    el.scrollTop = req.fraction * (el.scrollHeight - el.clientHeight)
  },
)

// Timeline clicks land in the store as a scroll request; execute it here.
watch(
  () => photosStore.scrollRequest,
  (req) => {
    if (req) {
      virtualizer.value.scrollToIndex(Math.floor(req.index / columns.value), { align: 'start' })
    }
  },
)

onMounted(() => {
  updateColumns()
  window.addEventListener('resize', updateColumns)
  reportViewport()
  // Arrow keys have to work on arrival, not only after the first click.
  // preventScroll: taking focus must not itself move the grid.
  scrollParentRef.value?.focus({ preventScroll: true })
})

onUnmounted(() => {
  window.removeEventListener('resize', updateColumns)

  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }
  if (fractionFrame !== null) {
    cancelAnimationFrame(fractionFrame)
  }
})
</script>

<style scoped>
.photo-grid-container {
  position: relative;
  /* Fill whatever v-main's column leaves after the filter bar — claiming the
     viewport height here while the filter bar sat above pushed the document
     one bar taller than the window. */
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding-right: var(--wisp-timeline-width);
  background: rgb(var(--v-theme-background));
}

.photo-grid-content {
  max-width: none;
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 0;
}

.photo-grid {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  /* No native scrollbar: the scrubber rail is the position indicator now, and
     two bars answering "where am I" side by side — one proportional to rows,
     one to photos — would disagree slightly and both look wrong. Scrolling
     itself (wheel, touch, keys) is untouched; only the bar is gone.
     scrollbar-width covers Firefox and Chromium, the -webkit rule Safari. */
  scrollbar-width: none;
}

.photo-grid::-webkit-scrollbar {
  display: none;
}

/* No focus ring, deliberately. This container is the app's ambient focus: it
   takes focus on mount and takes it back whenever focus would otherwise fall
   to <body>, so it holds focus nearly always — and Chrome matches
   :focus-visible on the programmatic focus, which put a full-height line down
   the edge of the grid from the moment the page loaded. An indicator that is
   always on distinguishes nothing. What a reader needs to see is *which photo*
   they are on, and that belongs on the cards, which keep their own
   :focus-visible ring; the scrubber rail keeps its own too. */
.photo-grid:focus,
.photo-grid:focus-visible {
  outline: none;
}

.photo-grid-spacer {
  position: relative;
  width: 100%;
}

.photo-grid-row {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  display: flex;
}

.photo-cell {
  padding: 2px;
  box-sizing: border-box;
  flex: 0 0 auto;
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
