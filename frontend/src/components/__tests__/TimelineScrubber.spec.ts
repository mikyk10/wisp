import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TimelineScrubber from '../TimelineScrubber.vue'
import { usePhotosStore } from '@/stores/photos'
import type { Photo } from '@/types'

// The component's geometry: jsdom lays nothing out, so the rail reports the
// rect we give it. 400px tall at the top of the viewport keeps the arithmetic
// legible: clientY 100 is fraction 0.25.
const RAIL_RECT = {
  top: 0, left: 0, right: 32, bottom: 400,
  width: 32, height: 400, x: 0, y: 0,
  toJSON: () => ({}),
} as DOMRect

function stubPhoto(id: number): Photo {
  return { id, url: `u${id}`, enabled: true, timestamp: '', tags: [] }
}

/**
 * A 100-photo catalogue: 2024-12 holds the first 10 indices, 2024-01 the next
 * 40, 2023-06 the last 50. Timeline state is seeded directly — this component
 * consumes the store's derived entries, it does not care how they were built.
 */
function seedStore() {
  const store = usePhotosStore()
  store.items = Array.from({ length: 100 }, (_, i) => stubPhoto(i))
  store.timeline = {
    '2024-12': { year: 2024, month: 12, startIndex: 0, count: 10 },
    '2024-01': { year: 2024, month: 1, startIndex: 10, count: 40 },
    '2023-06': { year: 2023, month: 6, startIndex: 50, count: 50 },
  }
  return store
}

function mountScrubber() {
  const wrapper = mount(TimelineScrubber, { attachTo: document.body })
  const rail = wrapper.find('.timeline-scrubber')
  if (rail.exists()) {
    ;(rail.element as HTMLElement).getBoundingClientRect = () => RAIL_RECT
  }
  return wrapper
}

