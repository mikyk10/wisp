import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DeliveryStrip from '../DeliveryStrip.vue'
import vuetify from '@/plugins/vuetify'
import type { Delivery, DeliveryKind } from '@/types'

/** Newest-first deliveries, the order the API documents. */
function makeDeliveries(kinds: DeliveryKind[]): Delivery[] {
  return kinds.map((kind, i) => ({
    deliveredAt: new Date(Date.UTC(2026, 7, 19, 12, 30 - i * 5)).toISOString(),
    kind,
    reason: kind === 'error' ? ('file_missing' as const) : null,
    imageId: kind === 'photo' ? 100 + i : null,
    catalogKey: 'photos',
    source: kind === 'photo' ? `/mnt/photos/IMG_${i}.jpg` : null,
    requestedSleepSeconds: 300,
    imageAvailable: kind === 'photo',
  }))
}

function mountStrip(deliveries: Delivery[]) {
  return mount(DeliveryStrip, {
    props: { deliveries },
    global: { plugins: [vuetify] },
    attachTo: document.body,
  })
}

function toneOf(classes: string[]): string {
  return classes.find((c) => c.startsWith('delivery-glyph--')) ?? ''
}

describe('DeliveryStrip', () => {
  it('renders nothing when there are no deliveries', () => {
    const wrapper = mountStrip([])
    expect(wrapper.find('.delivery-strip').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders one glyph per recorded delivery', () => {
    const wrapper = mountStrip(makeDeliveries(['photo', 'photo', 'error', 'colorbar', 'http']))
    expect(wrapper.findAll('.delivery-glyph')).toHaveLength(5)
    wrapper.unmount()
  })

  it('orders glyphs oldest to newest, reversing the newest-first wire order', () => {
    // Wire order: photo (newest), error, colorbar, http (oldest).
    const wrapper = mountStrip(makeDeliveries(['photo', 'error', 'colorbar', 'http']))

    const tones = wrapper.findAll('.delivery-glyph').map((g) => toneOf(g.classes()))
    expect(tones).toEqual([
      'delivery-glyph--generated', // http, oldest
      'delivery-glyph--generated', // colorbar
      'delivery-glyph--error',
      'delivery-glyph--photo', // newest
    ])
    wrapper.unmount()
  })

  it('shows a trailing run of five errors as five adjacent error glyphs', () => {
    // Newest five are errors, so on the strip they land at the right-hand end.
    const kinds: DeliveryKind[] = [
      'error',
      'error',
      'error',
      'error',
      'error',
      'photo',
      'photo',
      'photo',
    ]
    const wrapper = mountStrip(makeDeliveries(kinds))

    const tones = wrapper.findAll('.delivery-glyph').map((g) => toneOf(g.classes()))
    expect(tones).toHaveLength(8)
    expect(tones.slice(-5)).toEqual(Array(5).fill('delivery-glyph--error'))
    expect(tones.slice(0, 3)).toEqual(Array(3).fill('delivery-glyph--photo'))
    wrapper.unmount()
  })

  it('falls back to the unknown tone for a kind this build does not know', () => {
    const deliveries = makeDeliveries(['photo'])
    deliveries[0].kind = 'hologram' as DeliveryKind

    const wrapper = mountStrip(deliveries)

    expect(toneOf(wrapper.find('.delivery-glyph').classes())).toBe('delivery-glyph--unknown')
    wrapper.unmount()
  })

  it('takes the glyph tone from the kind, never from the catalogue key', () => {
    const deliveries = makeDeliveries(['error', 'colorbar'])
    deliveries[0].catalogKey = 'photos' // a provider-level failure keeps its key
    deliveries[1].catalogKey = null // a colour bar consults no catalogue

    const wrapper = mountStrip(deliveries)
    const tones = wrapper.findAll('.delivery-glyph').map((g) => toneOf(g.classes()))

    expect(tones).toEqual(['delivery-glyph--generated', 'delivery-glyph--error'])
    wrapper.unmount()
  })

  it('names the failure reason in an error glyph tooltip', () => {
    // Readable without expanding the panel — the point of the strip.
    const wrapper = mountStrip(makeDeliveries(['error']))

    const title = wrapper.find('.delivery-glyph').attributes('title') ?? ''
    expect(title).toContain('Error image')
    expect(title).toContain('The photo file was not found on disk.')
    wrapper.unmount()
  })

  it('describes the strip to assistive technology, including the failure count', () => {
    const wrapper = mountStrip(makeDeliveries(['error', 'error', 'photo']))

    expect(wrapper.find('.delivery-strip').attributes('aria-label')).toBe(
      '3 recorded deliveries, oldest first, 2 failed'
    )
    wrapper.unmount()
  })

  it('omits the failure count when nothing failed', () => {
    const wrapper = mountStrip(makeDeliveries(['photo', 'photo']))

    expect(wrapper.find('.delivery-strip').attributes('aria-label')).toBe(
      '2 recorded deliveries, oldest first'
    )
    wrapper.unmount()
  })
})
