<template>
  <!-- Opened by the badge on a card. This is the touch route to a photo's
       tags: the previous UI showed them on hover only, which on a phone means
       it showed them never. -->
  <v-bottom-sheet v-model="open">
    <v-card
      v-if="photo"
      class="photo-tags-sheet"
    >
      <div class="photo-tags-head">
        <img
          class="photo-tags-thumb"
          :src="photo.url"
          :alt="`Photo ${photo.id}`"
        >
        <div class="photo-tags-heading">
          <div class="photo-tags-title">
            {{ photo.tags.length }} {{ photo.tags.length === 1 ? 'tag' : 'tags' }}
          </div>
          <div class="photo-tags-stamp">
            {{ stamp }}
          </div>
        </div>
        <v-spacer />
        <v-btn
          class="photo-tags-close"
          icon="mdi-close"
          variant="text"
          size="small"
          aria-label="Close"
          @click="open = false"
        />
      </div>

      <v-divider />

      <div class="photo-tags-body">
        <!-- Tapping a tag filters by it. It is the shortest useful thing a
             reader can do from here — "more like this one" — and it saves
             finding the same word again in a picker of hundreds. -->
        <div
          v-if="photo.tags.length > 0"
          class="photo-tags-chips"
        >
          <v-chip
            v-for="tag in photo.tags"
            :key="tag"
            class="photo-tags-chip"
            size="small"
            variant="outlined"
            append-icon="mdi-filter-variant"
            @click="emit('filter', tag)"
          >
            {{ tag }}
          </v-chip>
        </div>
        <div
          v-else
          class="photo-tags-empty"
        >
          This photo has no tags yet
        </div>
      </div>
    </v-card>
  </v-bottom-sheet>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { formatAbsoluteTime } from '@/utils/time'
import type { Photo } from '@/types'

interface Props {
  photo: Photo | null
}

const props = defineProps<Props>()
const emit = defineEmits<{ filter: [tag: string] }>()

const open = defineModel<boolean>({ default: false })

const stamp = computed(() => {
  const timestamp = props.photo?.timestamp ?? ''
  if (timestamp === '') return 'No date'
  const formatted = formatAbsoluteTime(timestamp)
  return formatted === '' ? 'No date' : formatted
})
</script>

<style scoped>
.photo-tags-sheet {
  background: rgb(var(--v-theme-surface));
}

.photo-tags-head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
}

.photo-tags-thumb {
  width: 56px;
  height: 56px;
  object-fit: cover;
  border-radius: 4px;
  flex: 0 0 auto;
}

.photo-tags-title {
  font-weight: 600;
}

.photo-tags-stamp {
  font-size: 0.75rem;
  color: rgba(var(--v-theme-on-surface), 0.6);
}

.photo-tags-body {
  padding: 12px;
  max-height: 40vh;
  overflow-y: auto;
}

.photo-tags-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.photo-tags-empty {
  font-size: 0.85rem;
  color: rgba(var(--v-theme-on-surface), 0.6);
}
</style>
