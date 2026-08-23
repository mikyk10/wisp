<template>
  <v-expansion-panel
    :value="device.key"
    class="device-panel"
  >
    <v-expansion-panel-title class="device-panel-title">
      <div class="device-head">
        <div class="device-name">
          {{ device.name }}
        </div>

        <!-- A display that has never been handed an image is the single most
             useful thing this drawer can tell you, so it gets its own line
             rather than an empty timestamp. -->
        <div
          v-if="device.lastDeliveredAt === null"
          class="device-never"
        >
          <v-icon
            icon="mdi-timer-sand-empty"
            size="14"
            class="mr-1"
          />
          Never delivered
        </div>
        <template v-else>
          <div class="device-delivered">
            Last delivered {{ relativeTime }}
          </div>
          <div class="device-stamp">
            {{ absoluteTime }}
          </div>
        </template>

        <DeliveryStrip
          class="mt-2"
          :deliveries="deliveries"
        />
      </div>
    </v-expansion-panel-title>

    <v-expansion-panel-text class="device-panel-text">
      <dl class="device-detail">
        <div class="device-detail-row">
          <dt>Model</dt>
          <dd>{{ modelSummary }}</dd>
        </div>
        <div class="device-detail-row">
          <dt>Catalogs</dt>
          <dd>{{ catalogSummary }}</dd>
        </div>
        <div class="device-detail-row">
          <dt>Configured sleep</dt>
          <dd>{{ sleepSummary }}</dd>
        </div>
        <div class="device-detail-row">
          <dt>Wake schedule</dt>
          <dd>
            <template v-if="device.wakeSchedule.length > 0">
              <code
                v-for="entry in device.wakeSchedule"
                :key="entry"
                class="device-cron"
              >{{ entry }}</code>
            </template>
            <template v-else>
              No wake schedule
            </template>
          </dd>
        </div>
        <div class="device-detail-row">
          <dt>Display key</dt>
          <dd class="device-key">
            {{ device.key }}
          </dd>
        </div>
      </dl>

      <div
        v-if="loading"
        class="device-status device-status--row"
      >
        <v-progress-circular
          indeterminate
          size="16"
          width="2"
          class="mr-2"
        />
        Loading deliveries…
      </div>
      <div
        v-else-if="error"
        class="device-status device-status--error"
      >
        <div>Failed to load deliveries</div>
        <div class="device-status-detail">
          {{ error }}
        </div>
        <v-btn
          class="device-retry mt-2"
          size="small"
          variant="outlined"
          prepend-icon="mdi-refresh"
          @click="emit('retry', device.key)"
        >
          Retry
        </v-btn>
      </div>
      <div
        v-else-if="deliveries.length === 0"
        class="device-status"
      >
        No deliveries recorded.
      </div>
      <template v-else>
        <div class="device-deliveries-caption">
          Recorded deliveries — most recent first
        </div>
        <DeliveryRow
          v-for="(delivery, index) in deliveries"
          :key="`${delivery.deliveredAt}-${index}`"
          :delivery="delivery"
        />
      </template>
    </v-expansion-panel-text>
  </v-expansion-panel>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import DeliveryStrip from './DeliveryStrip.vue'
import DeliveryRow from './DeliveryRow.vue'
import { formatAbsoluteTime, formatDuration, formatRelativeTime } from '@/utils/time'
import type { Delivery, Device } from '@/types'

interface Props {
  device: Device
  deliveries: Delivery[]
  loading: boolean
  error: string | null
}

const props = defineProps<Props>()
const emit = defineEmits<{ retry: [deviceKey: string] }>()

const relativeTime = computed(() => formatRelativeTime(props.device.lastDeliveredAt))
const absoluteTime = computed(() => formatAbsoluteTime(props.device.lastDeliveredAt))

const modelSummary = computed(() => {
  const { model, width, height, orientation } = props.device
  return [model, `${width}×${height}`, orientation].filter(Boolean).join(' · ')
})

const catalogSummary = computed(() =>
  props.device.catalogKeys.length > 0 ? props.device.catalogKeys.join(', ') : 'None'
)

const sleepSummary = computed(() => {
  const duration = formatDuration(props.device.sleepDurationSeconds)
  return duration === '' ? 'Not set' : duration
})
</script>

<style scoped>
.device-panel-title {
  padding-top: 12px;
  padding-bottom: 12px;
}

.device-head {
  min-width: 0;
  width: 100%;
}

.device-name {
  font-size: 0.95rem;
  font-weight: 600;
  letter-spacing: 0.3px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-delivered {
  font-size: 0.78rem;
  color: rgba(var(--v-theme-on-surface), 0.7);
  margin-top: 2px;
}

.device-stamp {
  font-size: 0.72rem;
  color: rgba(var(--v-theme-on-surface), 0.42);
}

/* Dimmed, as agreed — but on its own line with an icon, so it cannot be
   mistaken for a missing value. */
.device-never {
  display: flex;
  align-items: center;
  margin-top: 2px;
  font-size: 0.78rem;
  font-style: italic;
  letter-spacing: 0.4px;
  color: rgba(var(--v-theme-on-surface), 0.45);
}

.device-detail {
  margin: 0 0 12px;
  font-size: 0.76rem;
}

.device-detail-row {
  display: flex;
  gap: 8px;
  padding: 2px 0;
}

.device-detail-row dt {
  flex: 0 0 108px;
  color: rgba(var(--v-theme-on-surface), 0.5);
}

.device-detail-row dd {
  margin: 0;
  min-width: 0;
  color: rgba(var(--v-theme-on-surface), 0.85);
  word-break: break-word;
}

.device-cron {
  display: inline-block;
  margin-right: 6px;
  padding: 0 4px;
  border-radius: 3px;
  background: rgba(var(--v-theme-on-surface), 0.08);
  font-size: 0.72rem;
}

.device-key {
  font-family: ui-monospace, 'SFMono-Regular', Menlo, monospace;
}

.device-status {
  display: flex;
  flex-direction: column;
  padding: 8px 0;
  font-size: 0.76rem;
  color: rgba(var(--v-theme-on-surface), 0.55);
}

.device-status--row {
  flex-direction: row;
  align-items: center;
}

.device-status--error {
  color: rgb(var(--v-theme-error));
  align-items: flex-start;
}

.device-status-detail {
  color: rgba(var(--v-theme-on-surface), 0.5);
}

.device-deliveries-caption {
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 1px;
  color: rgba(var(--v-theme-on-surface), 0.4);
  padding-bottom: 4px;
}
</style>
