import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TimelineScrollbar from '../TimelineScrollbar.vue'
import vuetify from '@/plugins/vuetify'
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

function mountScrollbar(pinia: ReturnType<typeof createPinia>) {
  return mount(TimelineScrollbar, {
    global: { plugins: [pinia, vuetify] },
    attachTo: document.body,
  })
}

describe('TimelineScrollbar', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    // jsdom does not implement scrollIntoView; stub it to prevent unhandled rejections
    // from the component's activeEntry watcher.
    Element.prototype.scrollIntoView = vi.fn()
  })

  it('renders one .timeline-entry per store entry', async () => {
    const photosStore = usePhotosStore(pinia)
    photosStore.timeline['2024-06'] = { year: 2024, month: 6, startIndex: 0, count: 10 }
    photosStore.timeline['2023-12'] = { year: 2023, month: 12, startIndex: 10, count: 5 }

    const wrapper = mountScrollbar(pinia)
    await wrapper.vm.$nextTick()

    const entries = wrapper.findAll('.timeline-entry')
    expect(entries).toHaveLength(2)
    wrapper.unmount()
  })

  it('displays the label and count for each entry', async () => {
    const photosStore = usePhotosStore(pinia)
    photosStore.timeline['2024-03'] = { year: 2024, month: 3, startIndex: 0, count: 7 }

    const wrapper = mountScrollbar(pinia)
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('2024/03')
    expect(wrapper.text()).toContain('7 photos')
    wrapper.unmount()
  })

  it('puts a scroll request into the store on click', async () => {
    const photosStore = usePhotosStore(pinia)
    photosStore.timeline['2024-06'] = { year: 2024, month: 6, startIndex: 42, count: 3 }

    const wrapper = mountScrollbar(pinia)
    await wrapper.vm.$nextTick()

    await wrapper.find('.timeline-entry').trigger('click')

    expect(photosStore.scrollRequest).toEqual({ index: 42, key: '2024-06' })
    expect(photosStore.activeTimelineKey).toBe('2024-06')
    wrapper.unmount()
  })

  it('marks the entry active when the store activeTimelineKey changes', async () => {
    const photosStore = usePhotosStore(pinia)
    photosStore.timeline['2024-06'] = { year: 2024, month: 6, startIndex: 0, count: 3 }
    photosStore.timeline['2023-01'] = { year: 2023, month: 1, startIndex: 3, count: 2 }

    const wrapper = mountScrollbar(pinia)
    await wrapper.vm.$nextTick()

    // No active entry initially
    expect(wrapper.find('.timeline-entry--active').exists()).toBe(false)

    photosStore.activeTimelineKey = '2024-06'
    await wrapper.vm.$nextTick()

    const active = wrapper.find('.timeline-entry--active')
    expect(active.exists()).toBe(true)
    expect(active.text()).toContain('2024/06')
    wrapper.unmount()
  })
})
