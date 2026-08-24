<template>
  <v-app>
    <v-app-bar
      app
      color="surface"
      elevation="0"
      class="fancy-app-bar"
    >
      <v-app-bar-title>
        <div class="d-flex align-center">
          <WispLogo
            :size="36"
            class="mr-2"
          />
          <span class="app-title-text">WiSP</span>
        </div>
      </v-app-bar-title>

      <v-spacer />

      <v-btn
        ref="deviceTrigger"
        class="device-drawer-trigger mr-2"
        icon="mdi-image-frame"
        variant="text"
        size="small"
        aria-label="Displays"
        title="Displays"
        @click="deviceDrawerOpen = true"
      />

      <!-- A 36px button, not a 240px field. The bar has to hold a title, a
           catalogue select and this on a 390px screen; the previous filter UI
           was a fixed-width autocomplete in this row, which is precisely what
           overflowed. What is being filtered lives in TagFilterBar below. -->
      <v-badge
        :model-value="filterTags.length > 0"
        :content="filterTags.length"
        color="primary"
        offset-x="6"
        offset-y="6"
      >
        <v-btn
          id="tag-filter-activator"
          class="tag-filter-trigger mr-2"
          icon="mdi-tag-multiple"
          variant="text"
          size="small"
          aria-label="Filter by tags"
          title="Filter by tags"
          @click="tagPickerOpen = true"
        />
      </v-badge>

      <div class="d-flex align-center">
        <v-select
          v-model="currentCatalog"
          :items="catalogs"
          density="compact"
          hide-details
          variant="outlined"
          class="catalog-select mr-3"
          style="max-width: 150px"
          color="primary"
          item-color="primary"
        />
        <v-chip
          v-if="selectedCount > 0"
          color="primary"
          class="mr-3"
          size="small"
        >
          <v-icon
            icon="mdi-check-circle"
            start
          />
          {{ selectedCount }} selected
        </v-chip>
      </div>
    </v-app-bar>

    <DeviceDrawer v-model="deviceDrawerOpen" />

    <TagPicker
      v-model:open="tagPickerOpen"
      :model-value="filterTags"
      :catalog-key="currentCatalog"
      anchor="#tag-filter-activator"
      @update:model-value="applyTags"
    />

    <PhotoTagsSheet
      v-model="tagSheetOpen"
      :photo="tagSheetPhoto"
      @filter="filterByTag"
    />

    <v-main>
      <!-- Catalog list fetch failure: nothing can be shown, so take over the whole view -->
      <div
        v-if="catalogsError"
        class="app-error d-flex flex-column align-center justify-center text-center"
      >
        <v-icon
          icon="mdi-cloud-alert-outline"
          size="72"
          class="app-error-icon"
        />
        <div class="mt-4 text-h6">
          Failed to load catalogs
        </div>
        <div class="mt-1 app-error-detail">
          {{ catalogsError }}
        </div>
        <v-btn
          class="catalog-retry mt-6"
          color="primary"
          variant="outlined"
          prepend-icon="mdi-refresh"
          @click="catalogsStore.initCatalogs()"
        >
          Retry
        </v-btn>
      </div>
      <template v-else>
        <TagFilterBar
          :shown="totalPhotos"
          :total="catalogTotal"
          :filter-tags="filterTags"
          @remove="removeTag"
          @clear="clearTags"
        />
        <PhotoGrid />
        <TimelineScrubber />
        <SelectionToolbar />
      </template>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useCatalogsStore } from '@/stores/catalogs'
import { usePhotosStore } from '@/stores/photos'
import { useSelectionStore } from '@/stores/selection'
import PhotoGrid from './components/PhotoGrid.vue'
import TagFilterBar from './components/TagFilterBar.vue'
import TagPicker from './components/TagPicker.vue'
import PhotoTagsSheet from './components/PhotoTagsSheet.vue'
import TimelineScrubber from './components/TimelineScrubber.vue'
import SelectionToolbar from './components/SelectionToolbar.vue'
import DeviceDrawer from './components/DeviceDrawer.vue'
import WispLogo from './components/WispLogo.vue'

