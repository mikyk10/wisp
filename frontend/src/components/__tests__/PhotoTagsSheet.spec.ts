import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import PhotoTagsSheet from '../PhotoTagsSheet.vue'
import vuetify from '@/plugins/vuetify'
import type { Photo } from '@/types'

const photo: Photo = {
  id: 42,
  url: '/thumb/42.jpg',
  enabled: true,
  timestamp: '2026-04-05T09:30:00Z',
  tags: ['sakura', 'night'],
}

function mountSheet(p: Photo | null = photo) {
  return mount(PhotoTagsSheet, {
    props: { photo: p, modelValue: true },
    global: { plugins: [vuetify] },
    attachTo: document.body,
  })
}

describe('PhotoTagsSheet', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('lists the photo tags', () => {
    const wrapper = mountSheet()

    const text = document.body.textContent ?? ''
    expect(text).toContain('sakura')
    expect(text).toContain('night')
    expect(text).toContain('2 tags')
    wrapper.unmount()
  })

  it('asks to filter by the tag that was tapped', async () => {
    // "More photos like this one" — the shortest useful thing to do from here,
    // and it saves finding the same word again in a picker of hundreds.
    const wrapper = mountSheet()

    const chips = Array.from(document.querySelectorAll<HTMLElement>('.photo-tags-chip'))
    chips[1].click()
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('filter')?.[0]).toEqual(['night'])
    wrapper.unmount()
  })

  it('says a photo has no tags rather than showing an empty sheet', () => {
    const wrapper = mountSheet({ ...photo, tags: [] })

    expect(document.body.textContent).toContain('This photo has no tags yet')
    wrapper.unmount()
  })

  it('renders nothing without a photo', () => {
    const wrapper = mountSheet(null)

    expect(document.querySelector('.photo-tags-sheet')).toBeNull()
    wrapper.unmount()
  })
})
