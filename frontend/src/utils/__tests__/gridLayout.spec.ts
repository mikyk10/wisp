import { describe, it, expect } from 'vitest'
import { computeGridLayout } from '../gridLayout'
import { GRID_HORIZONTAL_PADDING, GRID_ITEM_SIZE, TIMELINE_WIDTH } from '@/constants'

/** The width the grid actually computes with on a phone. */
const mobileReserved = TIMELINE_WIDTH.mobile + GRID_HORIZONTAL_PADDING

describe('computeGridLayout', () => {
  it('gives an iPhone three columns', () => {
    // Pinned against the real constants on purpose: this is the requirement,
    // and a future nudge to the target size, the timeline width or the
    // padding that quietly costs a column on a 390px phone should have to
    // come through this test. 375px (SE) is deliberately not pinned — with
    // the current timeline width it holds two generous columns.
    for (const width of [430, 390]) {
      const { columns } = computeGridLayout(width, GRID_ITEM_SIZE.mobile, mobileReserved)
      expect(columns, `${width}px phone`).toBe(3)
    }
  })

  it('stretches the cell to fill the row instead of leaving a gutter', () => {
    const { columns, itemSize } = computeGridLayout(390, GRID_ITEM_SIZE.mobile, mobileReserved)
    const available = 390 - mobileReserved

    // Stretched, never shrunk: the target is a floor.
    expect(itemSize).toBeGreaterThanOrEqual(GRID_ITEM_SIZE.mobile)
    // And still fitting: the row must not push under the scrubber.
    expect(columns * itemSize).toBeLessThanOrEqual(available)
    // The leftover is less than a column's worth — that is what "filled" means.
    expect(available - columns * itemSize).toBeLessThan(columns)
  })

  it('never answers zero columns, whatever the width', () => {
    for (const width of [0, 50, 118]) {
      const { columns, itemSize } = computeGridLayout(width, GRID_ITEM_SIZE.mobile, mobileReserved)
      expect(columns).toBe(1)
      expect(itemSize).toBeGreaterThanOrEqual(1)
    }
  })

  it('scales to the desktop the same way', () => {
    const reserved = TIMELINE_WIDTH.desktop + GRID_HORIZONTAL_PADDING
    const { columns, itemSize } = computeGridLayout(1512, GRID_ITEM_SIZE.desktop, reserved)
    expect(columns).toBe(5)
    expect(itemSize).toBeGreaterThanOrEqual(GRID_ITEM_SIZE.desktop)
  })
})
