<template>
  <div class="delivery-row">
    <!-- No <img> is created unless the backing image is still there: see the
         comment on `thumbnailUrl`. -->
    <img
      v-if="thumbnailUrl"
      class="delivery-thumb"
      :src="thumbnailUrl"
      :alt="`Image ${delivery.imageId}`"
      loading="lazy"
    >
    <div
      v-else
      class="delivery-thumb delivery-thumb--placeholder"
      :class="`delivery-thumb--${presentation.tone}`"
    >
      <v-icon
        :icon="placeholderIcon"
        size="18"
      />
    </div>

    <div class="delivery-body">
      <div class="delivery-line">
        <span class="delivery-kind">{{ kindLine }}</span>
        <span class="delivery-age">{{ relativeTime }}</span>
      </div>
      <div class="delivery-stamp">
        {{ absoluteTime }}
      </div>
      <!-- For an error row this is the only line that says what to go and fix. -->
      <div
        v-if="reasonText"
        class="delivery-reason"
      >
        {{ reasonText }}
      </div>
      <div
        v-if="sourceName"
        class="delivery-source"
        :title="delivery.source ?? ''"
      >
        {{ sourceName }}
      </div>
      <div
        v-if="unavailable"
        class="delivery-note"
      >
        Image no longer available
      </div>
      <div
        v-if="sleepPhrase"
        class="delivery-sleep"
      >
        {{ sleepPhrase }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { buildImageUrl } from '@/config'
import { presentationFor, reasonTextFor } from '@/utils/deliveries'
import { formatAbsoluteTime, formatDuration, formatRelativeTime } from '@/utils/time'
import type { Delivery } from '@/types'

interface Props {
  delivery: Delivery
}

const props = defineProps<Props>()

const presentation = computed(() => presentationFor(props.delivery.kind))

/**
 * The image URL, or null when there is nothing safe to point an <img> at.
 *
 * `imageAvailable: false` must not become a broken-image icon, because it
 * would not be one: the image endpoint answers a deleted photo with a fully
 * decodable error card under a 404 status, which the browser draws like any
 * other picture. A removed photo would then be indistinguishable from a
 * successful delivery. So the element is never created in the first place.
 */
const thumbnailUrl = computed<string | null>(() => {
  const { kind, imageAvailable, imageId, catalogKey } = props.delivery
  if (kind !== 'photo' || !imageAvailable || imageId === null || catalogKey === null) return null
  return buildImageUrl(catalogKey, imageId)
})

/** A photo whose file has since gone; every other kind simply has no thumbnail. */
const unavailable = computed(() => props.delivery.kind === 'photo' && !props.delivery.imageAvailable)

const placeholderIcon = computed(() =>
  unavailable.value ? 'mdi-image-off-outline' : presentation.value.icon
)

/**
 * The row's primary line: what the server sent, and which catalogue it was
 * read from — "Photo · artwork", "Error image · photos".
 *
 * The key is appended whenever the record carries one, and on no other basis.
 * That mirrors the field's own definition on the server — the catalogue that
 * was consulted, empty when none was — so the rule needs no list of kinds to
 * maintain: a colour bar consults nothing, a handler-level failure (unknown
 * display, no provider) never got as far as choosing, and both name nothing
 * rather than showing a placeholder. A kind added after this build gets the
 * same treatment without being enumerated here.
 *
 * It is deliberately not conditioned on how many catalogues the display is
 * currently configured with. A delivery is a record of the past and
 * `Device.catalogKeys` is present configuration, so suppressing the key on a
 * display that happens to hold one catalogue today would make its absence
 * ambiguous — no catalogue consulted, or one catalogue configured? — and would
 * render two panels of the same drawer to two different rules.
 */
const kindLine = computed(() => {
  const label = presentation.value.label
  const { catalogKey } = props.delivery
  return catalogKey ? `${label} · ${catalogKey}` : label
})

const reasonText = computed(() => reasonTextFor(props.delivery.reason))

const relativeTime = computed(() => formatRelativeTime(props.delivery.deliveredAt))
const absoluteTime = computed(() => formatAbsoluteTime(props.delivery.deliveredAt))

/** Trailing path segment; the full path stays in the title attribute. */
const sourceName = computed(() => {
  const source = props.delivery.source
  if (!source) return ''
  return source.split('/').pop() ?? source
})

/**
 * What the server told the frame to do next — a request, not an observation.
 * The frame may have ignored it, or never woken to read it.
 */
const sleepPhrase = computed(() => {
  const duration = formatDuration(props.delivery.requestedSleepSeconds)
  return duration === '' ? '' : `asked to sleep ${duration}`
})
</script>

<style scoped>
.delivery-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 8px 0;
  border-top: 1px solid rgba(var(--v-theme-on-surface), 0.08);
}

.delivery-thumb {
  width: 56px;
  height: 42px;
  flex: 0 0 auto;
  object-fit: cover;
  border-radius: 3px;
  background: rgba(var(--v-theme-on-surface), 0.06);
}

.delivery-thumb--placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(var(--v-theme-on-surface), 0.45);
}

.delivery-thumb--error {
  color: rgb(var(--v-theme-error));
  background: rgba(var(--v-theme-error), 0.1);
}

.delivery-body {
  min-width: 0;
  flex: 1 1 auto;
}

.delivery-line {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.delivery-kind {
  font-size: 0.82rem;
  font-weight: 600;
  color: rgba(var(--v-theme-on-surface), 0.9);
}

.delivery-age {
  font-size: 0.75rem;
  color: rgba(var(--v-theme-on-surface), 0.6);
  white-space: nowrap;
}

.delivery-stamp {
  font-size: 0.72rem;
  color: rgba(var(--v-theme-on-surface), 0.45);
}

.delivery-source,
.delivery-note,
.delivery-sleep {
  font-size: 0.72rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Wraps rather than truncates: it is a sentence, and half of it is no use. */
.delivery-reason {
  margin-top: 1px;
  font-size: 0.74rem;
  line-height: 1.35;
  color: rgba(var(--v-theme-on-surface), 0.75);
}

.delivery-source {
  color: rgba(var(--v-theme-on-surface), 0.6);
}

.delivery-note {
  color: rgba(var(--v-theme-on-surface), 0.5);
  font-style: italic;
}

.delivery-sleep {
  color: rgba(var(--v-theme-on-surface), 0.5);
}
</style>
