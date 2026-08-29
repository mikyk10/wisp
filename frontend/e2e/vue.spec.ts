import { test, expect } from '@playwright/test'
import type { Locator } from '@playwright/test'

// These tests run against the actual dev/preview server (mock mode — no backend).
// They verify the most important user-facing flows using public/mock-data/photos.ndjson.

test.describe('App startup', () => {
  test('loads and shows the app bar with title', async ({ page }) => {
    await page.goto('/')

    // App bar title
    await expect(page.getByText('WiSP')).toBeVisible()
  })

  test('keeps the whole wordmark on a phone', async ({ page }) => {
    // VAppBarTitle takes whatever width the controls leave it and hides the
    // overflow, so a control added to the bar cuts the wordmark mid-word
    // instead of anything visibly breaking. It read "WI" at 390px once.
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/')
    await expect(page.locator('.app-title-text')).toBeVisible()

    const cut = await page.evaluate(() => {
      const title = document.querySelector('.v-app-bar-title')?.getBoundingClientRect()
      const text = document.querySelector('.app-title-text')?.getBoundingClientRect()
      if (!title || !text) return true
      // Half a pixel of slack for sub-pixel layout.
      return text.right > title.right + 0.5 || text.left < title.left - 0.5
    })
    expect(cut, 'the wordmark is being clipped by the title box').toBe(false)

    // And the bar must not have solved it by pushing the page sideways.
    const scrolls = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth
    )
    expect(scrolls, 'the page scrolls horizontally').toBe(false)
  })

  test('shows the catalog selector in the app bar', async ({ page }) => {
    await page.goto('/')

    // Vuetify v-select for catalog selection
    const select = page.locator('.v-select')
    await expect(select).toBeVisible()
  })

  test('loads photos and shows the count in the filter bar', async ({ page }) => {
    await page.goto('/')

    // The count used to be a chip in the app bar. It moved into the filter bar
    // because the bar has no width to spare on a narrow screen — see
    // TagFilterBar — so this is where "N photos" is now.
    const bar = page.locator('.tag-filter-bar')
    await expect(bar).toBeVisible({ timeout: 15_000 })
    await expect(bar).toContainText('photos')
    await expect(bar).not.toHaveText('0 photos')
  })
})

test.describe('Photo grid', () => {
  test('renders photo cards inside the grid', async ({ page }) => {
    await page.goto('/')

    // Wait for at least one photo card to appear
    const card = page.locator('.photo-item').first()
    await expect(card).toBeVisible({ timeout: 15_000 })
  })

})

test.describe('Photo selection', () => {
  test('clicking a photo enters selection mode', async ({ page }) => {
    await page.goto('/')

    // Wait for at least one card
    const firstCard = page.locator('.photo-item').first()
    await expect(firstCard).toBeVisible({ timeout: 15_000 })

    await firstCard.click()

    // Selection toolbar slides in from the bottom
    const toolbar = page.locator('.selection-toolbar')
    await expect(toolbar).toBeVisible()

    // Selection count chip in the app bar shows "1 selected"
    const selectionChip = page.locator('.v-chip').filter({ hasText: 'selected' })
    await expect(selectionChip).toBeVisible()
  })

  test('Cancel button clears selection', async ({ page }) => {
    await page.goto('/')

    const firstCard = page.locator('.photo-item').first()
    await expect(firstCard).toBeVisible({ timeout: 15_000 })
    await firstCard.click()

    const toolbar = page.locator('.selection-toolbar')
    await expect(toolbar).toBeVisible()

    // Click the Cancel button
    await toolbar.getByText('Cancel').click()

    // Toolbar should disappear
    await expect(toolbar).not.toBeVisible()
  })

  test('Timeline scrubber answers a hover with the month under the pointer', async ({ page }) => {
    await page.goto('/')

    // The rail appears once photos exist; before that there is nothing to
    // navigate and it renders nothing at all.
    const rail = page.locator('.timeline-scrubber')
    await expect(rail).toBeVisible({ timeout: 15_000 })
    await expect(rail).toHaveAttribute('role', 'slider')

    // Idle, the rail keeps quiet. Hovering it must name the month (or the
    // honest "No date") beside the pointer without scrolling anything.
    await rail.hover()
    const bubble = page.locator('.scrubber-bubble')
    await expect(bubble).toBeVisible()
    await expect(bubble).toHaveText(/^(\d{4}\/\d{2}|No date)$/)
  })
})

