<template>
  <!-- `temporary` is deliberate: this layout is hand-rolled with position:fixed
       (TimelineScrubber pins itself to right:0, PhotoGrid compensates with
       padding-right, and the column maths reads window.innerWidth directly).
       A permanent drawer would shift --v-layout-right, which none of that
       observes, and the drawer would sit on top of the timeline. An overlay
       participates in no layout offsets and needs no change to either. -->
  <v-navigation-drawer
    v-model="open"
    location="right"
    temporary
    :width="420"
    class="device-drawer"
  >
    <div class="device-drawer-head">
      <v-icon
        icon="mdi-image-frame"
        size="20"
        class="mr-3"
      />
      <div class="device-drawer-heading">
        <div class="device-drawer-title">
          Displays
        </div>
        <div
          v-if="recentWindow > 0"
          class="device-drawer-subtitle"
        >
          Most recent {{ recentWindow }} deliveries per display
        </div>
      </div>
      <v-spacer />
      <v-btn
        class="device-drawer-refresh"
        icon="mdi-refresh"
        variant="text"
        size="small"
        aria-label="Refresh"
        :disabled="loading"
        @click="devicesStore.refresh()"
      />
      <v-btn
        class="device-drawer-close"
        icon="mdi-close"
        variant="text"
        size="small"
        aria-label="Close"
        @click="open = false"
      />
    </div>

    <v-divider />

    <!-- Recording off: the counts and timestamps below go on reporting real
         facts about the past while nothing new is being written, and a reader
         taking them as current would conclude a frame had stopped when what
         stopped was the bookkeeping. Nothing is hidden or zeroed — the numbers
         are true, they are just no longer moving. -->
    <div
      v-if="!recordingEnabled"
      class="device-drawer-recording"
    >
      <v-icon
        icon="mdi-record-circle-outline"
        size="16"
        color="warning"
        class="mr-2"
      />
      <span>{{ RECORDING_OFF_NOTICE }}</span>
    </div>

    <div class="device-drawer-body">
      <div
        v-if="loading && devices.length === 0"
        class="device-drawer-status"
      >
        <v-progress-circular
          indeterminate
          size="20"
          width="2"
          class="mr-2"
        />
        Loading displays…
      </div>

      <div
        v-else-if="error"
        class="device-drawer-status device-drawer-status--error"
      >
        <div class="text-body-2">
          Failed to load displays
        </div>
        <div class="device-drawer-status-detail">
          {{ error }}
        </div>
        <v-btn
          class="device-drawer-retry mt-4"
          color="primary"
          variant="outlined"
          size="small"
          prepend-icon="mdi-refresh"
          @click="devicesStore.refresh()"
        >
          Retry
        </v-btn>
      </div>

      <div
        v-else-if="devices.length === 0"
        class="device-drawer-status"
      >
        No displays are configured.
      </div>

      <v-expansion-panels
        v-else
        v-model="openKey"
        variant="accordion"
        class="device-drawer-panels"
      >
        <DeviceListItem
          v-for="device in devices"
          :key="device.key"
          :device="device"
          :deliveries="devicesStore.deliveriesFor(device.key)"
          :loading="devicesStore.isDeliveryLoading(device.key)"
          :error="devicesStore.deliveryErrorFor(device.key)"
          @retry="(key: string) => devicesStore.loadDeliveries(key, true)"
        />
      </v-expansion-panels>
    </div>

    <v-divider />

    <p class="device-drawer-footnote">
      {{ FOOTNOTE }}
    </p>
  </v-navigation-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import DeviceListItem from './DeviceListItem.vue'
import { useDevicesStore } from '@/stores/devices'

/**
 * The device drawer: which displays exist, and what the server last sent them.
 *
 * The standing caveat below is not boilerplate. Everything in here is the
 * server's own record of writing bytes to a response; a frame can be offline,
 * flat, or still showing something from yesterday, and no endpoint can tell
 * the difference. The wording throughout says "delivered" for that reason,
 * and never "showing", "displaying", or "online".
 */
const FOOTNOTE =
  '"Delivered" means the server sent the image. It cannot confirm the frame displayed it.'

const RECORDING_OFF_NOTICE =
  'Delivery recording is off. The counts and times below are real, but they stopped updating when recording did.'

const open = defineModel<boolean>({ default: false })

const devicesStore = useDevicesStore()

const devices = computed(() => devicesStore.devices)
const recentWindow = computed(() => devicesStore.recentWindow)
const recordingEnabled = computed(() => devicesStore.recordingEnabled)
const loading = computed(() => devicesStore.loading)
const error = computed(() => devicesStore.error)

/** Key of the expanded panel, or undefined when the accordion is closed. */
const openKey = ref<string | undefined>(undefined)

watch(
  open,
  async (isOpen) => {
    if (!isOpen) return
    // Deliveries are fetched for every display, not just the expanded one:
    // the collapsed strips are the feature, and the list endpoint carries
    // counts rather than the sequence they draw.
    await devicesStore.loadDevices()
    await devicesStore.loadAllDeliveries()
  },
  { immediate: true }
)

// Covers the display whose prefetch failed while the drawer was opening:
// loadDeliveries is a no-op for anything already cached.
watch(openKey, (key) => {
  if (key === undefined) return
  devicesStore.loadDeliveries(key)
})
</script>

<style scoped>
.device-drawer-head {
  display: flex;
  align-items: center;
  padding: 12px 16px;
}

.device-drawer-heading {
  min-width: 0;
}

.device-drawer-title {
  font-family: 'Poppins', 'Roboto', sans-serif;
  font-weight: 700;
  letter-spacing: 2px;
  text-transform: uppercase;
  font-size: 0.85rem;
  color: rgba(var(--v-theme-on-surface), 0.9);
}

.device-drawer-subtitle {
  font-size: 0.7rem;
  color: rgba(var(--v-theme-on-surface), 0.45);
}

.device-drawer-recording {
  display: flex;
  align-items: flex-start;
  padding: 8px 16px;
  font-size: 0.72rem;
  line-height: 1.4;
  background: rgba(var(--v-theme-warning), 0.07);
  color: rgba(var(--v-theme-on-surface), 0.75);
}

.device-drawer-body {
  /* Head, footnote and their dividers are fixed; only the list scrolls. */
  flex: 1 1 auto;
  overflow-y: auto;
}

.device-drawer-status {
  display: flex;
  align-items: center;
  padding: 24px 16px;
  font-size: 0.8rem;
  color: rgba(var(--v-theme-on-surface), 0.55);
}

.device-drawer-status--error {
  flex-direction: column;
  align-items: flex-start;
  color: rgb(var(--v-theme-error));
}

.device-drawer-status-detail {
  margin-top: 4px;
  font-size: 0.75rem;
  color: rgba(var(--v-theme-on-surface), 0.5);
}

.device-drawer-footnote {
  margin: 0;
  padding: 10px 16px;
  font-size: 0.7rem;
  line-height: 1.4;
  color: rgba(var(--v-theme-on-surface), 0.45);
}

.device-drawer-panels {
  border-radius: 0;
}
</style>

<style>
/* The drawer content is a fixed head/footnote with a scrolling list between
   them; Vuetify's own content wrapper has to become the flex column for that
   to work, so this rule is intentionally unscoped. */
.device-drawer .v-navigation-drawer__content {
  display: flex;
  flex-direction: column;
}
</style>
