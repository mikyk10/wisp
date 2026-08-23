<template>
  <v-card
    class="photo-item"
    :class="{ 'photo-item--selected': isSelected, 'photo-item--disabled': !photo.enabled }"
    role="option"
    tabindex="0"
    :aria-selected="isSelected"
    :aria-label="`Photo ${photo.id}`"
    @click="handleClick"
    @keydown.enter.prevent="handleSelect(false)"
    @keydown.space.prevent="handleSelect(false)"
  >
    <div class="photo-container">
      <v-img
        :src="photo.url"
        :alt="`Photo ${photo.id}`"
        aspect-ratio="1"
        cover
        class="photo-image"
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

      <!-- Tag badge (bottom-right).
           Always drawn, on every device, for a photo that has tags. It is the
           only route to a photo's tags on a touch screen — the previous UI put
           them behind hover, which on a phone means behind nothing — and on a
           pointer device it also says which photos have tags without having to
           sweep the grid to find out.

           @click.stop: the card itself selects, and tapping the badge must not
           also tick the photo. -->
      <button
        v-if="photo.tags.length > 0"
        class="tag-badge"
        type="button"
        :aria-label="`${photo.tags.length} tags on photo ${photo.id}`"
        @click.stop="photosStore.showPhotoTags(photo)"
      >
        <v-icon
          icon="mdi-tag"
          size="10"
        />
        {{ photo.tags.length }}
      </button>

      <!-- Hover preview of the same tags. An enhancement on top of the badge,
           not the way in: it is inside @media (hover: hover) so a touch screen
           never replays it on tap. Nothing is fetched — the tags arrived with
           the listing. -->
      <div
        v-if="photo.tags.length > 0"
        class="tag-overlay"
      >
        <span
          v-for="tag in photo.tags"
          :key="tag"
          class="tag-overlay-chip"
        >{{ tag }}</span>
      </div>

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
    </div>
  </v-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useSelectionStore } from '@/stores/selection'
import { usePhotosStore } from '@/stores/photos'
import type { Photo } from '@/types'

// TODO(lightbox): a zoom/lightbox view needs an endpoint that serves the
// source-resolution image; only thumbnails are exposed today. Once that
// exists, plain click should open the lightbox and selection should move to
// a dedicated checkbox (long-press on touch). handleSelect is already the
// single entry point selection will keep.

interface Props {
  photo: Photo
}

const props = defineProps<Props>()
const selectionStore = useSelectionStore()
const photosStore = usePhotosStore()

const isSelected = computed(() => selectionStore.isPhotoSelected(props.photo.id))

const handleSelect = (extendRange: boolean) => {
  if (extendRange) {
    selectionStore.selectRangeTo(props.photo.id)
  } else {
    selectionStore.togglePhotoSelection(props.photo.id)
  }
}

const handleClick = (event: MouseEvent) => {
  handleSelect(event.shiftKey)
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
  background: rgb(var(--v-theme-background));
  /* Shift+click must extend the selection, not the browser text selection */
  user-select: none;
}

.photo-item--selected {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: -2px;
}

.photo-item:focus-visible {
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

.tag-badge {
  position: absolute;
  right: 4px;
  bottom: 4px;
  z-index: 4;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 1px 5px;
  border-radius: 9px;
  border: none;
  font-size: 0.65rem;
  line-height: 1.5;
  font-variant-numeric: tabular-nums;
  color: rgba(255, 255, 255, 0.92);
  background: rgba(0, 0, 0, 0.55);
  cursor: pointer;
}

.tag-badge:focus-visible {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: 1px;
}

.tag-overlay {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 3;
  display: none;
  flex-wrap: wrap;
  gap: 3px;
  padding: 20px 6px 6px;
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.8));
  /* The card underneath still has to take the click. */
  pointer-events: none;
}

.tag-overlay-chip {
  font-size: 0.6rem;
  line-height: 1.4;
  color: rgba(255, 255, 255, 0.85);
  background: rgba(255, 255, 255, 0.12);
  border-radius: 3px;
  padding: 1px 5px;
}

/* Hover affordances only on devices that actually hover — touch screens
   otherwise replay them on tap ("sticky hover"). */
@media (hover: hover) {
  .photo-item:hover .tag-overlay {
    display: flex;
  }

  /* The overlay says the same thing in full, so the badge stands down while
     it is up rather than sitting on top of it. */
  .photo-item:hover .tag-badge {
    display: none;
  }

  .photo-item:hover .photo-image {
    transform: scale(1.06);
  }

  .photo-item:hover {
    box-shadow:
      0 0 20px rgba(var(--v-theme-primary), 0.2),
      0 0 6px rgba(var(--v-theme-primary), 0.1);
    z-index: 1;
  }
}
</style>