test.describe('Tags', () => {
  test('a tagged photo carries a badge that opens its tags', async ({ page }) => {
    await page.goto('/')

    // The badge is the only route to a photo's tags on a touch screen, so it
    // has to be there without hovering anything.
    const badge = page.locator('.tag-badge').first()
    await expect(badge).toBeVisible({ timeout: 15_000 })

    await badge.click()

    const sheet = page.locator('.photo-tags-sheet')
    await expect(sheet).toBeVisible()
    await expect(sheet).toContainText('tag')

    // Tapping the badge must not also select the photo — the card does that.
    await expect(page.locator('.selection-toolbar')).not.toBeVisible()
  })

  test('the filter picker opens from the app bar and stays open', async ({ page }) => {
    await page.goto('/')

    await page.locator('#tag-filter-activator').click()

    const picker = page.locator('.tag-picker')
    await expect(picker).toBeVisible()
    await expect(page.locator('.tag-picker-search')).toBeVisible()

    // Still open a moment later. Asserting visibility alone passes on a picker
    // that opens and closes again within the same click — which is what an
    // overlay does when the click that opened it also reaches the handler that
    // closes it on an outside click.
    await page.waitForTimeout(500)
    await expect(picker).toBeVisible()
  })
})

/**
 * Read scrollTop once it has held the same value for three samples.
 *
 * Only ever called where nothing is animating — either nothing has scrolled
 * yet, or a written scrollTop has cancelled what was — so this is a cheap
 * confirmation rather than a wait. It is not a
 * way to outlast an animated keyboard scroll and must not be used as one:
 * WebKit's has plateaus long enough for consecutive samples to agree while it
 * is still travelling, and no sample count fixes that reliably. Tests that
 * need a stable starting point write one.
 */
async function settledScrollTop(grid: Locator): Promise<number> {
  let previous = Number.NaN
  let repeats = 0
  await expect
    .poll(
      async () => {
        const now = await grid.evaluate((el) => el.scrollTop)
        repeats = now === previous ? repeats + 1 : 0
        previous = now
        return repeats
      },
      { intervals: [120], timeout: 20_000 },
    )
    .toBeGreaterThanOrEqual(3)
  return previous
}

test.describe('Keyboard', () => {
  // The grid is the only scroller in the app: .photo-grid-container is 100vh
  // with overflow:hidden, so the document itself never overflows and the
  // browser has nothing of its own to move. jsdom cannot show any of this —
  // it has no layout — so the actual fix is only verifiable here.
  test('arrow keys and PageDown scroll the grid without a click first', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('.photo-item').first()).toBeVisible()

    const grid = page.locator('.photo-grid')
    await expect(grid).toBeFocused()

    // Down, from the top.
    await page.keyboard.press('PageDown')
    await expect.poll(() => grid.evaluate((el) => el.scrollTop)).toBeGreaterThan(0)

    // Up, from a baseline this test *sets* rather than one it waits for.
    // WebKit's keyboard scrolling animates with a tail long enough that
    // "sample until scrollTop stops moving" cannot tell the end of the
    // PageDown from the start of the ArrowUp — it read 413 as settled and
    // then travelled on to 426, which no number of extra samples fixes
    // reliably. Writing scrollTop cancels the animation outright, which does.
    await grid.evaluate((el) => el.scrollTo({ top: 2000, behavior: 'instant' }))
    const baseline = await settledScrollTop(grid)

    await page.keyboard.press('ArrowUp')
    await expect.poll(() => grid.evaluate((el) => el.scrollTop)).toBeLessThan(baseline)
  })

  test('the keyboard survives the virtualizer recycling the focused card', async ({ page }) => {
    await page.goto('/')
    const first = page.locator('.photo-item').first()
    await expect(first).toBeVisible()

    // Focus a card, then scroll far enough that its row is unmounted. Focus
    // used to fall to <body> here and every key went dead.
    await first.focus()
    const grid = page.locator('.photo-grid')
    await grid.evaluate((el) => el.scrollTo({ top: 6000, behavior: 'instant' }))

    await expect(grid).toBeFocused()

    const before = await settledScrollTop(grid)
    await page.keyboard.press('ArrowDown')
    await expect.poll(() => grid.evaluate((el) => el.scrollTop)).toBeGreaterThan(before)
  })

  // The scrubber rail is focusable and runs its own arrow keys. The grid takes
  // focus back only when focus went *nowhere* — handing it to the rail is
  // somewhere, and stealing it there would make the rail's keys unreachable.
  test('the scrubber keeps focus and its own keys once it is tabbed to', async ({ page }) => {
    await page.goto('/')
    const grid = page.locator('.photo-grid')
    await expect(grid).toBeFocused()

    const rail = page.locator('.timeline-scrubber')
    await expect(rail).toBeVisible({ timeout: 15_000 })
    await rail.focus()
    await expect(rail).toBeFocused()

    const before = await settledScrollTop(grid)
    await page.keyboard.press('End')
    await expect.poll(() => grid.evaluate((el) => el.scrollTop)).toBeGreaterThan(before)

    // Still the rail's, after the grid's reclaim had its animation frame.
    await expect(rail).toBeFocused()
  })
})

