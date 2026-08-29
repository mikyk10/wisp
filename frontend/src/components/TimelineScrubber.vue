<template>
  <!-- A slider, because that is what it is: one continuous value (position in
       the catalogue), adjustable by pointer or keyboard, with a text rendering
       of the current value for assistive tech. The month list this replaces
       was a column of buttons; the slider keeps it keyboard-reachable without
       keeping the column. -->
  <div
    v-if="totalPhotos > 0"
    ref="railEl"
    class="timeline-scrubber"
    :class="{ 'timeline-scrubber--engaged': engaged }"
    role="slider"
    tabindex="0"
    aria-label="Photo timeline"
    aria-orientation="vertical"
    :aria-valuemin="0"
    :aria-valuemax="100"
    :aria-valuenow="Math.round(viewportFraction * 100)"
    :aria-valuetext="viewportLabel"
    @pointerdown="onPointerDown"
    @pointermove="onPointerMove"
    @pointerup="onPointerUp"
    @pointercancel="onPointerUp"
    @pointerenter="hovering = true"
    @pointerleave="onPointerLeave"
    @keydown="onKeyDown"
  >
    <div class="scrubber-track" />

    <!-- Year marks sit where each year's newest photo sits, so their spacing
         is the size of the year: a dense year is a long stretch of rail, a
         thin one a short stride. Years that would overlap are thinned in the
         mapping layer, not squeezed here. -->
    <div
      v-for="mark in marks"
      :key="mark.year"
      class="scrubber-year"
      :style="{ top: railTop(mark.fraction, 8) }"
    >
      {{ mark.year }}
    </div>

    <div
      class="scrubber-thumb"
      :style="{ top: railTop(viewportFraction, 4) }"
    />

    <!-- While the pointer works the rail, the bubble answers "what is under
         my finger"; while the grid merely scrolls, it answers "where am I
         now" and rides the thumb. Both are the same question — which month is
         at this fraction — asked of a different fraction. -->
    <div
      v-if="bubbleVisible"
      class="scrubber-bubble"
      :style="{ top: railTop(bubbleFraction, 14) }"
    >
      {{ bubbleLabel }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { usePhotosStore } from '@/stores/photos'
import {
  clampFraction,
  entryForIndex,
  fractionToIndex,
  yearMarks,
} from '@/utils/scrubber'

const photosStore = usePhotosStore()

const railEl = ref<HTMLElement | null>(null)
const railHeight = ref(0)

const totalPhotos = computed(() => photosStore.totalPhotos)
const entries = computed(() => photosStore.timelineEntries)
const viewportFraction = computed(() => photosStore.viewportFraction)

/** Label for a rail position; "No date" is for photos that genuinely have none. */
function labelAt(fraction: number): string {
  const entry = entryForIndex(entries.value, fractionToIndex(fraction, totalPhotos.value))
  return entry ? entry.label : 'No date'
}

const viewportLabel = computed(() => labelAt(viewportFraction.value))

/**
 * CSS `top` for an element centred on a rail fraction with translateY(-50%).
 * Clamped by half the element's height so the ends of the rail do not push
 * half of it out of the rail — the first year mark sits at fraction 0, and
 * unclamped its upper half disappears under the app bar.
 */
function railTop(fraction: number, halfHeightPx: number): string {
  return `clamp(${halfHeightPx}px, ${clampFraction(fraction) * 100}%, calc(100% - ${halfHeightPx}px))`
}

// ── Engagement ─────────────────────────────────────────────────────────────
// The rail is a thin line until someone is using it — pointer on it, drag in
// progress, or the grid in motion. Scrolling counts as using it because the
// whole point is knowing where you are *while* you move; once everything has
// been still for a moment the labels have answered their question and fade.
const SCROLL_IDLE_MS = 800

const hovering = ref(false)
const dragging = ref(false)
const scrolling = ref(false)
let scrollIdleTimer: number | null = null

watch(viewportFraction, () => {
  scrolling.value = true
  if (scrollIdleTimer !== null) clearTimeout(scrollIdleTimer)
  scrollIdleTimer = window.setTimeout(() => {
    scrolling.value = false
    scrollIdleTimer = null
  }, SCROLL_IDLE_MS)
})

const engaged = computed(() => hovering.value || dragging.value || scrolling.value)

// ── Bubble ─────────────────────────────────────────────────────────────────
const pointerFraction = ref(0)
const pointerOnRail = computed(() => hovering.value || dragging.value)

const bubbleVisible = computed(() => pointerOnRail.value || scrolling.value)
const bubbleFraction = computed(() =>
  pointerOnRail.value ? pointerFraction.value : viewportFraction.value,
)
const bubbleLabel = computed(() => labelAt(bubbleFraction.value))

// ── Year marks ─────────────────────────────────────────────────────────────
/** Minimum vertical space between two year labels before one is dropped. */
const YEAR_MARK_GAP_PX = 28

const marks = computed(() =>
  yearMarks(entries.value, totalPhotos.value, railHeight.value, YEAR_MARK_GAP_PX),
)

// The rail spans the viewport, so its height changes with the window and with
// nothing else the component can see; observe rather than recompute on guessed
// events.
const resizeObserver = new ResizeObserver(() => {
  railHeight.value = railEl.value?.clientHeight ?? 0
})
watch(railEl, (el, prev) => {
  if (prev) resizeObserver.unobserve(prev)
  if (el) {
    railHeight.value = el.clientHeight
    resizeObserver.observe(el)
  }
})

// ── Pointer ────────────────────────────────────────────────────────────────
function fractionOf(event: PointerEvent): number {
  const rect = railEl.value?.getBoundingClientRect()
  if (!rect || rect.height === 0) return 0
  return clampFraction((event.clientY - rect.top) / rect.height)
}

function onPointerDown(event: PointerEvent) {
  dragging.value = true
  pointerFraction.value = fractionOf(event)
  // Capture keeps move/up events coming when the drag strays off the rail,
  // which every real drag does. Optional-chained: test DOMs lack it, and a
  // drag that only works over the rail is still a working drag.
  railEl.value?.setPointerCapture?.(event.pointerId)
  photosStore.requestScrub(pointerFraction.value)
}

function onPointerMove(event: PointerEvent) {
  pointerFraction.value = fractionOf(event)
  if (dragging.value) {
    photosStore.requestScrub(pointerFraction.value)
  }
}

function onPointerUp() {
  dragging.value = false
}

function onPointerLeave() {
  hovering.value = false
}

// ── Keyboard ───────────────────────────────────────────────────────────────
/**
 * Index (into entries) of the month the viewport is in, for stepping from.
 * Falls back to the nearest dated month when the viewport sits on an undated
 * stretch — a step has to start somewhere.
 */
function currentEntryIndex(): number {
  const key = photosStore.activeTimelineKey
  const byKey = entries.value.findIndex((e) => e.key === key)
  if (byKey !== -1) return byKey

  const index = fractionToIndex(viewportFraction.value, totalPhotos.value)
  const at = entryForIndex(entries.value, index)
  if (at) return entries.value.findIndex((e) => e.key === at.key)
  // Before the first dated photo or after the last: step in from the edge.
  return index < (entries.value[0]?.startIndex ?? 0) ? 0 : entries.value.length - 1
}

/** Entries are newest-first, so "down the page" is "forward in the array". */
function stepMonths(delta: number) {
  const target = entries.value[currentEntryIndex() + delta]
  if (target) photosStore.requestTimelineScroll(target)
}

function stepYears(delta: number) {
  const from = entries.value[currentEntryIndex()]
  if (!from) return
  // Both directions land on the top of the target year — its newest month —
  // so a year-step always means "go to year N", not "go roughly a year that
  // way and land wherever is nearest".
  const targetYear =
    delta > 0
      ? entries.value.find((e) => e.year < from.year)?.year
      : [...entries.value].reverse().find((e) => e.year > from.year)?.year
  if (targetYear === undefined) return
  const target = entries.value.find((e) => e.year === targetYear)
  if (target) photosStore.requestTimelineScroll(target)
}

function onKeyDown(event: KeyboardEvent) {
  const handlers: Record<string, () => void> = {
    ArrowDown: () => stepMonths(1),
    ArrowUp: () => stepMonths(-1),
    PageDown: () => stepYears(1),
    PageUp: () => stepYears(-1),
    Home: () => photosStore.requestScrub(0),
    End: () => photosStore.requestScrub(1),
  }
  const handler = handlers[event.key]
  if (handler) {
    event.preventDefault()
    handler()
  }
}

onBeforeUnmount(() => {
  resizeObserver.disconnect()
  if (scrollIdleTimer !== null) clearTimeout(scrollIdleTimer)
})
</script>

<style scoped>
.timeline-scrubber {
  position: fixed;
  right: 0;
  top: var(--v-layout-top, 0px);
  bottom: 0;
  width: var(--wisp-timeline-width);
  z-index: 100;
  cursor: pointer;
  /* The rail owns every gesture that starts on it; without this, a touch
     drag scrolls the page instead of scrubbing. */
  touch-action: none;
  user-select: none;
}

.scrubber-track {
  position: absolute;
  top: 0;
  bottom: 0;
  right: 11px;
  width: 2px;
  background: rgba(var(--v-theme-on-surface), 0.12);
}

.scrubber-thumb {
  position: absolute;
  right: 6px;
  width: 12px;
  height: 3px;
  border-radius: 2px;
  background: rgb(var(--v-theme-primary));
  transform: translateY(-50%);
}

.scrubber-year {
  position: absolute;
  right: 16px;
  transform: translateY(-50%);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.4px;
  color: rgba(var(--v-theme-on-surface), 0.55);
  background: rgb(var(--v-theme-background));
  padding: 1px 4px;
  border-radius: 3px;
  opacity: 0;
  transition: opacity 0.2s ease;
  pointer-events: none;
}

.timeline-scrubber--engaged .scrubber-year {
  opacity: 1;
}

.scrubber-bubble {
  position: absolute;
  right: calc(100% + 4px);
  transform: translateY(-50%);
  white-space: nowrap;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.4px;
  padding: 4px 10px;
  border-radius: 4px;
  color: rgb(var(--v-theme-on-surface));
  background: rgb(var(--v-theme-surface));
  border: 1px solid rgba(var(--v-theme-primary), 0.25);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.4);
  pointer-events: none;
}

.timeline-scrubber:focus-visible {
  outline: none;
}

.timeline-scrubber:focus-visible .scrubber-track {
  background: rgba(var(--v-theme-primary), 0.5);
}
</style>
