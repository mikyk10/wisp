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
  VAutocomplete: {
    template: '<input />',
    props: ['modelValue', 'items', 'multiple', 'density', 'hideDetails', 'variant', 'placeholder', 'color', 'disabled', 'menuProps'],
    emits: ['update:modelValue'],
  },
  VIcon: { template: '<i />', props: ['icon', 'start'] },
  VSpacer: { template: '<div />' },
  VOverlay: { template: '<div />' },
}

// Own heavy components stubbed to avoid their internal complexity.
const componentStubs = {
  PhotoGrid: {
    template: '<div class="photo-grid-stub" />',
    setup: () => ({ scrollToIndex: () => {} }),
  },
  TimelineScrollbar: { template: '<div />', props: ['gridRef'] },
  SelectionToolbar: { template: '<div />' },
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

  it('shows the photo count chip text when photos are present', async () => {
    const catalogsStore = useCatalogsStore(pinia)
    vi.spyOn(catalogsStore, 'initCatalogs').mockResolvedValue(undefined)

    const wrapper = mountApp(pinia)
    usePhotosStore(pinia).items.push({ id: 1, url: '', enabled: true, timestamp: '' })
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('1 photos')
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
})
