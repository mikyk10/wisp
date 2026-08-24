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

/** Width (px) of the fixed timeline scrubber rail. */
export const TIMELINE_WIDTH = { desktop: 32, mobile: 24 } as const

/**
 * Target (minimum) size (px) of one square grid cell; the actual cell
 * stretches to fill the row (see utils/gridLayout.ts). The mobile value is
 * pinned by a test: it is the largest target that still yields three columns
 * on a 390px phone (iPhone 13) after the timeline's width and the padding
 * are taken out.
 */
export const GRID_ITEM_SIZE = { desktop: 256, mobile: 100 } as const

/**
 * Horizontal padding (px) reserved beside the grid when computing columns.
 * The scrubber's width is accounted for separately; what remains is the
 * cells' own 2px padding and a little breathing room.
 */
export const GRID_HORIZONTAL_PADDING = 8

/** Number of distinct placeholder images under public/mock-data/images/. */
export const MOCK_IMAGE_COUNT = 12
