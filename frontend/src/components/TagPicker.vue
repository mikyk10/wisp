<template>
  <!-- Two surfaces, one body. Below the mobile breakpoint the list opens from
       the bottom, where a phone has room for it and where the search field
       lands under the thumb rather than at the top of a tall screen. Above it,
       the same list hangs off the button that opened it.

       The previous attempt at this put the whole thing in the app bar as a
       fixed 240px control, which is what made it unusable on a narrow screen:
       the bar has no 240px to give. -->
  <component
    :is="surface"
    v-model="open"
    v-bind="surfaceProps"
  >
    <v-card class="tag-picker">
      <div class="tag-picker-head">
        <v-text-field
          ref="searchField"
          v-model="query"
          class="tag-picker-search"
          density="compact"
          variant="solo-filled"
          flat
          hide-details
          clearable
          autofocus
          placeholder="Search tags"
          prepend-inner-icon="mdi-magnify"
        />
        <v-btn
          class="tag-picker-close"
          icon="mdi-close"
          variant="text"
          size="small"
          aria-label="Close"
          @click="open = false"
        />
      </div>

      <v-divider />

      <!-- Selected tags first and always visible, so that what is being
           filtered can be read and undone without hunting for the chips in a
           list of hundreds. -->
      <div
        v-if="modelValue.length > 0"
        class="tag-picker-section"
      >
        <div class="tag-picker-label">
          Filtering by all of
        </div>
        <div class="tag-picker-chips">
          <v-chip
            v-for="tag in modelValue"
            :key="tag"
            class="tag-picker-chip"
            color="primary"
            variant="flat"
            size="small"
            closable
            @click:close="toggle(tag)"
          >
            {{ tag }}
          </v-chip>
        </div>
      </div>

      <div class="tag-picker-section tag-picker-section--list">
        <div
          v-if="loading"
          class="tag-picker-status"
        >
          <v-progress-circular
            indeterminate
            size="18"
            width="2"
            class="mr-2"
          />
          Loading tags…
        </div>
        <div
          v-else-if="error"
          class="tag-picker-status tag-picker-status--error"
        >
          {{ error }}
        </div>
        <div
          v-else-if="visible.length === 0"
          class="tag-picker-status"
        >
          {{ tags.length === 0 ? 'No tags in this catalog yet' : 'No tag matches that' }}
        </div>
        <div
          v-else
          class="tag-picker-chips"
        >
          <v-chip
            v-for="tag in visible"
            :key="tag.name"
            class="tag-picker-chip"
            :color="isSelected(tag.name) ? 'primary' : undefined"
            :variant="isSelected(tag.name) ? 'flat' : 'outlined'"
            size="small"
            @click="toggle(tag.name)"
          >
            {{ tag.name }}
            <!-- The count is what makes a long list usable: it says which tags
                 are worth reaching for, and it lets a combination that would
                 return nothing be avoided before spending a request on it. -->
            <span class="tag-picker-count">{{ tag.count }}</span>
          </v-chip>
        </div>

        <!-- Silently drawing a hundred of twelve hundred would leave a reader
             searching the list for a tag that is there, giving up, and
             concluding the catalogue does not have it. -->
        <div
          v-if="withheld > 0"
          class="tag-picker-more"
        >
          Showing the {{ visible.length }} most used of {{ matching.length }} — search to narrow
        </div>
      </div>
    </v-card>
  </component>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { VBottomSheet, VMenu } from 'vuetify/components'
import { photosApi } from '@/api/photos'
import { isApiMode } from '@/config'
import { MOBILE_BREAKPOINT } from '@/constants'
import type { TagUsage } from '@/types'

interface Props {
  modelValue: string[]
  catalogKey: string
  /**
   * Selector the desktop menu positions itself against; ignored by the bottom
   * sheet.
   *
   * A target, not an activator. Handing VMenu an activator makes it bind its
   * own click handler to that element, which then toggles the menu a second
   * time on the same click that opened it — the picker appears and vanishes
   * again before it can be read. Opening is the parent's business here,
   * because the two surfaces cannot share one activator: the bottom sheet has
   * none to bind to.
   */
  anchor?: string
}

const props = withDefaults(defineProps<Props>(), { anchor: undefined })
const emit = defineEmits<{
  'update:modelValue': [tags: string[]]
  'update:open': [open: boolean]
}>()

const open = defineModel<boolean>('open', { default: false })

const tags = ref<TagUsage[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const query = ref('')

const isNarrow = ref(window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT}px)`).matches)
const mobileQuery = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT}px)`)
const onBreakpoint = (e: MediaQueryListEvent) => {
  isNarrow.value = e.matches
}
mobileQuery.addEventListener('change', onBreakpoint)

