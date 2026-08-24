/**
 * Unit tests for App.vue.
 *
 * We avoid mounting Vuetify's layout components (VApp, VMain, …) for real
 * because they set CSS custom properties via setAttribute(), which jsdom
 * cannot handle. Instead we provide lightweight stubs that render their
 * default slot so that slot content (title text, chip text) is still visible.
 * Full DOM-level assertions (v-select rendering, chip classes, etc.) are
 * covered by the Playwright E2E suite.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import App from '../App.vue'
import { useCatalogsStore } from '@/stores/catalogs'
import { usePhotosStore } from '@/stores/photos'
import { useSelectionStore } from '@/stores/selection'

vi.mock('@/api/catalogs', () => ({
  catalogsApi: { fetchAll: vi.fn().mockResolvedValue([]) },
}))

vi.mock('@/api/photos', () => ({
  photosApi: { toggleVisibility: vi.fn().mockResolvedValue(undefined) },
}))

vi.mock('@/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config')>()
  return {
    ...actual,
    API_BASE_URL: '',
    isApiMode: () => false,
    buildImageUrl: (_: string, id: number) => `https://picsum.photos/240/240?random=${id}`,
    getDataSourceUrl: (p: string) => `/mock-data/${p}`,
  }
})

// ---------- stubs ----------

// Vuetify layout components are replaced with minimal slot-forwarding wrappers.
// This prevents the jsdom/Vuetify CSS-variable incompatibility while still
// rendering slot content so text assertions remain meaningful.
//
// Props MUST be declared explicitly on each stub: if a bound prop (e.g. :items)
// is not declared, Vue treats it as a fallthrough attribute and calls
// setAttribute() with the reactive value, which jsdom cannot serialize.
const vuetifyStubs = {
  VApp: { template: '<div><slot /></div>' },
  VAppBar: { template: '<div><slot /></div>' },
  VAppBarTitle: { template: '<span><slot /></span>' },
  VMain: { template: '<div><slot /></div>' },
  VChip: {
    template: '<span><slot /></span>',
    props: ['color', 'textColor'],
  },
  VSelect: {
    template: '<select />',
    props: ['modelValue', 'items', 'density', 'hideDetails', 'variant', 'color', 'itemColor'],
    emits: ['update:modelValue'],
  },
  VIcon: { template: '<i />', props: ['icon', 'start', 'size'] },
  VBadge: {
    template: '<span><slot /></span>',
    props: ['modelValue', 'content', 'color', 'offsetX', 'offsetY'],
  },
  VSpacer: { template: '<div />' },
  VOverlay: { template: '<div />' },
  VNavigationDrawer: {
    template: '<div><slot /></div>',
    props: ['modelValue', 'location', 'temporary', 'width'],
    emits: ['update:modelValue'],
  },
  VBtn: {
    template: '<button class="v-btn-stub" @click="$emit(\'click\')"><slot /></button>',
    props: ['color', 'variant', 'prependIcon', 'icon', 'size'],
    emits: ['click'],
  },
}

// Own heavy components stubbed to avoid their internal complexity.
const componentStubs = {
  PhotoGrid: { template: '<div class="photo-grid-stub" />' },
  TimelineScrubber: { template: '<div />' },
  SelectionToolbar: { template: '<div />' },
  DeviceDrawer: {
    template: '<div class="device-drawer-stub" :class="{ \'device-drawer-stub--open\': modelValue }" />',
    props: ['modelValue'],
    emits: ['update:modelValue'],
  },
  TagFilterBar: {
    name: 'TagFilterBar',
    template: '<div class="tag-filter-bar-stub" />',
    props: ['shown', 'total', 'filterTags'],
    emits: ['remove', 'clear'],
  },
  TagPicker: {
    name: 'TagPicker',
    template: '<div class="tag-picker-stub" :class="{ \'tag-picker-stub--open\': open }" />',
    props: ['open', 'modelValue', 'catalogKey', 'activator'],
    emits: ['update:open', 'update:modelValue'],
  },
  PhotoTagsSheet: {
    name: 'PhotoTagsSheet',
    template: '<div class="photo-tags-sheet-stub" />',
    props: ['modelValue', 'photo'],
    emits: ['update:modelValue', 'filter'],
  },
}

function mountApp(pinia: ReturnType<typeof createPinia>) {
  return mount(App, {
    global: {
      plugins: [pinia],
      stubs: { ...vuetifyStubs, ...componentStubs },
    },
  })
}

// ---------- tests ----------

describe('App', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.unstubAllGlobals()
    // mockImplementation creates a fresh ReadableStream on every call so
    // the stream is never reused (and therefore never "already locked").
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() =>
        Promise.resolve({
          ok: true,
          status: 200,
          body: new ReadableStream({ start: (c) => c.close() }),
        })
      )
    )
    Element.prototype.scrollIntoView = vi.fn()
  })

  it('calls catalogsStore.initCatalogs on mount', () => {
    const catalogsStore = useCatalogsStore(pinia)
    const spy = vi.spyOn(catalogsStore, 'initCatalogs').mockResolvedValue(undefined)

    mountApp(pinia)

    expect(spy).toHaveBeenCalledOnce()
  })

  it('renders the title text "WiSP"', () => {
    const catalogsStore = useCatalogsStore(pinia)
    vi.spyOn(catalogsStore, 'initCatalogs').mockResolvedValue(undefined)

    const wrapper = mountApp(pinia)

    expect(wrapper.text()).toContain('WiSP')
    wrapper.unmount()
  })

  // The count used to be a chip in the app bar. It moved into the filter bar
  // because the bar has no width to spare on a narrow screen — see TagFilterBar
  // — so what is asserted here is that App still hands it the number.
  it('hands the photo count to the filter bar', async () => {
    const catalogsStore = useCatalogsStore(pinia)
    vi.spyOn(catalogsStore, 'initCatalogs').mockResolvedValue(undefined)

    const wrapper = mountApp(pinia)
    usePhotosStore(pinia).items.push({ id: 1, url: '', enabled: true, timestamp: '', tags: [] })
    await wrapper.vm.$nextTick()

    expect(wrapper.findComponent({ name: 'TagFilterBar' }).props('shown')).toBe(1)
    wrapper.unmount()
  })

  it('replaces the main view with an error state when the catalog fetch failed', async () => {
    const catalogsStore = useCatalogsStore(pinia)
    const initSpy = vi.spyOn(catalogsStore, 'initCatalogs').mockResolvedValue(undefined)

    const wrapper = mountApp(pinia)
    catalogsStore.error = 'backend unreachable'
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Failed to load catalogs')
    expect(wrapper.text()).toContain('backend unreachable')
    expect(wrapper.find('.photo-grid-stub').exists()).toBe(false)

    // Retry button re-runs initCatalogs. The button is addressed by its own
    // class rather than by position: find() returns the first match, and the
    // app bar also renders a button (the device drawer trigger).
    initSpy.mockClear()
    await wrapper.find('.catalog-retry').trigger('click')
    expect(initSpy).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('opens the device drawer from the app bar trigger', async () => {
    const catalogsStore = useCatalogsStore(pinia)
    vi.spyOn(catalogsStore, 'initCatalogs').mockResolvedValue(undefined)

    const wrapper = mountApp(pinia)
    expect(wrapper.find('.device-drawer-stub--open').exists()).toBe(false)

    await wrapper.find('.device-drawer-trigger').trigger('click')

    expect(wrapper.find('.device-drawer-stub--open').exists()).toBe(true)
    wrapper.unmount()
  })

  it('shows the selection count chip text when photos are selected', async () => {
    const catalogsStore = useCatalogsStore(pinia)
    vi.spyOn(catalogsStore, 'initCatalogs').mockResolvedValue(undefined)

    const wrapper = mountApp(pinia)
    useSelectionStore(pinia).togglePhotoSelection(42)
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('1 selected')
    wrapper.unmount()
  })

  // ---------- Escape ----------
  //
  // One handler resolves Escape for the whole app, in priority order:
  // an open Vuetify overlay wins, then the displays drawer, then the
  // selection. See handleEscape in App.vue.
  describe('Escape', () => {
    const pressEscape = (init: KeyboardEventInit = {}) =>
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', ...init }))

    it('clears the selection when nothing is open on top of the grid', async () => {
      const wrapper = mountApp(pinia)
      const selectionStore = useSelectionStore(pinia)
      selectionStore.togglePhotoSelection(1)
      await wrapper.vm.$nextTick()

      pressEscape()
      await wrapper.vm.$nextTick()

      expect(selectionStore.isSelectionMode).toBe(false)
      wrapper.unmount()
    })

    it('leaves the selection alone while a Vuetify overlay is up', async () => {
      const wrapper = mountApp(pinia)
      const selectionStore = useSelectionStore(pinia)
      selectionStore.togglePhotoSelection(1)
      await wrapper.vm.$nextTick()

      // What an open tag picker / tag sheet / select menu looks like in the
      // DOM. Vuetify closes it from its own listener; we must not also act.
      const overlay = document.createElement('div')
      overlay.className = 'v-overlay v-overlay--active'
      document.body.appendChild(overlay)

      pressEscape()
      await wrapper.vm.$nextTick()

      expect(selectionStore.isSelectionMode).toBe(true)

      overlay.remove()
      wrapper.unmount()
    })

    it('closes the displays drawer rather than clearing the selection', async () => {
      const wrapper = mountApp(pinia)
      const selectionStore = useSelectionStore(pinia)
      selectionStore.togglePhotoSelection(1)
      await wrapper.find('.device-drawer-trigger').trigger('click')

      expect(wrapper.find('.device-drawer-stub--open').exists()).toBe(true)

      pressEscape()
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.device-drawer-stub--open').exists()).toBe(false)
      expect(selectionStore.isSelectionMode).toBe(true)
      wrapper.unmount()
    })

    it('ignores Escape sent while an IME is composing', async () => {
      const wrapper = mountApp(pinia)
      const selectionStore = useSelectionStore(pinia)
      selectionStore.togglePhotoSelection(1)
      await wrapper.vm.$nextTick()

      pressEscape({ isComposing: true })
      await wrapper.vm.$nextTick()

      expect(selectionStore.isSelectionMode).toBe(true)
      wrapper.unmount()
    })

    it('stops listening once unmounted', async () => {
      const wrapper = mountApp(pinia)
      const selectionStore = useSelectionStore(pinia)
      selectionStore.togglePhotoSelection(1)
      await wrapper.vm.$nextTick()
      wrapper.unmount()

      pressEscape()

      expect(selectionStore.isSelectionMode).toBe(true)
    })
  })
})
