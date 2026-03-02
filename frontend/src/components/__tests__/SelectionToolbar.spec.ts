import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SelectionToolbar from '../SelectionToolbar.vue'
import vuetify from '@/plugins/vuetify'
import { useSelectionStore } from '@/stores/selection'
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

function mountToolbar(pinia: ReturnType<typeof createPinia>) {
  return mount(SelectionToolbar, {
    global: { plugins: [pinia, vuetify] },
    attachTo: document.body,
  })
}

describe('SelectionToolbar', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  it('is not rendered when no photo is selected', () => {
    const wrapper = mountToolbar(pinia)
    expect(wrapper.find('.selection-toolbar').exists()).toBe(false)
    wrapper.unmount()
  })

  it('appears after a photo is selected', async () => {
    const wrapper = mountToolbar(pinia)
    const selectionStore = useSelectionStore(pinia)

    selectionStore.togglePhotoSelection(1)
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.selection-toolbar').exists()).toBe(true)
    wrapper.unmount()
  })

  it('displays the selected count', async () => {
    const wrapper = mountToolbar(pinia)
    const selectionStore = useSelectionStore(pinia)

    selectionStore.togglePhotoSelection(1)
    selectionStore.togglePhotoSelection(2)
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('2 selected')
    wrapper.unmount()
  })

  it('calls clearSelection when the cancel button is clicked', async () => {
    const wrapper = mountToolbar(pinia)
    const selectionStore = useSelectionStore(pinia)
    selectionStore.togglePhotoSelection(1)
    await wrapper.vm.$nextTick()

    const clearSpy = vi.spyOn(selectionStore, 'clearSelection')
    await wrapper.find('.selection-toolbar').findAll('button')[0].trigger('click')

    expect(clearSpy).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('calls toggleSelectedPhotosStatus when the status button is clicked', async () => {
    const wrapper = mountToolbar(pinia)
    const photosStore = usePhotosStore(pinia)
    photosStore.items.push({ id: 1, url: '', enabled: true, timestamp: '' })

    const selectionStore = useSelectionStore(pinia)
    selectionStore.togglePhotoSelection(1)
    await wrapper.vm.$nextTick()

    const toggleSpy = vi
      .spyOn(selectionStore, 'toggleSelectedPhotosStatus')
      .mockResolvedValue(undefined)
    await wrapper.find('.selection-toolbar').findAll('button')[1].trigger('click')

    expect(toggleSpy).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
})
