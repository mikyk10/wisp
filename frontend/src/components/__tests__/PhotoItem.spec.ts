import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import PhotoItem from '../PhotoItem.vue'
import vuetify from '@/plugins/vuetify'
import { useSelectionStore } from '@/stores/selection'
import { usePhotosStore } from '@/stores/photos'

const photo = { id: 1, url: 'https://example.com/image.jpg', enabled: true, timestamp: '', tags: [] }

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

describe('PhotoItem tags', () => {
  const tagged = { ...photo, tags: ['sakura', 'night'] }
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  function mountItem(p: typeof photo) {
    return mount(PhotoItem, { props: { photo: p }, global: { plugins: [pinia, vuetify] } })
  }

  // The badge is the whole point of the redesign: it is the only way to a
  // photo's tags on a touch screen, where the previous hover-only overlay
  // simply never appeared.
  it('shows a badge with the tag count on a tagged photo', () => {
    const wrapper = mountItem(tagged)

    const badge = wrapper.find('.tag-badge')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toContain('2')
    wrapper.unmount()
  })

  it('shows no badge on a photo with no tags', () => {
    const wrapper = mountItem(photo)

    expect(wrapper.find('.tag-badge').exists()).toBe(false)
    wrapper.unmount()
  })

  it('opens the tag sheet without also selecting the photo', async () => {
    // The card selects; the badge must not do both.
    const photosStore = usePhotosStore()
    const selectionStore = useSelectionStore()
    const show = vi.spyOn(photosStore, 'showPhotoTags')
    const toggle = vi.spyOn(selectionStore, 'togglePhotoSelection')

    const wrapper = mountItem(tagged)
    await wrapper.find('.tag-badge').trigger('click')

    expect(show).toHaveBeenCalledWith(tagged)
    expect(toggle).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('renders the hover overlay from the tags it already has', () => {
    // No request: the tags arrived with the listing. The old version fetched
    // per card on hover, which is one request per visible photo.
    const wrapper = mountItem(tagged)

    const overlay = wrapper.find('.tag-overlay')
    expect(overlay.exists()).toBe(true)
    expect(overlay.text()).toContain('sakura')
    expect(overlay.text()).toContain('night')
    wrapper.unmount()
  })
})
