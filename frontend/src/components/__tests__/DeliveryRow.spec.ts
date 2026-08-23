import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DeliveryRow from '../DeliveryRow.vue'
import vuetify from '@/plugins/vuetify'
import type { Delivery } from '@/types'

vi.mock('@/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config')>()
  return {
    ...actual,
    API_BASE_URL: '',
    isApiMode: () => false,
    buildImageUrl: (catalogKey: string, id: number) => `/mock-data/${catalogKey}/${id}.jpg`,
    getDataSourceUrl: (p: string) => `/mock-data/${p}`,
  }
})

const baseDelivery: Delivery = {
  deliveredAt: '2026-08-19T12:30:00Z',
  kind: 'photo',
  reason: null,
  imageId: 4821,
  catalogKey: 'photos',
  source: '/mnt/photos/IMG_0421.jpg',
  requestedSleepSeconds: 300,
  imageAvailable: true,
}

function mountRow(delivery: Partial<Delivery> = {}) {
  return mount(DeliveryRow, {
    props: { delivery: { ...baseDelivery, ...delivery } },
    global: { plugins: [vuetify] },
    attachTo: document.body,
  })
}

describe('DeliveryRow', () => {
  it('renders a thumbnail for an available photo', () => {
    const wrapper = mountRow()

    const img = wrapper.find('img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBe('/mock-data/photos/4821.jpg')
    wrapper.unmount()
  })

  it('creates no <img> at all when the image is no longer available', () => {
    // A deleted photo answers 404 with a decodable error card, which the
    // browser would render like a successful delivery — so the element must
    // never exist, not merely fail to load.
    const wrapper = mountRow({ imageAvailable: false })

    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('.delivery-thumb--placeholder').exists()).toBe(true)
    expect(wrapper.text()).toContain('Image no longer available')
    wrapper.unmount()
  })

  it('creates no <img> when a photo record carries no image id', () => {
    const wrapper = mountRow({ imageId: null })
    expect(wrapper.find('img').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders a placeholder rather than an image for an http delivery', () => {
    const wrapper = mountRow({ kind: 'http', imageId: null, source: 'https://example.com/a.png' })

    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).toContain('HTTP image')
    expect(wrapper.text()).not.toContain('Image no longer available')
    wrapper.unmount()
  })

  it('renders a placeholder for a colour bar delivery', () => {
    // catalogKey is null because the server's colour bar loader is built
    // without one — the pattern is generated and consults no catalogue.
    const wrapper = mountRow({ kind: 'colorbar', imageId: null, catalogKey: null, source: null })

    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('.delivery-kind').text()).toBe('Colour bar')
    wrapper.unmount()
  })

  it('renders an error delivery as a failure, with no image', () => {
    const wrapper = mountRow({ kind: 'error', imageId: null, source: null })

    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('.delivery-thumb--error').exists()).toBe(true)
    expect(wrapper.text()).toContain('Error image')
    wrapper.unmount()
  })

  it('classifies an error as a failure whether or not it names a catalogue', () => {
    // catalogKey: null means "no catalogue was involved", never "this failed".
    // The commoner error path — a provider that gave up on a catalogue it had
    // already chosen — keeps its key, so an error with one is normal.
    const withCatalog = mountRow({
      kind: 'error',
      reason: 'db_error',
      imageId: null,
      catalogKey: 'photos',
      source: null,
    })
    const withoutCatalog = mountRow({
      kind: 'error',
      reason: 'db_error',
      imageId: null,
      catalogKey: null,
      source: null,
    })

    // Both are failures, and neither invents an image to show.
    for (const wrapper of [withCatalog, withoutCatalog]) {
      expect(wrapper.find('img').exists()).toBe(false)
      expect(wrapper.find('.delivery-thumb--error').exists()).toBe(true)
      expect(wrapper.find('.delivery-reason').text()).toBe('The catalog could not be read.')
    }

    // They differ only in whether there is a catalogue to name.
    expect(withCatalog.find('.delivery-kind').text()).toBe('Error image · photos')
    expect(withoutCatalog.find('.delivery-kind').text()).toBe('Error image')

    withCatalog.unmount()
    withoutCatalog.unmount()
  })

  it('names the catalogue a photo was read from', () => {
    // The file name below is not a substitute: "plate-1.png" says nothing
    // about which catalogue on a display that draws from several.
    const wrapper = mountRow({ catalogKey: 'artwork', source: '/mnt/artwork/plate-1.png' })

    expect(wrapper.find('.delivery-kind').text()).toBe('Photo · artwork')
    expect(wrapper.find('.delivery-source').text()).toBe('plate-1.png')
    wrapper.unmount()
  })

  it('names the catalogue an http delivery was fetched under', () => {
    // The source is a URL, but the key is what names the configured provider,
    // which is the thing an operator actually edits.
    const wrapper = mountRow({
      kind: 'http',
      imageId: null,
      catalogKey: 'weather',
      source: 'https://example.com/a.png',
    })

    expect(wrapper.find('.delivery-kind').text()).toBe('HTTP image · weather')
    wrapper.unmount()
  })

  it('names the catalogue on a kind this build does not know, rather than hiding it', () => {
    // The rule is "show the key the record carries", not a list of kinds, so a
    // kind added after this build still reports where it read from.
    const wrapper = mountRow({
      kind: 'hologram' as Delivery['kind'],
      imageId: null,
      catalogKey: 'photos',
    })

    expect(wrapper.find('.delivery-kind').text()).toBe('Unrecognised kind · photos')
    wrapper.unmount()
  })

  it('names nothing at all on a record that consulted no catalogue', () => {
    // A colour bar and a handler-level failure (unknown display, no provider)
    // never chose a catalogue. Neither may show a separator or a placeholder
    // where a name would go — the line is exactly the label and nothing else.
    const colorbar = mountRow({ kind: 'colorbar', imageId: null, catalogKey: null, source: null })
    const handlerError = mountRow({
      kind: 'error',
      reason: 'unknown_display',
      imageId: null,
      catalogKey: null,
      source: null,
    })

    expect(colorbar.find('.delivery-kind').text()).toBe('Colour bar')
    expect(handlerError.find('.delivery-kind').text()).toBe('Error image')

    colorbar.unmount()
    handlerError.unmount()
  })

  it('does not read a missing catalogue as a failure', () => {
    // A colour bar consults no catalogue and is not an error.
    const wrapper = mountRow({ kind: 'colorbar', reason: null, imageId: null, catalogKey: null })

    expect(wrapper.text()).toContain('Colour bar')
    expect(wrapper.text()).not.toContain('Error image')
    expect(wrapper.find('.delivery-thumb--error').exists()).toBe(false)
    expect(wrapper.find('.delivery-reason').exists()).toBe(false)
    wrapper.unmount()
  })

  it('says why an error happened, in words rather than in the raw code', () => {
    const wrapper = mountRow({ kind: 'error', reason: 'file_missing', imageId: null, source: null })

    expect(wrapper.find('.delivery-reason').text()).toBe('The photo file was not found on disk.')
    expect(wrapper.text()).not.toContain('file_missing')
    wrapper.unmount()
  })

  it('keeps load_failed and encode_failed apart', () => {
    const load = mountRow({ kind: 'error', reason: 'load_failed', imageId: null })
    const encode = mountRow({ kind: 'error', reason: 'encode_failed', imageId: null })

    expect(load.find('.delivery-reason').text()).toBe('The photo could not be read.')
    expect(encode.find('.delivery-reason').text()).toBe(
      'The photo could not be converted for this panel.'
    )
    load.unmount()
    encode.unmount()
  })

  it('falls back for a reason code this build does not know', () => {
    const wrapper = mountRow({
      kind: 'error',
      reason: 'sunspots' as Delivery['reason'],
      imageId: null,
    })

    // Not dressed up as a description of a failure we cannot describe, but the
    // code is kept so it can be looked up in the server logs.
    const text = wrapper.find('.delivery-reason').text()
    expect(text).toContain('not recognised')
    expect(text).toContain('sunspots')
    wrapper.unmount()
  })

  it('shows no reason line for a delivery that carries none', () => {
    const wrapper = mountRow()
    expect(wrapper.find('.delivery-reason').exists()).toBe(false)
    wrapper.unmount()
  })

  it('falls back for a kind this build does not know', () => {
    const wrapper = mountRow({ kind: 'hologram' as Delivery['kind'], imageId: null })

    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).toContain('Unrecognised kind')
    wrapper.unmount()
  })

  it('phrases the sleep header as a request, not as elapsed sleep', () => {
    const wrapper = mountRow()

    expect(wrapper.text()).toContain('asked to sleep 5m')
    expect(wrapper.text()).not.toContain('slept')
    expect(wrapper.text()).not.toContain('next wake')
    wrapper.unmount()
  })

  it('omits the sleep line when the server recorded no sleep request', () => {
    // The wire type is a plain int, so "no request" arrives as 0, not null.
    const zero = mountRow({ requestedSleepSeconds: 0 })
    expect(zero.find('.delivery-sleep').exists()).toBe(false)
    zero.unmount()

    const wrapper = mountRow({ requestedSleepSeconds: null })

    expect(wrapper.find('.delivery-sleep').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows the file name and keeps the full path in the title attribute', () => {
    const wrapper = mountRow()

    const source = wrapper.find('.delivery-source')
    expect(source.text()).toBe('IMG_0421.jpg')
    expect(source.attributes('title')).toBe('/mnt/photos/IMG_0421.jpg')
    wrapper.unmount()
  })

  it('shows both the relative age and the absolute timestamp', () => {
    const wrapper = mountRow()

    expect(wrapper.find('.delivery-age').text()).not.toBe('')
    expect(wrapper.find('.delivery-stamp').text()).toContain('2026')
    wrapper.unmount()
  })

  describe('hover preview', () => {
    // The overlay is teleported to the body, so it is outside the wrapper and
    // has to be looked for in the document.
    const peek = () => document.querySelector<HTMLImageElement>('.delivery-peek')

    function hover(wrapper: ReturnType<typeof mountRow>, clientX = 100, clientY = 100) {
      return wrapper.find('.delivery-thumb').trigger('mouseenter', { clientX, clientY })
    }

    it('shows the same picture the row already loaded, not a second request', async () => {
      // Reusing the row's URL is what keeps this free: the browser has the
      // response cached, so hovering costs no fetch and no database read.
      const wrapper = mountRow()

      expect(peek()).toBeNull()
      await hover(wrapper)

      expect(peek()?.getAttribute('src')).toBe('/mock-data/photos/4821.jpg')
      wrapper.unmount()
    })

    it('takes the preview away again on mouseleave', async () => {
      const wrapper = mountRow()
      await hover(wrapper)
      expect(peek()).not.toBeNull()

      await wrapper.find('.delivery-thumb').trigger('mouseleave')

      expect(peek()).toBeNull()
      wrapper.unmount()
    })

    it('places the preview beside the pointer', async () => {
      const wrapper = mountRow()

      await hover(wrapper, 100, 200)

      // 16px clear of the pointer, on the side with room for it.
      expect(peek()?.style.left).toBe('116px')
      expect(peek()?.style.top).toBe('216px')
      wrapper.unmount()
    })

    it('flips to the other side of the pointer rather than leaving the screen', async () => {
      // The drawer is pinned to the right-hand edge, so this is where the
      // pointer usually is: placed only ever down-and-right, the preview would
      // spend most of its life off screen.
      const wrapper = mountRow()

      await hover(wrapper, window.innerWidth - 10, window.innerHeight - 10)

      const left = Number.parseInt(peek()?.style.left ?? '', 10)
      const top = Number.parseInt(peek()?.style.top ?? '', 10)
      expect(left + 256).toBeLessThanOrEqual(window.innerWidth)
      expect(top + 320).toBeLessThanOrEqual(window.innerHeight)
      wrapper.unmount()
    })

    it('closes on scroll, which moves the row out from under a still pointer', async () => {
      const wrapper = mountRow()
      await hover(wrapper)
      expect(peek()).not.toBeNull()

      window.dispatchEvent(new Event('scroll'))
      await wrapper.vm.$nextTick()

      expect(peek()).toBeNull()
      wrapper.unmount()
    })

    it('takes its scroll listener with it when unmounted while hovered', async () => {
      // A panel collapsing or the history refreshing unmounts the row without
      // any mouseleave ever arriving. Vue removes the teleported element on its
      // own, so the element going away proves nothing — the listener is what
      // would be left behind, once per row, for the life of the page.
      const add = vi.spyOn(window, 'addEventListener')
      const remove = vi.spyOn(window, 'removeEventListener')

      const wrapper = mountRow()
      await hover(wrapper)

      const registered = add.mock.calls.find(([type]) => type === 'scroll')
      expect(registered).toBeDefined()

      wrapper.unmount()

      // Same handler, same capture flag: anything else leaves the original
      // registration in place.
      expect(remove).toHaveBeenCalledWith('scroll', registered![1], registered![2])
      add.mockRestore()
      remove.mockRestore()
    })

    it('has no preview to show where there is no thumbnail', async () => {
      const wrapper = mountRow({ imageAvailable: false })

      await wrapper.find('.delivery-thumb').trigger('mouseenter', { clientX: 10, clientY: 10 })

      expect(peek()).toBeNull()
      wrapper.unmount()
    })
  })
})