const surface = computed(() => (isNarrow.value ? VBottomSheet : VMenu))
const surfaceProps = computed(() =>
  isNarrow.value
    ? {}
    : { target: props.anchor, closeOnContentClick: false, location: 'bottom end' as const },
)

/*
 * How many tags are drawn at once.
 *
 * Chosen from measurement rather than taste. On a CPU throttled to 6x — a
 * mid-range phone — drawing this catalogue's 1,255 tags took 1,443ms from the
 * click, against 514ms for 200 and 488ms for 50. Almost all of the difference
 * is wrapping layout, which grows faster than the count: the step from 50 to
 * 200 costs 26ms, the step from 200 to 1,255 costs 929ms. A hundred sits well
 * inside the flat part, where the cost is the overlay opening rather than the
 * tags.
 *
 * The list is ordered by use, so this is the hundred most used. The tail is
 * not lost, it is searched for: matching runs over every tag and only the
 * drawing is capped. It is not much of a loss either — a third of this
 * catalogue's tags are on a single photograph each, which is not something
 * anyone finds by scrolling.
 */
const RENDER_LIMIT = 100

/** Every tag matching the search, however many that is. */
const matching = computed(() => {
  const q = (query.value ?? '').trim().toLowerCase()
  if (q === '') return tags.value
  return tags.value.filter((t) => t.name.toLowerCase().includes(q))
})

const visible = computed(() => matching.value.slice(0, RENDER_LIMIT))

/** How many matches are being withheld, so the reader can be told. */
const withheld = computed(() => matching.value.length - visible.value.length)

function isSelected(name: string): boolean {
  return props.modelValue.includes(name)
}

/**
 * Toggling emits the whole next selection rather than a single tag, so the
 * parent applies one filter change per click instead of reconstructing it.
 */
function toggle(name: string) {
  const next = isSelected(name)
    ? props.modelValue.filter((t) => t !== name)
    : [...props.modelValue, name]
  emit('update:modelValue', next)
}

/**
 * The list is read when the picker opens, not when the page loads: a catalogue
 * can hold hundreds of tags and most sessions never filter at all.
 */
async function load() {
  if (!isApiMode() || props.catalogKey === '') {
    tags.value = []
    return
  }
  loading.value = true
  error.value = null
  try {
    tags.value = await photosApi.getCatalogTags(props.catalogKey)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load tags'
  } finally {
    loading.value = false
  }
}

// immediate, so that a picker mounted already open reads its list. Without it
// the load hangs off the transition from closed to open, and anything that
// starts open — a test, or a future entry point that opens it directly — shows
// an empty catalogue instead of its tags.
watch(
  open,
  (isOpen) => {
    emit('update:open', isOpen)
    if (isOpen) {
      query.value = ''
      void load()
    }
  },
  { immediate: true },
)

// A different catalogue has different tags; anything already read is wrong for
// it, and showing the previous catalogue's list would offer filters that match
// nothing here.
watch(
  () => props.catalogKey,
  () => {
    tags.value = []
    if (open.value) void load()
  },
)
</script>

<style scoped>
.tag-picker {
  display: flex;
  flex-direction: column;
  /* The sheet is allowed most of a phone screen but never all of it: the strip
     of grid left showing is what says the list is a layer over the photos
     rather than a page you have navigated to. */
  max-height: 70vh;
  background: rgb(var(--v-theme-surface));
}

.tag-picker-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 10px 10px 12px;
}

.tag-picker-search {
  flex: 1 1 auto;
  min-width: 0;
}

.tag-picker-section {
  padding: 10px 12px;
}

.tag-picker-section--list {
  overflow-y: auto;
  flex: 1 1 auto;
}

.tag-picker-label {
  font-size: 0.7rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(var(--v-theme-on-surface), 0.6);
  margin-bottom: 8px;
}

.tag-picker-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tag-picker-chip {
  max-width: 100%;
}

.tag-picker-count {
  margin-left: 6px;
  font-variant-numeric: tabular-nums;
  opacity: 0.6;
}

.tag-picker-more {
  padding-top: 10px;
  font-size: 0.75rem;
  color: rgba(var(--v-theme-on-surface), 0.6);
}

.tag-picker-status {
  display: flex;
  align-items: center;
  padding: 12px 0;
  font-size: 0.85rem;
  color: rgba(var(--v-theme-on-surface), 0.6);
}

.tag-picker-status--error {
  color: rgb(var(--v-theme-error));
}
</style>
