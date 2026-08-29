/**
 * Unit tests for PhotoGrid.vue.
 *
 * The virtualizer (@tanstack/vue-virtual) is mocked because it relies on real
 * DOM dimensions for virtual scroll calculations that jsdom cannot provide.
 * Vuetify layout components are also stubbed to avoid CSS-variable issues.
 * Full DOM rendering is covered by the Playwright E2E suite.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import PhotoGrid from '../PhotoGrid.vue'
import { usePhotosStore } from '@/stores/photos'
import { useCatalogsStore } from '@/stores/catalogs'

vi.mock('@/api/photos', () => ({
  photosApi: { toggleVisibility: vi.fn().mockResolvedValue(undefined) },
}))

vi.mock('@/api/catalogs', () => ({
  catalogsApi: { fetchAll: vi.fn().mockResolvedValue([]) },
}))

vi.mock('@/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config')>()
  return {
    ...actual,
    API_BASE_URL: '',
    isApiMode: () => false,
    buildImageUrl: (_: string, id: number) => `/mock-data/images/photo-${id % 12}.svg`,
    getDataSourceUrl: (p: string) => `/mock-data/${p}`,
  }
})

// ---------- virtualizer mock ----------

const scrollToIndexSpy = vi.fn()

vi.mock('@tanstack/vue-virtual', () => ({
  useVirtualizer: () => ({
    value: {
      getVirtualItems: () => [],
      getTotalSize: () => 0,
      scrollToIndex: scrollToIndexSpy,
      measure: vi.fn(),
    },
  }),
}))

// ---------- stubs ----------

const stubs = {
  VOverlay: {
    template: '<div class="v-overlay-stub"><slot /></div>',
    props: ['contained'],
  },
  VContainer: { template: '<div><slot /></div>', props: ['fluid'] },
  VProgressCircular: { template: '<div />', props: ['indeterminate', 'size', 'color'] },
  VIcon: { template: '<i />', props: ['icon', 'size'] },
  VAlert: {
    template: '<div class="v-alert-stub">{{ title }} {{ text }}<slot /></div>',
    props: ['type', 'variant', 'title', 'text'],
  },
  VBtn: {
    template: '<button class="v-btn-stub" @click="$emit(\'click\')"><slot /></button>',
    props: ['color', 'variant', 'prependIcon'],
    emits: ['click'],
  },
  PhotoItem: { template: '<div class="photo-item-stub" />', props: ['photo'] },
}

function mountGrid(pinia: ReturnType<typeof createPinia>, attach = false) {
  return mount(PhotoGrid, {
    // Focus is a no-op on a detached element, so the keyboard tests mount
    // into the real document.
    ...(attach ? { attachTo: document.body } : {}),
    global: { plugins: [pinia], stubs },
  })
}

/** Let the handler's requestAnimationFrame callback run. */
const nextFrame = () => new Promise((resolve) => requestAnimationFrame(() => resolve(null)))

// ---------- tests ----------

