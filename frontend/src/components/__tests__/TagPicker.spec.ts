import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import TagPicker from '../TagPicker.vue'
import vuetify from '@/plugins/vuetify'
import { photosApi } from '@/api/photos'
import { MOBILE_BREAKPOINT } from '@/constants'

vi.mock('@/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config')>()
  return { ...actual, API_BASE_URL: 'http://api.test', isApiMode: () => true }
})

vi.mock('@/api/photos', () => ({
  photosApi: { getCatalogTags: vi.fn() },
}))

const getCatalogTags = vi.mocked(photosApi.getCatalogTags)

/**
 * Pin the viewport width. The picker chooses its surface from this — a sheet
 * on a phone, a menu on a desktop — and jsdom reports no width of its own.
 */
function setViewport(width: number) {
  window.matchMedia = ((query: string) => {
    const max = Number(/max-width:\s*(\d+)px/.exec(query)?.[1] ?? '0')
    return {
      matches: width <= max,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
      onchange: null,
    } as unknown as MediaQueryList
  }) as typeof window.matchMedia
}

async function openPicker(width = 390, modelValue: string[] = []) {
  setViewport(width)
  const wrapper = mount(TagPicker, {
    props: { modelValue, catalogKey: 'photos', open: true },
    global: { plugins: [vuetify] },
    attachTo: document.body,
  })
  // The list is read on open, so let the request settle before asserting.
  await new Promise((resolve) => setTimeout(resolve, 0))
  await wrapper.vm.$nextTick()
  return wrapper
}

/**
 * The picker's body is teleported into Vuetify's overlay container, which is
 * outside the wrapper — so everything is read from the document.
 */
function listChips(): HTMLElement[] {
  return Array.from(document.querySelectorAll('.tag-picker-section--list .v-chip'))
}

describe('TagPicker', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    getCatalogTags.mockReset()
    getCatalogTags.mockResolvedValue([
      { name: 'sky', count: 340 },
      { name: 'sea', count: 88 },
      { name: 'sakura', count: 12 },
    ])
  })

  it('reads the tag list when it opens, not before', async () => {
    setViewport(390)
    const wrapper = mount(TagPicker, {
      props: { modelValue: [], catalogKey: 'photos', open: false },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })

    // Most sessions never filter; a catalogue's hundreds of tags should not be
    // fetched on every page load to pay for the ones that do.
    expect(getCatalogTags).not.toHaveBeenCalled()

    await wrapper.setProps({ open: true })
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(getCatalogTags).toHaveBeenCalledWith('photos')
    wrapper.unmount()
  })

  it('shows each tag with the number of photos carrying it', async () => {
    const wrapper = await openPicker()

    const text = document.body.textContent ?? ''
    expect(text).toContain('sky')
    expect(text).toContain('340')
    wrapper.unmount()
  })

  it('narrows the list as you type, which is the only way through hundreds', async () => {
    const wrapper = await openPicker()

    const input = document.querySelector('.tag-picker-search input') as HTMLInputElement
    input.value = 'sak'
    input.dispatchEvent(new Event('input'))
    await wrapper.vm.$nextTick()

    const list = document.querySelector('.tag-picker-section--list')?.textContent ?? ''
    expect(list).toContain('sakura')
    expect(list).not.toContain('sky')
    wrapper.unmount()
  })

  it('emits the whole next selection when a tag is picked', async () => {
    const wrapper = await openPicker(390, ['sky'])

    listChips()[1].click() // sea
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([['sky', 'sea']])
    wrapper.unmount()
  })

  it('unpicks a tag that is already in the filter', async () => {
    const wrapper = await openPicker(390, ['sky', 'sea'])

    listChips()[0].click() // sky, already selected
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([['sea']])
    wrapper.unmount()
  })

  it('opens from the bottom on a narrow screen', async () => {
    // A phone has no room for a menu hanging off a 36px button, and a sheet
    // puts the search field where the thumb is rather than at the top of a
    // tall screen.
    const wrapper = await openPicker(MOBILE_BREAKPOINT)

    expect(document.querySelector('.v-bottom-sheet')).not.toBeNull()
    wrapper.unmount()
  })

  it('hangs off the button on a wide screen', async () => {
    const wrapper = await openPicker(MOBILE_BREAKPOINT + 1)

    expect(document.querySelector('.v-bottom-sheet')).toBeNull()
    wrapper.unmount()
  })

  describe('long tag lists', () => {
    // Production carries 1,255 tags in one catalogue. Drawing them all took
    // 1,443ms from the click on a phone-class CPU, against ~500ms for a
    // hundred — almost all of it wrapping layout.
    const many = Array.from({ length: 300 }, (_, i) => ({
      name: `tag${String(i).padStart(3, '0')}`,
      count: 300 - i,
    }))

    it('draws only the most used, not the whole list', async () => {
      getCatalogTags.mockResolvedValue(many)

      const wrapper = await openPicker()

      expect(listChips()).toHaveLength(100)
      // Ordered by use, so the cap keeps the tags worth reaching for.
      expect(listChips()[0].textContent).toContain('tag000')
      wrapper.unmount()
    })

    it('says how much it is withholding', async () => {
      // Silently drawing a hundred of three hundred would leave a reader
      // searching for a tag that is there and concluding it is not.
      getCatalogTags.mockResolvedValue(many)

      const wrapper = await openPicker()

      expect(document.body.textContent).toContain('100 most used of 300')
      wrapper.unmount()
    })

    it('still reaches a tag past the cap by searching', async () => {
      // The tail is not lost, it is searched for: matching runs over every
      // tag and only the drawing is capped.
      getCatalogTags.mockResolvedValue(many)
      const wrapper = await openPicker()

      const input = document.querySelector('.tag-picker-search input') as HTMLInputElement
      input.value = 'tag287'
      input.dispatchEvent(new Event('input'))
      await wrapper.vm.$nextTick()

      const list = document.querySelector('.tag-picker-section--list')?.textContent ?? ''
      expect(list).toContain('tag287')
      wrapper.unmount()
    })

    it('drops the notice once the search fits', async () => {
      getCatalogTags.mockResolvedValue(many)
      const wrapper = await openPicker()

      const input = document.querySelector('.tag-picker-search input') as HTMLInputElement
      input.value = 'tag29'
      input.dispatchEvent(new Event('input'))
      await wrapper.vm.$nextTick()

      expect(document.body.textContent).not.toContain('most used of')
      wrapper.unmount()
    })

    it('leaves a short list alone', async () => {
      getCatalogTags.mockResolvedValue(many.slice(0, 40))

      const wrapper = await openPicker()

      expect(listChips()).toHaveLength(40)
      expect(document.body.textContent).not.toContain('most used of')
      wrapper.unmount()
    })
  })

  it('says so when the catalogue has no tags at all', async () => {
    getCatalogTags.mockResolvedValue([])

    const wrapper = await openPicker()

    expect(document.body.textContent).toContain('No tags in this catalog yet')
    wrapper.unmount()
  })

  it('surfaces a failed read instead of looking like an empty catalogue', async () => {
    getCatalogTags.mockRejectedValue(new Error('network down'))

    const wrapper = await openPicker()

    expect(document.body.textContent).toContain('network down')
    wrapper.unmount()
  })
})