const catalogsStore = useCatalogsStore()
const photosStore = usePhotosStore()
const selectionStore = useSelectionStore()

/** Right-hand device drawer; see DeviceDrawer.vue for why it is `temporary`. */
const deviceDrawerOpen = ref(false)
const deviceTrigger = ref<ComponentPublicInstance | null>(null)

const catalogs = computed(() => catalogsStore.catalogs)
const catalogsError = computed(() => catalogsStore.error)
const currentCatalog = computed({
  get: () => catalogsStore.currentCatalog,
  set: (val: string) => catalogsStore.setCurrentCatalog(val),
})
const totalPhotos = computed(() => photosStore.totalPhotos)
const selectedCount = computed(() => selectionStore.selectedCount)

const tagPickerOpen = ref(false)
const filterTags = computed(() => photosStore.filterTags)

/**
 * How many photos the catalogue holds with no filter applied.
 *
 * Remembered from the last unfiltered stream rather than asked for, so that
 * "1,204 of 18,443" can be read while a filter is on without a second count
 * query — and so the number does not change as tags are added and removed.
 */
const catalogTotal = ref(0)
watch(
  () => [photosStore.streamCompleted, photosStore.filterTags.length] as const,
  ([completed, filtered]) => {
    if (completed && filtered === 0) catalogTotal.value = photosStore.totalPhotos
  },
)

function applyTags(tags: string[]) {
  void photosStore.setFilterTags(catalogsStore.currentCatalog, tags)
}

function removeTag(tag: string) {
  applyTags(filterTags.value.filter((t) => t !== tag))
}

function clearTags() {
  applyTags([])
}

/** "More photos like this one", from a tag on the per-photo sheet. */
function filterByTag(tag: string) {
  photosStore.hidePhotoTags()
  if (!filterTags.value.includes(tag)) applyTags([...filterTags.value, tag])
}

const tagSheetPhoto = computed(() => photosStore.tagSheetPhoto)
const tagSheetOpen = computed({
  get: () => photosStore.tagSheetPhoto !== null,
  set: (open: boolean) => {
    if (!open) photosStore.hidePhotoTags()
  },
})

/**
 * Escape, resolved in one place.
 *
 * It used to be handled in SelectionToolbar with a plain window listener that
 * looked only at the selection, which made Escape do the wrong thing wherever
 * something was open on top of the grid: dismissing the tag picker also wiped
 * the photo selection behind it, and the displays drawer did not close at all.
 *
 * Registered in the *capture* phase. VOverlay attaches its own window listener
 * per open overlay, and in the bubble phase Vuetify would close the picker
 * first — this handler would then see nothing open and fall through to the
 * selection, which is the bug. Capture runs before any of that, so the
 * priority below is the one that decides.
 */
function handleEscape(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  // Escape while an IME is converting cancels the conversion — it belongs to
  // the input, not to us. keyCode 229 covers browsers that report the
  // composition state that way instead.
  if (event.isComposing || event.keyCode === 229) return

  // 1. Anything Vuetify put up — the tag picker, the per-photo tag sheet, the
  //    catalogue select's menu — closes itself, topmost first, which is what
  //    we want. Yield rather than double-handle it.
  if (document.querySelector('.v-overlay--active')) return

  // 2. VNavigationDrawer is not a VOverlay and ships no Escape handling, so
  //    the displays drawer (and the delivery history inside it) had no way to
  //    be dismissed from the keyboard.
  if (deviceDrawerOpen.value) {
    deviceDrawerOpen.value = false
    // Vuetify returns focus to the activator when an overlay closes; a drawer
    // has no activator to return it to, so focus would be left on an element
    // that is on its way out of the DOM.
    nextTick(() => {
      const el = deviceTrigger.value?.$el
      if (el instanceof HTMLElement) el.focus()
    })
    event.preventDefault()
    return
  }

  // 3. Nothing on top: Escape means "never mind" about the selection.
  if (selectionStore.isSelectionMode) {
    selectionStore.clearSelection()
    event.preventDefault()
  }
}

