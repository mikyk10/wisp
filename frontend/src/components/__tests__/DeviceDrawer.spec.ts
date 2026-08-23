/**
 * Unit tests for DeviceDrawer.vue.
 *
 * VNavigationDrawer is replaced with a slot-forwarding stub: it registers
 * itself with Vuetify's layout, which only exists inside a mounted VApp, and
 * VApp is the component this suite already avoids because it sets CSS custom
 * properties jsdom cannot handle. Everything below the drawer chrome —
 * panels, strips, rows — mounts for real.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DeviceDrawer from '../DeviceDrawer.vue'
import vuetify from '@/plugins/vuetify'
import { useDevicesStore } from '@/stores/devices'

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

const stubs = {
  VNavigationDrawer: {
    template: '<div class="drawer-stub"><slot /></div>',
    props: ['modelValue', 'location', 'temporary', 'width'],
    emits: ['update:modelValue'],
  },
}

function mountDrawer(pinia: ReturnType<typeof createPinia>, modelValue = true) {
  return mount(DeviceDrawer, {
    props: { modelValue },
    global: { plugins: [pinia, vuetify], stubs },
    attachTo: document.body,
  })
}

describe('DeviceDrawer', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  // ── vocabulary ───────────────────────────────────────────────────────────

  it('carries the standing footnote verbatim', async () => {
    const wrapper = mountDrawer(pinia)
    await flushPromises()

    expect(wrapper.text()).toContain(
      '"Delivered" means the server sent the image. It cannot confirm the frame displayed it.'
    )
    wrapper.unmount()
  })

  it('never claims a frame is showing an image or is online', async () => {
    const wrapper = mountDrawer(pinia)
    await flushPromises()

    const text = wrapper.text().toLowerCase()
    expect(text).toContain('last delivered')
    for (const forbidden of ['now showing', 'currently displaying', 'last seen', 'online']) {
      expect(text).not.toContain(forbidden)
    }
    wrapper.unmount()
  })

  // ── recording switched off ───────────────────────────────────────────────

  it('says nothing about recording while it is switched on', async () => {
    const wrapper = mountDrawer(pinia)
    await flushPromises()

    expect(wrapper.find('.device-drawer-recording').exists()).toBe(false)
    wrapper.unmount()
  })

  it('warns that the figures stopped updating when recording did', async () => {
    const store = useDevicesStore(pinia)
    vi.spyOn(store, 'loadDevices').mockImplementation(async () => {
      store.recordingEnabled = false
      store.devices = [
        {
          key: 'a1b2c3d4e5f6',
          name: 'living-room',
          model: 'ws7in3e',
          width: 800,
          height: 480,
          orientation: 'landscape',
          catalogKeys: ['photos'],
          sleepDurationSeconds: 300,
          wakeSchedule: [],
          lastDeliveredAt: '2026-08-19T12:30:00Z',
          recentDeliveryCount: 12,
          recentErrorCount: 3,
        },
      ]
    })
    vi.spyOn(store, 'loadAllDeliveries').mockResolvedValue(undefined)

    const wrapper = mountDrawer(pinia)
    await flushPromises()

    expect(wrapper.find('.device-drawer-recording').text()).toBe(
      'Delivery recording is off. The counts and times below are real, but they stopped updating when recording did.'
    )
    // The stale figures stay on screen: they are true, just not current.
    expect(wrapper.text()).toContain('living-room')
    expect(wrapper.text()).toContain('Last delivered')
    wrapper.unmount()
  })

  // ── states ───────────────────────────────────────────────────────────────

  it('names a display that has never been handed an image', async () => {
    const wrapper = mountDrawer(pinia)
    await flushPromises()

    expect(wrapper.text()).toContain('Never delivered')
    expect(wrapper.find('.device-never').exists()).toBe(true)
    wrapper.unmount()
  })

  it('shows the loading state while the display list is in flight', async () => {
    const store = useDevicesStore(pinia)
    vi.spyOn(store, 'loadDevices').mockImplementation(async () => {
      store.loading = true
    })
    vi.spyOn(store, 'loadAllDeliveries').mockResolvedValue(undefined)

    const wrapper = mountDrawer(pinia)
    await flushPromises()

    expect(wrapper.text()).toContain('Loading displays…')
    wrapper.unmount()
  })

  it('shows the error state with a retry', async () => {
    const store = useDevicesStore(pinia)
    vi.spyOn(store, 'loadDevices').mockImplementation(async () => {
      store.error = 'backend unreachable'
    })
    vi.spyOn(store, 'loadAllDeliveries').mockResolvedValue(undefined)

    const wrapper = mountDrawer(pinia)
    await flushPromises()

    expect(wrapper.text()).toContain('Failed to load displays')
    expect(wrapper.text()).toContain('backend unreachable')

    const refreshSpy = vi.spyOn(store, 'refresh').mockResolvedValue(undefined)
    await wrapper.find('.device-drawer-retry').trigger('click')
    expect(refreshSpy).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('shows the empty state when no display is configured', async () => {
    const store = useDevicesStore(pinia)
    vi.spyOn(store, 'loadDevices').mockResolvedValue(undefined)
    vi.spyOn(store, 'loadAllDeliveries').mockResolvedValue(undefined)

    const wrapper = mountDrawer(pinia)
    await flushPromises()

    expect(wrapper.text()).toContain('No displays are configured.')
    wrapper.unmount()
  })

  // ── strips ───────────────────────────────────────────────────────────────

  it('draws every display strip without any panel being expanded', async () => {
    const wrapper = mountDrawer(pinia)
    await flushPromises()

    // Nothing is expanded: panel bodies are not rendered at all.
    expect(wrapper.find('.device-detail').exists()).toBe(false)
    // …yet the run of errors is already on screen.
    const tones = wrapper.findAll('.delivery-glyph').map((g) => g.classes())
    expect(tones.length).toBeGreaterThan(0)
    expect(tones.filter((c) => c.includes('delivery-glyph--error'))).toHaveLength(5)
    wrapper.unmount()
  })

  // ── loading behaviour ────────────────────────────────────────────────────

  it('loads nothing until it is opened', async () => {
    const store = useDevicesStore(pinia)
    const loadSpy = vi.spyOn(store, 'loadDevices').mockResolvedValue(undefined)
    vi.spyOn(store, 'loadAllDeliveries').mockResolvedValue(undefined)

    const wrapper = mountDrawer(pinia, false)
    await flushPromises()
    expect(loadSpy).not.toHaveBeenCalled()

    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    expect(loadSpy).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('fetches a delivery list for every display when it opens', async () => {
    const store = useDevicesStore(pinia)
    const allSpy = vi.spyOn(store, 'loadAllDeliveries')

    const wrapper = mountDrawer(pinia)
    await flushPromises()

    expect(allSpy).toHaveBeenCalledOnce()
    expect(store.devices.length).toBeGreaterThan(0)
    for (const device of store.devices) {
      expect(store.deliveries[device.key]).toBeDefined()
    }
    wrapper.unmount()
  })

  it('refreshes the list and the strips from the header button', async () => {
    const store = useDevicesStore(pinia)
    const wrapper = mountDrawer(pinia)
    await flushPromises()

    const refreshSpy = vi.spyOn(store, 'refresh').mockResolvedValue(undefined)
    await wrapper.find('.device-drawer-refresh').trigger('click')

    expect(refreshSpy).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('closes itself from the header close button', async () => {
    const wrapper = mountDrawer(pinia)
    await flushPromises()

    await wrapper.find('.device-drawer-close').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([false])
    wrapper.unmount()
  })
})
