/**
 * Unit tests for PhotoGrid.vue.
 *
 * RecycleScroller (vue-virtual-scroller) is stubbed because it relies on real
 * DOM dimensions for virtual scroll calculations that jsdom cannot provide.
 * Vuetify layout components are also stubbed to avoid CSS-variable issues.
 * Full DOM rendering is covered by the Playwright E2E suite.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import PhotoGrid from '../PhotoGrid.vue'
import { usePhotosStore } from '@/stores/photos'

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

const scrollToItemSpy = vi.fn()

const stubs = {
  VOverlay: {
    template: '<div class="v-overlay-stub"><slot /></div>',
    props: ['contained'],
  },
  VContainer: { template: '<div><slot /></div>', props: ['fluid'] },
  VProgressCircular: { template: '<div />', props: ['indeterminate', 'size', 'color'] },
  VIcon: { template: '<i />', props: ['icon'] },
  RecycleScroller: {
    template: '<div class="recycle-scroller-stub" />',
    props: ['items', 'itemHeight', 'itemSize', 'gridItems', 'buffer'],
    setup() {
      return { scrollToItem: scrollToItemSpy }
    },
  },
  PhotoItem: { template: '<div class="photo-item-stub" />', props: ['photo'] },
}

function mountGrid(pinia: ReturnType<typeof createPinia>) {
  return mount(PhotoGrid, {
    global: { plugins: [pinia], stubs },
  })
}

// ---------- tests ----------

describe('PhotoGrid', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    scrollToItemSpy.mockClear()
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

  it('scrollToIndex delegates to RecycleScroller.scrollToItem', async () => {
    const wrapper = mountGrid(pinia)
    await wrapper.vm.$nextTick()

    wrapper.vm.scrollToIndex(42)

    expect(scrollToItemSpy).toHaveBeenCalledWith(42)
    wrapper.unmount()
  })
})