describe('TimelineScrubber', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders nothing for an empty catalogue', () => {
    usePhotosStore()
    const wrapper = mountScrubber()
    expect(wrapper.find('.timeline-scrubber').exists()).toBe(false)
    wrapper.unmount()
  })

  it('is a vertical slider whose value reads as the current month', async () => {
    const store = seedStore()
    const wrapper = mountScrubber()
    const rail = wrapper.find('.timeline-scrubber')

    expect(rail.attributes('role')).toBe('slider')
    expect(rail.attributes('aria-orientation')).toBe('vertical')
    expect(rail.attributes('tabindex')).toBe('0')

    store.reportScrollFraction(0.75) // index 75 → 2023-06
    await wrapper.vm.$nextTick()
    expect(rail.attributes('aria-valuenow')).toBe('75')
    expect(rail.attributes('aria-valuetext')).toBe('2023/06')
    wrapper.unmount()
  })

  it('names the month under the pointer while hovering, without scrolling anything', async () => {
    const store = seedStore()
    const wrapper = mountScrubber()
    const rail = wrapper.find('.timeline-scrubber')

    await rail.trigger('pointerenter')
    await rail.trigger('pointermove', { clientY: 100 }) // fraction 0.25 → index 25 → 2024-01

    expect(wrapper.find('.scrubber-bubble').text()).toBe('2024/01')
    expect(store.scrubRequest).toBeNull()
    wrapper.unmount()
  })

  it('scrubs the grid to the fraction under the finger, from press through drag', async () => {
    const store = seedStore()
    const wrapper = mountScrubber()
    const rail = wrapper.find('.timeline-scrubber')

    await rail.trigger('pointerdown', { clientY: 100 })
    expect(store.scrubRequest?.fraction).toBe(0.25)

    await rail.trigger('pointermove', { clientY: 300 })
    expect(store.scrubRequest?.fraction).toBe(0.75)

    // Released: the pointer keeps moving, the grid stays put.
    await rail.trigger('pointerup')
    const afterRelease = store.scrubRequest
    await rail.trigger('pointermove', { clientY: 40 })
    expect(store.scrubRequest).toBe(afterRelease)
    wrapper.unmount()
  })

  it('says "No date" over a stretch no month claims', async () => {
    const store = usePhotosStore()
    store.items = Array.from({ length: 100 }, (_, i) => stubPhoto(i))
    // Dated coverage starts at index 40: the first 40 photos are undated.
    store.timeline = {
      '2024-01': { year: 2024, month: 1, startIndex: 40, count: 60 },
    }
    const wrapper = mountScrubber()
    const rail = wrapper.find('.timeline-scrubber')

    await rail.trigger('pointerenter')
    await rail.trigger('pointermove', { clientY: 40 }) // fraction 0.1 → index 10, undated

    expect(wrapper.find('.scrubber-bubble').text()).toBe('No date')
    wrapper.unmount()
  })

  it('marks the years along the rail, positioned by their share of the photos', async () => {
    seedStore()
    const wrapper = mountScrubber()
    // jsdom lays nothing out, so hand the rail the height the measurement
    // watcher will read on its post-mount flush.
    Object.defineProperty(wrapper.find('.timeline-scrubber').element, 'clientHeight', {
      value: 400,
      configurable: true,
    })
    await wrapper.vm.$nextTick()

    const years = wrapper.findAll('.scrubber-year')
    expect(years.map((n) => n.text())).toEqual(['2024', '2023'])
    // 2023 holds the last 50 of 100 photos, so its label sits halfway down.
    expect((years[1].element as HTMLElement).style.top).toContain('50%')
    wrapper.unmount()
  })

  it('keeps the first year mark inside the rail instead of half under the app bar', async () => {
    seedStore()
    const wrapper = mountScrubber()
    Object.defineProperty(wrapper.find('.timeline-scrubber').element, 'clientHeight', {
      value: 400,
      configurable: true,
    })
    await wrapper.vm.$nextTick()

    // The newest year sits at fraction 0, and the label is centred on its
    // position — unclamped, its top half leaves the rail. jsdom re-serialises
    // clamp() oddly, so assert the parts rather than the exact string: a bare
    // '0%' is the regression, the 8px floor is the fix.
    const first = wrapper.findAll('.scrubber-year')[0].element as HTMLElement
    expect(first.style.top).not.toBe('0%')
    expect(first.style.top).toContain('clamp(')
    expect(first.style.top).toContain('8px')
    wrapper.unmount()
  })

  it('steps to the adjacent month on arrow keys', async () => {
    const store = seedStore()
    store.activeTimelineKey = '2024-01'
    const wrapper = mountScrubber()
    const rail = wrapper.find('.timeline-scrubber')

    await rail.trigger('keydown', { key: 'ArrowDown' }) // down the page = older
    expect(store.scrollRequest?.key).toBe('2023-06')

    store.activeTimelineKey = '2024-01'
    await rail.trigger('keydown', { key: 'ArrowUp' })
    expect(store.scrollRequest?.key).toBe('2024-12')
    wrapper.unmount()
  })

  it('steps to the adjacent year on page keys and to the ends on Home and End', async () => {
    const store = seedStore()
    store.activeTimelineKey = '2024-12'
    const wrapper = mountScrubber()
    const rail = wrapper.find('.timeline-scrubber')

    await rail.trigger('keydown', { key: 'PageDown' })
    expect(store.scrollRequest?.key).toBe('2023-06')

    store.activeTimelineKey = '2023-06'
    await rail.trigger('keydown', { key: 'PageUp' })
    expect(store.scrollRequest?.key).toBe('2024-12')

    await rail.trigger('keydown', { key: 'Home' })
    expect(store.scrubRequest?.fraction).toBe(0)
    await rail.trigger('keydown', { key: 'End' })
    expect(store.scrubRequest?.fraction).toBe(1)
    wrapper.unmount()
  })

  it('surfaces the labels while the grid scrolls and fades them once it stops', async () => {
    vi.useFakeTimers()
    const store = seedStore()
    const wrapper = mountScrubber()
    const rail = wrapper.find('.timeline-scrubber')

    expect(rail.classes()).not.toContain('timeline-scrubber--engaged')

    store.reportScrollFraction(0.6)
    await wrapper.vm.$nextTick()
    expect(rail.classes()).toContain('timeline-scrubber--engaged')
    // The bubble rides the thumb and names the viewport month — this is the
    // "know where you are while you move" behaviour, no pointer involved.
    expect(wrapper.find('.scrubber-bubble').text()).toBe('2023/06')

    vi.advanceTimersByTime(900)
    await wrapper.vm.$nextTick()
    expect(rail.classes()).not.toContain('timeline-scrubber--engaged')
    expect(wrapper.find('.scrubber-bubble').exists()).toBe(false)
    wrapper.unmount()
  })
})
