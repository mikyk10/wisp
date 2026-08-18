import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, it, expect, beforeEach } from 'vitest'
import PhotoItem from '../PhotoItem.vue'
import vuetify from '@/plugins/vuetify'
import { useSelectionStore } from '@/stores/selection'
import { usePhotosStore } from '@/stores/photos'

const photo = { id: 1, url: 'https://example.com/image.jpg', enabled: true, timestamp: '' }

describe('PhotoItem', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  it('renders image with correct src', () => {
    const wrapper = mount(PhotoItem, {
      props: { photo },
      global: { plugins: [pinia, vuetify] },
    })
    expect(wrapper.html()).toContain(photo.url)
  })

  it('is not selected by default', () => {
    const wrapper = mount(PhotoItem, {
      props: { photo },
      global: { plugins: [pinia, vuetify] },
    })
    expect(wrapper.classes()).not.toContain('photo-item--selected')
  })

  it('toggles selection on click', async () => {
    const wrapper = mount(PhotoItem, {
      props: { photo },
      global: { plugins: [pinia, vuetify] },
    })
    const selectionStore = useSelectionStore(pinia)
    expect(selectionStore.isPhotoSelected(photo.id)).toBe(false)
    await wrapper.trigger('click')
    expect(selectionStore.isPhotoSelected(photo.id)).toBe(true)
  })

  it('selects a range on Shift+click', async () => {
    const photosStore = usePhotosStore(pinia)
    for (const id of [1, 2, 3]) {
      photosStore.items.push({ ...photo, id })
    }
    const selectionStore = useSelectionStore(pinia)
    selectionStore.togglePhotoSelection(1) // anchor

    const wrapper = mount(PhotoItem, {
      props: { photo: { ...photo, id: 3 } },
      global: { plugins: [pinia, vuetify] },
    })
    await wrapper.trigger('click', { shiftKey: true })

    expect(selectionStore.selectedCount).toBe(3)
    expect(selectionStore.isPhotoSelected(2)).toBe(true)
  })

  it('toggles selection with Enter and Space', async () => {
    const wrapper = mount(PhotoItem, {
      props: { photo },
      global: { plugins: [pinia, vuetify] },
    })
    const selectionStore = useSelectionStore(pinia)

    await wrapper.trigger('keydown.enter')
    expect(selectionStore.isPhotoSelected(photo.id)).toBe(true)

    await wrapper.trigger('keydown.space')
    expect(selectionStore.isPhotoSelected(photo.id)).toBe(false)
  })

  it('exposes listbox option semantics', async () => {
    const wrapper = mount(PhotoItem, {
      props: { photo },
      global: { plugins: [pinia, vuetify] },
    })

    expect(wrapper.attributes('role')).toBe('option')
    expect(wrapper.attributes('tabindex')).toBe('0')
    expect(wrapper.attributes('aria-selected')).toBe('false')

    await wrapper.trigger('click')
    expect(wrapper.attributes('aria-selected')).toBe('true')
  })
})
