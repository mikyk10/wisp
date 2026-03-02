import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, it, expect, beforeEach } from 'vitest'
import PhotoItem from '../PhotoItem.vue'
import vuetify from '@/plugins/vuetify'
import { useSelectionStore } from '@/stores/selection'

const photo = { id: 1, url: 'https://example.com/image.jpg', enabled: true, timestamp: '' }

describe('PhotoItem', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
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
})
