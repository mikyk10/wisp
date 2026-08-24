/**
 * Column arithmetic for the photo grid.
 *
 * Two decisions live here, and both exist to be testable away from a browser:
 *
 * Columns come from the layout width and a *target* cell size — as many
 * columns as whole targets fit. The cell then stretches to use what is left
 * over, instead of leaving a dead gutter beside the scrubber: with c columns
 * the stretch is bounded by 1/c of the target, so cells grow a little and
 * never a lot. Rows stay uniform — every cell in every row shares the one
 * size — which is the property the scrubber's fraction mapping stands on.
 *
 * The width this is fed must be the LAYOUT viewport
 * (document.documentElement.clientWidth), never window.innerWidth. On iOS,
 * innerWidth follows the visual viewport: pinching in shrinks it, the resize
 * handler dutifully reflows to one column, and the resize that should undo it
 * at scale 1.0 is not reliably delivered — leaving the grid collapsed until a
 * reload. A pinch is a magnifying glass, not a window resize; layout must not
 * see it at all.
 */

export interface GridLayout {
  columns: number
  /** Actual cell size (px): the target stretched to fill the row. */
  itemSize: number
}

export function computeGridLayout(
  layoutWidth: number,
  targetItemSize: number,
  reservedWidth: number,
): GridLayout {
  const available = Math.max(0, layoutWidth - reservedWidth)
  const columns = Math.max(1, Math.floor(available / targetItemSize))
  const itemSize = Math.max(1, Math.floor(available / columns))
  return { columns, itemSize }
}