describe('PhotoGrid', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    scrollToIndexSpy.mockClear()
  })

  it('shows the loading overlay when loading with no items', () => {
    const photosStore = usePhotosStore(pinia)
    photosStore.loading = true

    const wrapper = mountGrid(pinia)

    expect(wrapper.find('.v-overlay-stub').exists()).toBe(true)
    expect(wrapper.text()).toContain('Loading photos')
    wrapper.unmount()
  })

  it('hides the loading overlay once items are present', () => {
    const photosStore = usePhotosStore(pinia)
    photosStore.loading = true
    photosStore.items.push({ id: 1, url: '', enabled: true, timestamp: '2024-01-01T00:00:00Z' })

    const wrapper = mountGrid(pinia)

    expect(wrapper.find('.v-overlay-stub').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows "Loading more" when loading with items already present', () => {
    const photosStore = usePhotosStore(pinia)
    photosStore.loading = true
    photosStore.items.push({ id: 1, url: '', enabled: true, timestamp: '2024-01-01T00:00:00Z' })

    const wrapper = mountGrid(pinia)

    expect(wrapper.text()).toContain('Loading more')
    wrapper.unmount()
  })

  it('a store scroll request scrolls the virtualizer to the target row', async () => {
    // jsdom innerWidth is 1024: desktop layout → 3 columns
    // (1024 - 120 timeline - 8 padding = 896; floor(896 / 256) = 3).
    const photosStore = usePhotosStore(pinia)
    const wrapper = mountGrid(pinia)
    await wrapper.vm.$nextTick()

    photosStore.requestTimelineScroll({ key: '2024-06', startIndex: 42 })
    await wrapper.vm.$nextTick()

    expect(scrollToIndexSpy).toHaveBeenCalledWith(14, { align: 'start' })
    wrapper.unmount()
  })

  it('shows the error state with a Retry button when the store has an error', async () => {
    const photosStore = usePhotosStore(pinia)
    photosStore.error = 'network exploded'

    const wrapper = mountGrid(pinia)

    expect(wrapper.text()).toContain('Failed to load photos')
    expect(wrapper.text()).toContain('network exploded')
    expect(wrapper.find('.photo-grid').exists()).toBe(false)

    const catalogsStore = useCatalogsStore(pinia)
    catalogsStore.currentCatalog = 'album-a'
    const spy = vi.spyOn(catalogsStore, 'setCurrentCatalog').mockImplementation(() => {})

    await wrapper.find('.v-btn-stub').trigger('click')

    expect(spy).toHaveBeenCalledWith('album-a')
    wrapper.unmount()
  })

  it('shows the empty state after a completed stream with zero photos', () => {
    const photosStore = usePhotosStore(pinia)
    photosStore.streamCompleted = true

    const wrapper = mountGrid(pinia)

    expect(wrapper.text()).toContain('No photos in this catalog')
    expect(wrapper.find('.photo-grid').exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not show the empty state before the first load has completed', () => {
    const wrapper = mountGrid(pinia)

    expect(wrapper.text()).not.toContain('No photos in this catalog')
    expect(wrapper.find('.photo-grid').exists()).toBe(true)
    wrapper.unmount()
  })

  // ---------- keyboard ----------
  //
  // The grid is the app's only scroller and the document never overflows, so
  // nothing responds to Arrow / PageDown / Space unless this element holds
  // focus. See the tabindex comment in PhotoGrid.vue.
  describe('keyboard focus', () => {
    it('makes the scroll container focusable', () => {
      const wrapper = mountGrid(pinia)

      expect(wrapper.find('.photo-grid').attributes('tabindex')).toBe('0')
      wrapper.unmount()
    })

    it('takes focus on mount, so the arrow keys scroll without a click first', () => {
      const wrapper = mountGrid(pinia, true)

      expect(document.activeElement).toBe(wrapper.find('.photo-grid').element)
      wrapper.unmount()
    })

    it('reclaims focus when the virtualizer unmounts the focused card', async () => {
      const wrapper = mountGrid(pinia, true)
      const grid = wrapper.find('.photo-grid').element as HTMLElement

      // What a recycled row looks like: the focused element leaves the DOM and
      // focus lands on <body>, which scrolls nothing.
      grid.blur()
      expect(document.activeElement).toBe(document.body)

      grid.dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: null }))
      await nextFrame()

      expect(document.activeElement).toBe(grid)
      wrapper.unmount()
    })

    // Firefox and WebKit fire no focusout when the focused element is deleted,
    // so the scroll that deleted it has to be enough on its own — otherwise
    // the fix exists in Chromium only.
    it('reclaims focus from the scroll alone, with no focusout to help', async () => {
      const wrapper = mountGrid(pinia, true)
      const grid = wrapper.find('.photo-grid').element as HTMLElement

      grid.blur()
      expect(document.activeElement).toBe(document.body)

      grid.dispatchEvent(new Event('scroll'))
      await nextFrame()

      expect(document.activeElement).toBe(grid)
      wrapper.unmount()
    })

    it('leaves focus alone when it moved somewhere else', async () => {
      const wrapper = mountGrid(pinia, true)
      const grid = wrapper.find('.photo-grid').element as HTMLElement
      const elsewhere = document.createElement('button')
      document.body.appendChild(elsewhere)

      elsewhere.focus()
      grid.dispatchEvent(
        new FocusEvent('focusout', { bubbles: true, relatedTarget: elsewhere }),
      )
      await nextFrame()

      expect(document.activeElement).toBe(elsewhere)

      elsewhere.remove()
      wrapper.unmount()
    })

    it('does not steal focus from an open overlay', async () => {
      const wrapper = mountGrid(pinia, true)
      const grid = wrapper.find('.photo-grid').element as HTMLElement

      const overlay = document.createElement('div')
      overlay.className = 'v-overlay v-overlay--active'
      document.body.appendChild(overlay)

      grid.blur()
      grid.dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: null }))
      grid.dispatchEvent(new Event('scroll'))
      await nextFrame()

      expect(document.activeElement).toBe(document.body)

      overlay.remove()
      wrapper.unmount()
    })

    // The rail is focusable and runs its own arrow keys; a scroll it caused
    // must not pull focus off it.
    it('leaves focus where it is when something else holds it', async () => {
      const wrapper = mountGrid(pinia, true)
      const grid = wrapper.find('.photo-grid').element as HTMLElement
      const elsewhere = document.createElement('button')
      document.body.appendChild(elsewhere)

      elsewhere.focus()
      grid.dispatchEvent(new Event('scroll'))
      await nextFrame()

      expect(document.activeElement).toBe(elsewhere)

      elsewhere.remove()
      wrapper.unmount()
    })
  })
})
