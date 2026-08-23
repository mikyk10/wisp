<template>
  <div
    v-if="oldestFirst.length > 0"
    class="delivery-strip"
    role="img"
    :aria-label="ariaLabel"
  >
    <span
      v-for="(delivery, index) in oldestFirst"
      :key="`${delivery.deliveredAt}-${index}`"
      class="delivery-glyph"
      :class="`delivery-glyph--${presentationFor(delivery.kind).tone}`"
      :title="glyphTitle(delivery)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { presentationFor, reasonTextFor } from '@/utils/deliveries'
import { formatRelativeTime } from '@/utils/time'
import type { Delivery } from '@/types'

/**
 * One glyph per recorded delivery, oldest on the left.
 *
 * This is the point of the drawer: a display that has quietly started failing
 * shows a solid block of error-coloured glyphs at the right-hand end, and it
 * does so in the collapsed panel header, without anyone having to open
 * anything or read a number. Errors are also drawn taller than the other
 * glyphs, so a run stays legible without relying on colour alone.
 */

interface Props {
  deliveries: Delivery[]
}

const props = defineProps<Props>()

/**
 * The API returns deliveries newest first; time reads left to right here, so
 * the list is reversed for display. The store's copy keeps the wire order.
 */
const oldestFirst = computed(() => [...props.deliveries].reverse())

const errorCount = computed(() => props.deliveries.filter((d) => d.kind === 'error').length)

const ariaLabel = computed(() => {
  const total = props.deliveries.length
  const failed = errorCount.value
  const base = `${total} recorded deliveries, oldest first`
  return failed > 0 ? `${base}, ${failed} failed` : base
})

/**
 * Hovering a glyph names the delivery, and for a failure says why — so a run
 * of errors can be read without expanding the panel it sits in.
 */
function glyphTitle(delivery: Delivery): string {
  const age = formatRelativeTime(delivery.deliveredAt)
  const label = presentationFor(delivery.kind).label
  const heading = age === '' ? label : `${label} · ${age}`
  const reason = reasonTextFor(delivery.reason)
  return reason === '' ? heading : `${heading}\n${reason}`
}
</script>

<style scoped>
.delivery-strip {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 14px;
  /* Long histories shrink rather than wrap: the shape of the run matters more
     than any individual glyph, and a wrapped strip loses that shape. */
  min-width: 0;
  overflow: hidden;
}

.delivery-glyph {
  flex: 0 1 4px;
  min-width: 2px;
  border-radius: 1px;
}

.delivery-glyph--photo {
  height: 10px;
  background: rgba(var(--v-theme-primary), 0.75);
}

.delivery-glyph--generated {
  height: 10px;
  background: rgba(var(--v-theme-on-surface), 0.3);
}

.delivery-glyph--error {
  height: 14px;
  background: rgb(var(--v-theme-error));
}

.delivery-glyph--unknown {
  height: 6px;
  background: rgba(var(--v-theme-on-surface), 0.18);
}
</style>
