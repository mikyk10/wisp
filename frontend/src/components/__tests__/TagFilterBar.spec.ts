import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import TagFilterBar from '../TagFilterBar.vue'
import vuetify from '@/plugins/vuetify'

function mountBar(props: Partial<InstanceType<typeof TagFilterBar>['$props']> = {}) {
  return mount(TagFilterBar, {
    props: { shown: 18443, total: 18443, filterTags: [], ...props },
    global: { plugins: [vuetify] },
  })
}

describe('TagFilterBar', () => {
  it('reads as a plain total when nothing is filtered', () => {
    const wrapper = mountBar()

    expect(wrapper.text()).toContain('18,443')
    expect(wrapper.text()).toContain('photos')
    expect(wrapper.text()).not.toContain('of')
    wrapper.unmount()
  })

  it('says how much of the catalogue is left when a filter is on', () => {
    // Without the total, a filtered grid is just a smaller number with no way
    // to tell a narrow filter from a catalogue that lost most of its photos.
    const wrapper = mountBar({ shown: 1204, total: 18443, filterTags: ['sakura'] })

    expect(wrapper.text()).toContain('1,204')
    expect(wrapper.text()).toContain('of 18,443')
    wrapper.unmount()
  })

  it('shows every active tag, not the first and a count', () => {
    // The previous UI showed "first + 2", which left the second and third
    // filters unreadable and unremovable without reopening the picker. On a
    // narrow screen the chips wrap onto another line instead.
    const wrapper = mountBar({ filterTags: ['sakura', 'night', 'sea'] })

    const text = wrapper.text()
    expect(text).toContain('sakura')
    expect(text).toContain('night')
    expect(text).toContain('sea')
    wrapper.unmount()
  })

  it('removes one tag where it stands', async () => {
    const wrapper = mountBar({ filterTags: ['sakura', 'night'] })

    await wrapper.findAll('.v-chip__close')[1].trigger('click')

    expect(wrapper.emitted('remove')?.[0]).toEqual(['night'])
    wrapper.unmount()
  })

  it('clears the whole filter in one action', async () => {
    const wrapper = mountBar({ filterTags: ['sakura', 'night'] })

    await wrapper.find('.tag-filter-clear').trigger('click')

    expect(wrapper.emitted('clear')).toHaveLength(1)
    wrapper.unmount()
  })

  it('offers nothing to clear when no filter is on', () => {
    const wrapper = mountBar()

    expect(wrapper.find('.tag-filter-clear').exists()).toBe(false)
    wrapper.unmount()
  })
})
