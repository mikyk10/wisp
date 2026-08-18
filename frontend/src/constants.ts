/**
 * Layout constants shared between JS (column math, matchMedia) and CSS.
 *
 * The CSS side lives as custom properties in App.vue (--wisp-timeline-width,
 * --wisp-grid-item-size); the mobile breakpoint appears once more in that
 * file's @media query because CSS media queries cannot read custom
 * properties. Change values here and in App.vue together.
 */

/** Viewport width (px) at or below which the mobile layout applies. */
export const MOBILE_BREAKPOINT = 768

/** Width (px) of the fixed timeline sidebar. */
export const TIMELINE_WIDTH = { desktop: 120, mobile: 80 } as const

/** Size (px) of one square grid cell. */
export const GRID_ITEM_SIZE = { desktop: 256, mobile: 130 } as const

/** Horizontal padding (px) reserved around the grid when computing columns. */
export const GRID_HORIZONTAL_PADDING = 32

/** Number of distinct placeholder images under public/mock-data/images/. */
export const MOCK_IMAGE_COUNT = 12