onMounted(() => {
  catalogsStore.initCatalogs()
  window.addEventListener('keydown', handleEscape, true)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleEscape, true)
})
</script>

<style>
/* Layout constants — JS twin lives in src/constants.ts (keep in sync).
   The 768px breakpoint is repeated in the @media query below because CSS
   media queries cannot read custom properties.
   --wisp-bg mirrors the vuetify.ts background color for the pre-mount
   flash; everything inside <v-app> uses Vuetify's --v-theme-* variables. */
:root {
  --wisp-bg: #0f1117;
  /* The scrubber needs only a finger's width of rail; the month labels float
     over the grid on demand instead of owning a column. The width the old
     sidebar held goes back to the photos. */
  --wisp-timeline-width: 32px;
}

@media (max-width: 768px) {
  :root {
    --wisp-timeline-width: 24px;
  }
}

/* Global styles */
html,
body {
  margin: 0;
  padding: 0;
  /* The page itself never scrolls: every scrolling surface in the app is an
     inner container with its own overflow, and the only position indicator is
     the scrubber. A document scrollbar here means the height maths below has
     broken, and hiding the symptom beats shipping a second, slightly-off
     scrollbar — but the flex column below is the actual fix. */
  overflow: hidden;
  background: var(--wisp-bg);
}

/* One viewport, distributed: v-main pads itself below the app bar, the filter
   bar takes its own height, the grid gets the rest. Nothing states "100vh
   minus the app bar" for itself any more — the grid doing exactly that while
   the filter bar sat above it is what made the document taller than the
   window by one filter bar. */
.v-main {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.fancy-app-bar {
  border-bottom: 1px solid rgba(var(--v-theme-primary), 0.15) !important;
}

.app-error {
  flex: 1 1 auto;
  padding: 24px;
}

.app-error-icon {
  opacity: 0.4;
}

.app-error-detail {
  font-size: 0.85rem;
  opacity: 0.5;
  max-width: 480px;
}

/* Applied directly to the span element to avoid relying on CSS inheritance from Vuetify components.
   Using a class directly on the text element prevents specificity conflicts with .v-app-bar-title. */
.app-title-text {
  font-family: 'Poppins', 'Roboto', sans-serif;
  font-weight: 700;
  letter-spacing: 3px;
  text-transform: uppercase;
  font-size: 1rem;
  color: rgba(var(--v-theme-on-surface), 0.9);
  white-space: nowrap;
}

/* The title is the one thing in this bar that must not be squeezed.
   VAppBarTitle takes the space the controls leave it and hides the overflow,
   so on a 390px screen the wordmark was being cut mid-word — the logo, the
   gap and "WiSP" need 97px and the box had shrunk to 70. Sizing to content
   makes the select give way instead, which it can: it is a fixed-width
   control with room to spare. */
.v-app-bar-title {
  flex: 0 0 auto;
  margin-inline-start: 0;
}

/* Use !important to prioritise Poppins regardless of bundle order */
.v-application {
  font-family: 'Poppins', 'Roboto', sans-serif !important;
}

/* Scrollbar customisation */
::-webkit-scrollbar {
  width: 6px;
}

::-webkit-scrollbar-track {
  background: var(--wisp-bg);
}

::-webkit-scrollbar-thumb {
  background: rgba(var(--v-theme-primary), 0.25);
  border-radius: 3px;
}

::-webkit-scrollbar-thumb:hover {
  background: rgba(var(--v-theme-primary), 0.5);
}

/* Animations */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* Responsive */
@media (max-width: 600px) {
  /* This rule named .app-title, which nothing carries — the element is
     .app-title-text — so it had never applied. The wordmark is set in
     letter-spacing that costs three pixels a character, which is worth
     giving back on a narrow screen. */
  .app-title-text {
    font-size: 0.85rem;
    letter-spacing: 2px;
  }

  /* The catalogue names are short; 150px was never needed on a phone, and
     the width is better spent on the title beside it. */
  .catalog-select {
    max-width: 120px;
  }
}
</style>
