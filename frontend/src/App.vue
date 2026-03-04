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
        <!-- Wrapper div is the flex item with fixed width.
             v-autocomplete fills it with width:100% (Vuetify default).
             menu-props caps the dropdown so long tag names never force a resize. -->
        <div
          class="mr-3"
          style="width: 240px; flex: 0 0 240px; overflow: visible"
        >
          <v-autocomplete
            v-model="selectedTags"
            :items="availableTags"
            multiple
            density="compact"
            hide-details
            variant="outlined"
            placeholder="Filter by tags"
            color="primary"
            :disabled="!currentCatalog || availableTags.length === 0"
            :menu-props="{ maxWidth: 240 }"
          >
            <template #selection="{ item, index }">
              <v-chip
                v-if="index === 0"
                size="small"
                closable
                @click:close.stop="removeTag(item.value)"
              >
                {{ item.title }}
              </v-chip>
              <span
                v-else-if="index === 1"
                class="text-caption ml-1 text-medium-emphasis align-self-center"
              >
                +{{ selectedTags.length - 1 }}
              </span>
            </template>
          </v-autocomplete>
        </div>
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

    <v-main>
      <PhotoGrid ref="gridRef" />
      <TimelineScrollbar :grid-ref="gridRef" />
      <SelectionToolbar />
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useCatalogsStore } from '@/stores/catalogs'
import { usePhotosStore } from '@/stores/photos'
import { useSelectionStore } from '@/stores/selection'
import { photosApi } from '@/api/photos'
import PhotoGrid from './components/PhotoGrid.vue'
import TimelineScrollbar from './components/TimelineScrollbar.vue'
import SelectionToolbar from './components/SelectionToolbar.vue'
import WispLogo from './components/WispLogo.vue'

const catalogsStore = useCatalogsStore()
const photosStore = usePhotosStore()
const selectionStore = useSelectionStore()

const gridRef = ref<InstanceType<typeof PhotoGrid> | null>(null)

const catalogs = computed(() => catalogsStore.catalogs)
const currentCatalog = computed({
  get: () => catalogsStore.currentCatalog,
  set: (val: string) => catalogsStore.setCurrentCatalog(val),
})
const totalPhotos = computed(() => photosStore.totalPhotos)
const selectedCount = computed(() => selectionStore.selectedCount)

const availableTags = ref<string[]>([])

watch(
  () => catalogsStore.currentCatalog,
  async (catalog) => {
    availableTags.value = []
    if (catalog) {
      availableTags.value = await photosApi.getCatalogTags(catalog)
    }
  },
  { immediate: true },
)

const selectedTags = computed({
  get: () => photosStore.filterTags,
  set: async (val: string[]) => {
    photosStore.filterTags = val
    photosStore.resetPhotos()
    await photosStore.loadPhotosStream(catalogsStore.currentCatalog, val)
  },
})

function removeTag(tag: string) {
  selectedTags.value = selectedTags.value.filter((t) => t !== tag)
}

onMounted(() => {
  catalogsStore.initCatalogs()
})
</script>

<style>
/* Global styles */
html,
body {
  margin: 0;
  padding: 0;
  overflow-x: hidden;
  background: #0f1117;
}

.fancy-app-bar {
  border-bottom: 1px solid rgba(0, 210, 168, 0.15) !important;
}

/* Applied directly to the span element to avoid relying on CSS inheritance from Vuetify components.
   Using a class directly on the text element prevents specificity conflicts with .v-app-bar-title. */
.app-title-text {
  font-family: 'Poppins', 'Roboto', sans-serif;
  font-weight: 700;
  letter-spacing: 3px;
  text-transform: uppercase;
  font-size: 1rem;
  color: rgba(255, 255, 255, 0.9);
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
  background: #0f1117;
}

::-webkit-scrollbar-thumb {
  background: rgba(0, 210, 168, 0.25);
  border-radius: 3px;
}

::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 210, 168, 0.5);
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
