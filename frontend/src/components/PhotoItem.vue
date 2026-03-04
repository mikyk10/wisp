<template>
  <v-card
    class="photo-item"
    :class="{ 'photo-item--selected': isSelected, 'photo-item--disabled': !photo.enabled }"
    @click="handleClick"
    @mouseenter="isHovered = true"
    @mouseleave="isHovered = false"
  >
    <div class="photo-container">
      <v-img
        :src="photo.url"
        :alt="`Photo ${photo.id}`"
        aspect-ratio="1"
        cover
        class="photo-image"
        :class="{ 'photo-image--hovered': isHovered }"
      >
        <template #placeholder>
          <div class="d-flex align-center justify-center fill-height">
            <v-progress-circular
              color="grey-lighten-4"
              indeterminate
            />
          </div>
        </template>
        <template #error>
          <div class="d-flex align-center justify-center fill-height">
            <v-icon
              icon="mdi-image-broken-variant"
              color="grey-lighten-2"
              size="48"
            />
          </div>
        </template>
      </v-img>
      
      <!-- Hidden state overlay -->
      <div
        v-if="!photo.enabled"
        class="disabled-overlay"
      >
        <v-icon
          icon="mdi-eye-off"
          size="36"
          color="white"
          class="eye-off-icon"
        />
      </div>

      <!-- Selection overlay -->
      <div
        v-if="isSelected"
        class="selection-overlay"
      />

      <!-- Selection checkmark (top-left) -->
      <div
        v-if="isSelected"
        class="selection-checkmark"
      >
        <v-icon
          icon="mdi-check"
          size="14"
          color="white"
        />
      </div>

      <!-- Tag overlay (bottom, on hover) -->
      <div
        v-if="isHovered && tags.length > 0"
        class="tag-overlay"
      >
        <span
          v-for="tag in tags"
          :key="tag"
          class="tag-chip"
        >{{ tag }}</span>
      </div>
    </div>
  </v-card>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useSelectionStore } from '@/stores/selection'
import { photosApi } from '@/api/photos'

interface Photo {
  id: number
  url: string
  enabled: boolean
  timestamp: string
}

interface Props {
  photo: Photo
}

const props = defineProps<Props>()
const selectionStore = useSelectionStore()
const isHovered = ref(false)
const tags = ref<string[]>([])
const tagsLoaded = ref(false)

const isSelected = computed(() => selectionStore.isPhotoSelected(props.photo.id))

// Reset when RecycleScroller reuses this component for a different photo
watch(() => props.photo.id, () => {
  tags.value = []
  tagsLoaded.value = false
})

// Fetch tags once on first hover
watch(isHovered, async (hovered) => {
  if (hovered && !tagsLoaded.value) {
    tagsLoaded.value = true
    tags.value = await photosApi.getTags(props.photo.id)
  }
})

const handleClick = () => {
  selectionStore.togglePhotoSelection(props.photo.id)
}
</script>

<style scoped>
.photo-item {
  cursor: pointer;
  transition:
    box-shadow 0.25s ease,
    transform 0.2s ease;
  border-radius: 0;
  overflow: hidden;
  width: 100%;
  min-width: 0;
  background: #0f1117;
}

.photo-item--selected {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: -2px;
}

.selection-overlay {
  position: absolute;
  inset: 0;
  background-color: rgba(255, 255, 255, 0.5);
  z-index: 4;
}

.photo-item--disabled {
  filter: grayscale(75%);
}

.photo-container {
  position: relative;
  width: 100%;
  aspect-ratio: 1;
}

.photo-image {
  transition: transform 0.3s ease;
  width: 100%;
  height: 100%;
}

.photo-image--hovered {
  transform: scale(1.06);
}

.disabled-overlay {
  position: absolute;
  inset: 0;
  background-color: rgba(10, 12, 18, 0.62);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 5;
}

.eye-off-icon {
  opacity: 0.65;
  filter: drop-shadow(0 1px 4px rgba(0, 0, 0, 0.6));
}

.selection-checkmark {
  position: absolute;
  top: 6px;
  left: 6px;
  z-index: 10;
  background-color: rgb(var(--v-theme-primary));
  border-radius: 50%;
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.photo-item:hover {
  box-shadow:
    0 0 20px rgba(0, 210, 168, 0.2),
    0 0 6px rgba(0, 210, 168, 0.1);
  z-index: 1;
}

.tag-overlay {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 20px 6px 6px;
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.8));
  z-index: 3;
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
  pointer-events: none;
}

.tag-chip {
  font-size: 0.6rem;
  line-height: 1.4;
  color: rgba(255, 255, 255, 0.85);
  background: rgba(255, 255, 255, 0.12);
  border-radius: 3px;
  padding: 1px 5px;
}
</style>
