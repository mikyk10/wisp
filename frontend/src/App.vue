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
        class="device-drawer-trigger mr-2"
        icon="mdi-image-frame"
        variant="text"
        size="small"
        aria-label="Displays"
        title="Displays"
        @click="deviceDrawerOpen = true"
      />

      <div class="d-flex align-center">
        <v-select
          v-model="currentCatalog"
          :items="catalogs"
          density="compact"
          hide-details
          variant="outlined"
          class="mr-3"
          style="max-width: 150px"
          color="primary"
          item-color="primary"
        />
        <v-chip
          v-if="totalPhotos > 0"
          variant="outlined"
          color="primary"
          class="mr-3"
          size="small"
        >
          <v-icon
            icon="mdi-image-multiple"
            start
          />
          {{ totalPhotos }} photos
        </v-chip>

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
        <PhotoGrid />
        <TimelineScrollbar />
        <SelectionToolbar />
      </template>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useCatalogsStore } from '@/stores/catalogs'
import { usePhotosStore } from '@/stores/photos'
import { useSelectionStore } from '@/stores/selection'
import PhotoGrid from './components/PhotoGrid.vue'
import TimelineScrollbar from './components/TimelineScrollbar.vue'
import SelectionToolbar from './components/SelectionToolbar.vue'
import DeviceDrawer from './components/DeviceDrawer.vue'
import WispLogo from './components/WispLogo.vue'

const catalogsStore = useCatalogsStore()
const photosStore = usePhotosStore()
const selectionStore = useSelectionStore()

/** Right-hand device drawer; see DeviceDrawer.vue for why it is `temporary`. */
const deviceDrawerOpen = ref(false)

const catalogs = computed(() => catalogsStore.catalogs)
const catalogsError = computed(() => catalogsStore.error)
const currentCatalog = computed({
  get: () => catalogsStore.currentCatalog,
  set: (val: string) => catalogsStore.setCurrentCatalog(val),
})
const totalPhotos = computed(() => photosStore.totalPhotos)
const selectedCount = computed(() => selectionStore.selectedCount)

onMounted(() => {
  catalogsStore.initCatalogs()
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
  --wisp-timeline-width: 120px;
}

@media (max-width: 768px) {
  :root {
    --wisp-timeline-width: 80px;
  }
}

/* Global styles */
html,
body {
  margin: 0;
  padding: 0;
  overflow-x: hidden;
  background: var(--wisp-bg);
}

.fancy-app-bar {
  border-bottom: 1px solid rgba(var(--v-theme-primary), 0.15) !important;
}

.app-error {
  height: calc(100vh - var(--v-layout-top, 0px));
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
  .app-title {
    font-size: 0.8rem;
    letter-spacing: 2px;
  }
}
</style>