test.describe('Escape', () => {
  test('closes the tag picker without dropping the selection behind it', async ({ page }) => {
    await page.goto('/')
    await page.locator('.photo-item').first().click()
    await expect(page.locator('.selection-toolbar')).toBeVisible()

    await page.locator('#tag-filter-activator').click()
    await expect(page.locator('.tag-picker')).toBeVisible()

    await page.keyboard.press('Escape')

    await expect(page.locator('.tag-picker')).not.toBeVisible()
    // The picker was what Escape was aimed at; the selection is untouched.
    await expect(page.locator('.selection-toolbar')).toBeVisible()

    // A second Escape, with nothing on top, is the one that clears it.
    await page.keyboard.press('Escape')
    await expect(page.locator('.selection-toolbar')).not.toBeVisible()
  })

  test('closes the per-photo tag sheet', async ({ page }) => {
    await page.goto('/')
    await page.locator('.tag-badge').first().click()
    await expect(page.locator('.photo-tags-sheet')).toBeVisible()

    await page.keyboard.press('Escape')

    await expect(page.locator('.photo-tags-sheet')).not.toBeVisible()
  })

  test('closes the displays drawer and hands focus back to its trigger', async ({ page }) => {
    await page.goto('/')
    await page.locator('.device-drawer-trigger').click()

    // Not toBeVisible: a closed temporary drawer stays in the DOM and is moved
    // off screen with a transform, which still has a bounding box, so
    // toBeVisible() passes either way. The --active class is the real state.
    const drawer = page.locator('.device-drawer')
    await expect(drawer).toHaveClass(/v-navigation-drawer--active/)
    await expect(page.locator('.v-navigation-drawer__scrim')).toBeVisible()

    // Focus something inside it, as tabbing in would: closing has to bring
    // focus back out, or it is left on an element on its way off screen and
    // the next Tab starts from nowhere.
    await page.locator('.device-drawer-close').focus()

    // VNavigationDrawer is not a VOverlay and ships no Escape handling of its
    // own, so this did nothing at all before.
    await page.keyboard.press('Escape')

    await expect(drawer).not.toHaveClass(/v-navigation-drawer--active/)
    await expect(page.locator('.v-navigation-drawer__scrim')).toHaveCount(0)
    await expect(page.locator('.device-drawer-trigger')).toBeFocused()
  })

  test('typing in the tag search is not disturbed by the IME guard', async ({ page }) => {
    await page.goto('/')
    await page.locator('#tag-filter-activator').click()

    const search = page.locator('.tag-picker-search input')
    await search.fill('a')
    await expect(search).toHaveValue('a')

    await page.keyboard.press('Escape')
    await expect(page.locator('.tag-picker')).not.toBeVisible()
  })
})
