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
        <template v-slot:placeholder>
          <div class="d-flex align-center justify-center fill-height">
            <v-progress-circular
              color="grey-lighten-4"
              indeterminate
            ></v-progress-circular>
          </div>
        </template>
        <template v-slot:error>
          <div class="d-flex align-center justify-center fill-height">
            <v-icon icon="mdi-image-broken-variant" color="grey-lighten-2" size="48"></v-icon>
          </div>
        </template>
      </v-img>
      
      <!-- Hidden state overlay -->
      <div v-if="!photo.enabled" class="disabled-overlay">
        <v-icon icon="mdi-eye-off" size="36" color="white" class="eye-off-icon"></v-icon>
      </div>

      <!-- Selection overlay -->
      <div v-if="isSelected" class="selection-overlay"></div>

      <!-- Selection checkmark (top-left) -->
      <div v-if="isSelected" class="selection-checkmark">
        <v-icon icon="mdi-check" size="14" color="white"></v-icon>
      </div>
    </div>
  </v-card>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useSelectionStore } from '@/stores/selection'

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

const isSelected = computed(() => selectionStore.isPhotoSelected(props.photo.id))

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
</style>
