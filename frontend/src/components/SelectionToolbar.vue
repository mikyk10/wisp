<template>
  <v-slide-y-reverse-transition>
    <v-card
      v-if="isSelectionMode"
      class="selection-toolbar"
      elevation="8"
    >
      <v-card-text class="d-flex align-center justify-space-between pa-4">
        <div class="selection-info">
          <v-icon
            icon="mdi-check-circle"
            class="mr-2"
          />
          <span class="text-h6">{{ selectedCount }} selected</span>
        </div>
        
        <div class="toolbar-actions">
          <span
            v-if="error"
            class="error-text mr-4"
          >
            <v-icon
              icon="mdi-alert-circle-outline"
              size="16"
              class="mr-1"
            />{{ error }}
          </span>

          <v-btn
            variant="outlined"
            class="mr-3"
            :disabled="updating"
            @click="clearSelection"
          >
            <v-icon
              icon="mdi-close"
              class="mr-1"
            />
            Cancel
          </v-btn>

          <v-btn
            color="primary"
            :loading="updating"
            :disabled="updating"
            @click="toggleStatus"
          >
            <v-icon
              icon="mdi-toggle-switch"
              class="mr-1"
            />
            Toggle Status
          </v-btn>
        </div>
      </v-card-text>
    </v-card>
  </v-slide-y-reverse-transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useSelectionStore } from '@/stores/selection'

const selectionStore = useSelectionStore()

const isSelectionMode = computed(() => selectionStore.isSelectionMode)
const selectedCount = computed(() => selectionStore.selectedCount)
const updating = computed(() => selectionStore.updating)
const error = computed(() => selectionStore.error)

const clearSelection = () => {
  selectionStore.clearSelection()
}

const toggleStatus = async () => {
  await selectionStore.toggleSelectedPhotosStatus()
}
</script>

<style scoped>
.selection-toolbar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 1000;
  border-radius: 0;
  background: #1a1d27;
  border-top: 1px solid rgba(0, 210, 168, 0.3);
  box-shadow: 0 -4px 24px rgba(0, 0, 0, 0.4);
}

.selection-info {
  display: flex;
  align-items: center;
  color: rgb(var(--v-theme-primary));
  font-weight: 600;
  letter-spacing: 0.5px;
}

.toolbar-actions {
  display: flex;
  align-items: center;
}

.error-text {
  display: flex;
  align-items: center;
  color: rgb(var(--v-theme-error));
  font-size: 0.8rem;
}

@media (max-width: 600px) {
  .selection-toolbar .v-card-text {
    flex-direction: column;
    gap: 16px;
  }

  .toolbar-actions {
    width: 100%;
    justify-content: center;
  }
}
</style>
